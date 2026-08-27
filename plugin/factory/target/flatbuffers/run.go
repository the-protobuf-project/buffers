package flatbuffers

// run.go holds one Generate call's mutable state and renders one .fbs.
//
// The body is assembled before the header, which is the ordering worth knowing:
// a substituted well-known type is discovered while projecting a field, and the
// include line it requires has to sit above the namespace.

import (
	"sort"
	"strings"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// run is one Generate call's mutable state.
type run struct {
	*Target
	// schema is the graph being rendered.
	schema *bufir.Schema

	// needed is every substituted well-known record the whole run reached for,
	// which decides what the prelude file contains.
	needed map[preludeType]bool

	// fileNeeds is the same for the file currently being rendered, which decides
	// whether that file includes the prelude at all.
	//
	// The two are separate because a substitution is discovered *while* rendering
	// a field, and the include line has to be written above everything. The body
	// is therefore rendered first and the header assembled afterwards; a single
	// run-wide set would make every file after the first include a prelude it may
	// not use.
	fileNeeds map[preludeType]bool

	// diags accumulates problems found while projecting types.
	diags []bufir.Diagnostic
}

// file renders one .fbs, or nil when the file has nothing this target emits.
func (r *run) file(f *bufir.File) ([]byte, error) {
	msgs := r.emittable(f)
	enums := r.emittableEnums(f)
	if len(msgs) == 0 && len(enums) == 0 {
		return nil, nil
	}

	// The body is rendered before the header. Substituted well-known types are
	// discovered as fields are projected, and the include line they require has
	// to sit above the namespace — so what the header says depends on what the
	// body turned out to need.
	r.fileNeeds = map[preludeType]bool{}

	var b emit.Buf
	for _, e := range enums {
		b.Line("")
		r.enum(&b, e)
	}

	// Structs before tables, and structs among themselves in dependency order: a
	// FlatBuffers struct is laid out inline, so flatc must know a nested struct's
	// size before it can size the one containing it.
	structs, tables := split(msgs)
	for _, m := range r.topoSort(structs) {
		b.Line("")
		if err := r.record(&b, f, m); err != nil {
			return nil, err
		}
	}

	// Map entries and union arm wrappers are synthesized types the schema
	// references; they must exist before the tables that use them.
	for _, m := range tables {
		r.mapEntries(&b, f, m)
	}
	for _, m := range tables {
		r.unionTypes(&b, f, m)
	}

	for _, m := range tables {
		b.Line("")
		if err := r.record(&b, f, m); err != nil {
			return nil, err
		}
	}

	r.fileFooter(&b, f, msgs)

	var head emit.Buf
	head.Raw(r.banner(f.Path))
	head.Line("")
	// Includes come before the namespace: flatc parses them as file-level
	// directives and rejects one that follows a declaration.
	if includes := r.includes(f); len(includes) > 0 {
		for _, inc := range includes {
			head.Linef("include %q;", inc)
		}
		head.Line("")
	}
	head.Linef("namespace %s;", f.Namespace)
	head.Raw(b.String())

	return head.Bytes(), nil
}

// includes maps the file's proto imports onto .fbs includes.
//
// Imports of google/protobuf/* are dropped: those types are substituted by the
// prelude, so including a .fbs rendering of them would be including a file this
// plugin never wrote.
func (r *run) includes(f *bufir.File) []string {
	var out []string
	for _, imp := range f.Imports {
		if strings.HasPrefix(imp, "google/protobuf/") {
			continue
		}
		out = append(out, fbsPath(imp))
	}
	if len(r.fileNeeds) > 0 {
		out = append(out, preludePath)
	}
	sort.Strings(out)
	return out
}
