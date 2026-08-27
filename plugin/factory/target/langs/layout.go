package langs

import (
	"path"
	"sort"
)

// Nesting describes how a generator lays out its own output.
type Nesting uint8

const (
	// NestFlat writes one file per schema into the output directory, with any
	// namespace expressed inside the file.
	NestFlat Nesting = iota

	// NestByNamespace builds a directory tree from the schema's namespace.
	NestByNamespace
)

// nesting records which behaviour each flatc backend has. Every entry was
// determined by running flatc and looking at the tree, not from documentation.
//
// A language absent from this map is treated as NestFlat, which is the
// conservative default: mirroring the source tree for a generator that also
// nests produces a doubled path, which is obvious and easily fixed, while
// failing to mirror one that does not silently collides files.
var nesting = map[string]Nesting{
	"go":         NestByNamespace,
	"java":       NestByNamespace,
	"kotlin":     NestByNamespace,
	"kotlin-kmp": NestByNamespace,
	"python":     NestByNamespace,
	"csharp":     NestByNamespace,
	"php":        NestByNamespace,

	"cpp":     NestFlat,
	"c++":     NestFlat,
	"rust":    NestFlat,
	"swift":   NestFlat,
	"dart":    NestFlat,
	"ts":      NestFlat,
	"lobster": NestFlat,
	"lua":     NestFlat,
	"nim":     NestFlat,
}

// NestingOf reports how a language's generator lays out its output.
func NestingOf(lang string) Nesting { return nesting[lang] }

// Group is one set of schema files sharing a package, which is to say one source
// directory.
//
// The grouping is by directory rather than by declared namespace because the
// directory is what decides the output path, and two files in one directory that
// declared different namespaces would still be written side by side.
type Group struct {
	// Dir is the group's directory relative to the schema root, e.g.
	// "sensors/v1". Empty for schemas at the root.
	Dir string

	// Files are the schema files in the group, relative to the schema root.
	Files []string

	// Namespace is the schema's declared namespace, e.g. "sensors.v1". It is what
	// flatc derives every language's package from unless overridden below.
	Namespace string

	// JVMPackage is the package Java and Kotlin output should declare, from the
	// proto's java_package. Empty leaves flatc's default (the namespace).
	JVMPackage string

	// GoPackage is the package name Go output should declare, from the proto's
	// go_package alias. Empty leaves flatc's default.
	GoPackage string

	// GoModule is the Go module the generated code will live in.
	//
	// Without it, flatc writes cross-package imports as bare namespace paths —
	// `import "buffers/wellknown"` — which resolve against no module and fail with
	// "package buffers/wellknown is not in std". It is the same requirement
	// capnpc-go has, for the same reason: the generator knows the package tree
	// and not the module root it hangs from.
	GoModule string
}

// Plan turns a set of groups into the invocations needed to compile them.
//
// One invocation per group rather than one for everything: the output directory
// and the package flags both vary per group, and flatc takes them globally.
func Plan(target, lang, schemaDir, outDir string, groups []Group) []Request {
	out := make([]Request, 0, len(groups))

	for _, g := range groups {
		if len(g.Files) == 0 {
			continue
		}
		req := Request{
			Target:    target,
			Language:  lang,
			SchemaDir: schemaDir,
			OutDir:    outDir,
			Files:     g.Files,
		}
		if NestingOf(lang) == NestFlat && g.Dir != "" {
			// Reproduce the source tree, which is what protoc does and what a
			// generator that emits one flat file per schema will not do alone.
			req.OutDir = path.Join(outDir, g.Dir)
		}
		req.Flags = packageFlags(target, lang, g)
		req.Flags = append(req.Flags, includeFlags(target, lang)...)
		out = append(out, req)
	}
	return out
}

// GroupFiles buckets schema files by directory, preserving a stable order.
func GroupFiles(files []string) map[string][]string {
	out := map[string][]string{}
	for _, f := range files {
		dir := path.Dir(f)
		if dir == "." {
			dir = ""
		}
		out[dir] = append(out[dir], f)
	}
	for dir := range out {
		sort.Strings(out[dir])
	}
	return out
}
