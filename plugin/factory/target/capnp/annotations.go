package capnp

import (
	"fmt"
	"sort"

	"github.com/the-protobuf-project/protokit/buffers"
)

// annotationSpec describes one language's annotation requirements.
type annotationSpec struct {
	// Alias is the `using` name bound in the emitted schema.
	Alias string

	// Import is the path the annotation schema is imported from. It is absolute
	// in capnp terms (leading slash), so it resolves against the import path
	// rather than against the importing file.
	Import string

	// Render writes the file-level annotation lines for one file, or returns
	// false when the file lacks what the language needs.
	Render func(*run, *buffers.File) ([]string, bool)
}

// annotationSpecs is the set of languages whose generators need annotations.
//
// Rust, C++ and TypeScript are absent because their generators derive everything
// from the file path and the schema itself. C++ is the near-miss worth naming:
// `$Cxx.namespace` is an annotation too, but c++.capnp ships inside capnp's own
// standard import path, so it is always resolvable and is emitted unconditionally
// in capnp.go rather than gated here.
var annotationSpecs = map[string]annotationSpec{
	"go": {
		Alias:  "Go",
		Import: "/go.capnp",
		Render: func(r *run, f *buffers.File) ([]string, bool) {
			pkg := f.GoPackage
			if pkg == "" {
				return nil, false
			}
			lines := []string{fmt.Sprintf("$Go.package(%q);", pkg)}

			// $Go.import must name where the *generated file* ends up, and
			// capnpc-go writes output mirroring the schema tree — sensors/v1/x.capnp
			// becomes <out>/sensors/v1/x.capnp.go. The proto's own go_package is a
			// different path entirely (it describes where protoc-gen-go's output
			// goes), so using it here produces an import that does not resolve.
			//
			// That leaves the module root, which nothing in the schema knows, so it
			// is configuration. Without it the import line is omitted: capnpc-go
			// only requires the package annotation, and a schema whose files never
			// reference each other generates fine without it.
			if r.goModule != "" {
				lines = append(lines, fmt.Sprintf("$Go.import(%q);", goImportPath(r.goModule, f.Path)))
			} else if r.crossFile(f) {
				r.collect(&buffers.Diagnostic{
					Rule: buffers.RuleTarget,
					Node: buffers.NodeID(f.Path),
					Message: fmt.Sprintf("%s references types in another file, and capnp Go output was "+
						"requested without go_module; capnpc-go will emit an import path it cannot resolve", f.Path),
					Hint: "set go_module to the Go module the generated capnp code will live in " +
						"(the go_module opt, or go_module: in buffers.yaml)",
				})
			}
			return lines, true
		},
	},
	"java": {
		Alias:  "Java",
		Import: "/capnp/java.capnp",
		Render: func(_ *run, f *buffers.File) ([]string, bool) {
			if f.JVMPackage == "" {
				return nil, false
			}
			return []string{
				fmt.Sprintf("$Java.package(%q);", f.JVMPackage),
				// capnproto-java puts every type in one outer class, and needs to
				// be told its name. It is derived from the proto file rather than
				// declared, matching what protoc does for java_outer_classname.
				fmt.Sprintf("$Java.outerClassname(%q);", outerClassname(f.Path)),
			}, true
		},
	},
}

// AnnotationLanguages returns the languages that require annotation blocks, which
// is what the CLI checks before putting schema files on the import path.
func AnnotationLanguages() []string {
	out := make([]string, 0, len(annotationSpecs))
	for lang := range annotationSpecs {
		out = append(out, lang)
	}
	sort.Strings(out)
	return out
}

// NeedsAnnotations reports whether a language's generator requires them.
func NeedsAnnotations(lang string) bool {
	_, ok := annotationSpecs[normalizeLang(lang)]
	return ok
}
