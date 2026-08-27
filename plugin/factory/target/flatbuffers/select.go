package flatbuffers

// select.go decides what this target emits and in what order: which declarations
// survive the filters, and the dependency ordering a struct's inline layout
// requires.

import (
	"path"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
)

// emittable returns the file's messages that this target renders, flattening
// nested messages — FlatBuffers has no nesting, so a nested proto message becomes
// a sibling type.
func (r *run) emittable(f *buffers.File) []*buffers.Message {
	var out []*buffers.Message
	var walk func(msgs []*buffers.Message)
	walk = func(msgs []*buffers.Message) {
		for _, m := range msgs {
			// A map entry is never emitted as itself: mapEntries synthesizes a
			// keyed replacement per map field instead.
			if !m.Skip && !m.IsMapEntry && allows(m.Targets) {
				out = append(out, m)
			}
			walk(m.Nested)
		}
	}
	walk(f.Messages)
	return out
}

// emittableEnums returns the file's enums, nested ones included, since
// FlatBuffers has no nesting.
func (r *run) emittableEnums(f *buffers.File) []*buffers.Enum {
	var out []*buffers.Enum
	var walk func(msgs []*buffers.Message)
	walk = func(msgs []*buffers.Message) {
		for _, m := range msgs {
			for _, e := range m.Enums {
				if !e.Skip {
					out = append(out, e)
				}
			}
			walk(m.Nested)
		}
	}
	for _, e := range f.Enums {
		if !e.Skip {
			out = append(out, e)
		}
	}
	walk(f.Messages)
	return out
}

// topoSort orders structs so that a struct is declared after every struct it
// embeds. A FlatBuffers struct is laid out inline, so its size depends on theirs.
//
// The graph is acyclic by construction: the IR's layout pass refuses to pack a
// message that transitively contains itself.
func (r *run) topoSort(structs []*buffers.Message) []*buffers.Message {
	index := map[buffers.NodeID]*buffers.Message{}
	for _, m := range structs {
		index[m.Node] = m
	}

	var out []*buffers.Message
	done := map[buffers.NodeID]bool{}
	var visit func(m *buffers.Message)
	visit = func(m *buffers.Message) {
		if done[m.Node] {
			return
		}
		done[m.Node] = true
		for _, f := range m.Fields {
			if f.Kind != buffers.KindMessage {
				continue
			}
			if dep, ok := index[buffers.NodeID(f.Message)]; ok {
				visit(dep)
			}
		}
		out = append(out, m)
	}
	for _, m := range structs {
		visit(m)
	}
	return out
}

// split partitions messages into packed structs and evolvable tables.
func split(msgs []*buffers.Message) (structs, tables []*buffers.Message) {
	for _, m := range msgs {
		if m.Layout == buffers.LayoutStruct {
			structs = append(structs, m)
			continue
		}
		tables = append(tables, m)
	}
	return structs, tables
}

// ownerOf returns the file declaring a named type, for namespace qualification.
func (r *run) ownerOf(fullName string) *buffers.File {
	if m := r.schema.Messages[buffers.NodeID(fullName)]; m != nil {
		return m.File
	}
	if e := r.schema.Enums[buffers.NodeID(fullName)]; e != nil {
		return e.File
	}
	return nil
}

// collect records a diagnostic, ignoring nil so callers can pass a projection's
// result directly.
func (r *run) collect(d *buffers.Diagnostic) {
	if d != nil {
		r.diags = append(r.diags, *d)
	}
}

// banner renders the file header, naming the .proto the schema came from.
//
// The source is the input path, not the output path: a reader who finds a
// surprising line in a .fbs needs to know which .proto to edit, and telling them
// the name of the file they already have open helps nobody.
func (r *run) banner(protoPath string) string {
	info := r.info
	info.Source = protoPath
	return provenance.Render(provenance.Slash, info)
}

// allows reports whether a target allow-list admits this target.
func allows(list []string) bool {
	if len(list) == 0 {
		return true
	}
	for _, t := range list {
		if t == "flatbuffers" {
			return true
		}
	}
	return false
}

// fbsPath maps a proto path onto its .fbs sibling.
func fbsPath(protoPath string) string {
	return strings.TrimSuffix(protoPath, path.Ext(protoPath)) + ".fbs"
}
