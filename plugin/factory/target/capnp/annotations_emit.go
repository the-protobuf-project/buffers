package capnp

// annotations_emit.go writes the per-language annotation block into a file, and
// derives the values it carries from the proto's own options.
//
// The derivation is the part worth reading: $Go.import means what go_package
// already means, and $Java.package what java_package does, so neither is
// re-declared in buffers.v1. Asking an author to restate an import path in a
// second vocabulary is how the two come to disagree.

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// annotationHeader renders the `using` and annotation lines for one file, for
// every language this run was told to support.
func (r *run) annotationHeader(b *emit.Buf, f *buffers.File) {
	langs := make([]string, 0, len(r.annotations))
	for lang := range r.annotations {
		if _, ok := annotationSpecs[lang]; ok {
			langs = append(langs, lang)
		}
	}
	sort.Strings(langs)

	for _, lang := range langs {
		spec := annotationSpecs[lang]
		lines, ok := spec.Render(r, f)
		if !ok {
			// The file does not declare what this language needs. Emitting the
			// `using` alone would add an unresolvable import for no benefit, so
			// the omission is reported instead — a Go build that fails on
			// "missing package annotation" is far less clear than this.
			r.collect(&buffers.Diagnostic{
				Rule: buffers.RuleTarget,
				Node: buffers.NodeID(f.Path),
				Message: fmt.Sprintf("capnp %s output was requested, but %s declares no %s; "+
					"the generator will reject the emitted schema", lang, f.Path, missingOption(lang)),
				Hint: fmt.Sprintf("add `option %s` to the .proto", missingOption(lang)),
			})
			continue
		}
		b.Linef("using %s = import %q;", spec.Alias, spec.Import)
		for _, line := range lines {
			b.Line(line)
		}
	}
}

// missingOption names the proto option a language's annotation is derived from,
// so the diagnostic points at the fix rather than at the symptom.
func missingOption(lang string) string {
	switch lang {
	case "go":
		return "go_package"
	case "java":
		return "java_package (or (buffers.v1.file).jvm_package)"
	}
	return lang + " package"
}

// goImportPath builds the Go import path for a generated capnp file: the module
// root joined to the schema file's directory, since that is where capnpc-go
// writes it.
func goImportPath(module, schemaPath string) string {
	dir := path.Dir(capnpPath(schemaPath))
	if dir == "." || dir == "/" {
		return module
	}
	return module + "/" + dir
}

// crossFile reports whether a file references a type declared elsewhere, which is
// the only case where a missing $Go.import actually breaks the build.
func (r *run) crossFile(f *buffers.File) bool {
	for _, imp := range f.Imports {
		if !strings.HasPrefix(imp, "google/protobuf/") {
			return true
		}
	}
	return len(r.fileNeeds) > 0
}

// outerClassname derives capnproto-java's outer class name from the proto path,
// matching protoc's own rule: the base name, PascalCased, with "Proto" appended
// when that would otherwise collide with a type declared in the file.
func outerClassname(protoPath string) string {
	base := protoPath
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".proto")

	var b strings.Builder
	for _, part := range strings.Split(base, "_") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String() + "Capnp"
}

// normalizeLang folds the spellings a caller might use onto the keys above.
// "c++" and "cpp" are the same generator; "kotlin" is produced by the Java one.
func normalizeLang(lang string) string {
	switch strings.ToLower(lang) {
	case "cpp", "c++":
		return "c++"
	case "kotlin", "kt", "java":
		return "java"
	case "golang", "go":
		return "go"
	case "typescript", "ts":
		return "ts"
	}
	return strings.ToLower(lang)
}

// warnEnumOnlyGo reports a file that will hit a known capnpc-go defect.
//
// capnpc-go (through v3.1.0-alpha.2) emits `capnp.EnumList[T]` for an enum but
// only adds the `capnp` import when the file also contains a struct or an
// interface. A file holding nothing but enums therefore generates Go that does
// not compile:
//
//	enums.capnp.go:62:24: undefined: capnp
//
// It reproduces on a hand-written six-line schema, so it is not something this
// plugin causes or can correct — emitting the enum somewhere else would mean
// abandoning the one-proto-file-to-one-capnp-file mapping that makes the output
// navigable, to work around someone else's bug.
//
// What is worth doing is saying so here, because the alternative is the user
// meeting `undefined: capnp` in generated code and having no reason to suspect
// their own schema is fine.
func (r *run) warnEnumOnlyGo(f *buffers.File, msgs []*buffers.Message, enums []*buffers.Enum, svcs []*buffers.Service) {
	if !r.annotations["go"] || len(enums) == 0 || len(msgs) > 0 || len(svcs) > 0 {
		return
	}
	r.collect(&buffers.Diagnostic{
		Rule: buffers.RuleLint,
		Node: buffers.NodeID(f.Path),
		Message: fmt.Sprintf("%s declares only enums, and capnpc-go emits capnp.EnumList without the "+
			"matching import for such a file — the generated Go will not compile (undefined: capnp)", f.Path),
		Hint: "an upstream capnpc-go defect, not a problem with this schema; move one message into " +
			"the file, or generate this file's Go with a patched capnpc-go",
	})
}

// warnInterfacesForJava reports a schema Java cannot be generated from.
//
// capnproto-java implements serialization only — capnproto.org lists it that way
// — and its plugin does not decline an interface, it aborts on one:
//
//	*** Uncaught exception ***
//	main/cpp/capnpc-java.c++:1675: failed: interfaces not implemented
//
// which names a line in someone else's C++ and nothing about the schema that
// caused it. The RPC interfaces this target emits are exactly what triggers it,
// so a schema with a service compiles as C++, Go and Rust and dies here.
//
// Kotlin is covered by the same check without naming it: Cap'n Proto has no
// Kotlin generator, so a Kotlin run is a Java run.
func (r *run) warnInterfacesForJava(f *buffers.File, svcs []*buffers.Service) {
	if !r.annotations["java"] || len(svcs) == 0 {
		return
	}
	names := make([]string, len(svcs))
	for i, s := range svcs {
		names[i] = s.Name
	}
	r.collect(&buffers.Diagnostic{
		Rule: buffers.RuleTarget,
		Node: buffers.NodeID(f.Path),
		Message: fmt.Sprintf("%s emits interfaces for %s, and capnproto-java implements serialization "+
			"only — capnpc-java aborts on an interface with \"failed: interfaces not implemented\" "+
			"rather than reporting it", f.Path, strings.Join(names, ", ")),
		Hint: "keep the services out of the Java build with (buffers.v1.service).targets, or take " +
			"Java from the FlatBuffers or Thrift target, both of which generate a service surface",
	})
}
