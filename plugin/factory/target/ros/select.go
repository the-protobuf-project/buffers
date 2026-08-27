package ros

// select.go decides what this target emits: which declarations survive the skip
// and target filters, flattened for a format that has no nesting.

import (
	"path"
	"sort"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
)

// flattenMessages returns every message a file declares, nested ones included.
// ROS has no nested types, so a nested proto message becomes its own file.
func flattenMessages(f *buffers.File) []*buffers.Message {
	var out []*buffers.Message
	var walk func(msgs []*buffers.Message)
	walk = func(msgs []*buffers.Message) {
		for _, m := range msgs {
			if !m.Skip && !m.IsMapEntry && allows(m.Targets) {
				out = append(out, m)
			}
			walk(m.Nested)
		}
	}
	walk(f.Messages)
	return out
}

// flattenEnums returns every enum a file declares, nested ones included, since
// ROS has no nesting and each becomes its own file.
func flattenEnums(f *buffers.File) []*buffers.Enum {
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

// liveArms returns a oneof's surviving arms in ordinal order.
func liveArms(one *buffers.Oneof) []*buffers.Field {
	arms := make([]*buffers.Field, 0, len(one.Fields))
	for _, f := range one.Fields {
		if f.Skip || !allows(f.Targets) {
			continue
		}
		arms = append(arms, f)
	}
	sort.SliceStable(arms, func(i, j int) bool { return arms[i].Ordinal < arms[j].Ordinal })
	return arms
}

// collect records a diagnostic, ignoring nil so callers can pass a projection's
// result directly.
func (r *run) collect(d *buffers.Diagnostic) {
	if d != nil {
		r.diags = append(r.diags, *d)
	}
}

// banner renders the file header, naming the .proto the schema came from — not
// the output path, since a reader who finds a surprising line needs to know
// which .proto to edit.
func (r *run) banner(protoPath string) string {
	info := r.info
	info.Source = protoPath
	return provenance.Render(provenance.Hash, info)
}

// firstLine returns a doc comment's first line, for a one-line summary.
func firstLine(doc string) string {
	if i := strings.Index(doc, "\n"); i >= 0 {
		return doc[:i]
	}
	return doc
}

// allows reports whether a target allow-list admits this target. An empty list
// admits everything, which is the common case.
func allows(list []string) bool {
	if len(list) == 0 {
		return true
	}
	for _, t := range list {
		if t == "ros" {
			return true
		}
	}
	return false
}

// msgPath and srvPath place a file in the ROS package layout a colcon build
// expects: <package>/msg/<Type>.msg and <package>/srv/<Name>.srv.
func msgPath(pkg, typeName string) string { return path.Join(pkg, "msg", typeName+".msg") }

// srvPath places a .srv in the ROS package layout a colcon build expects.
func srvPath(pkg, name string) string { return path.Join(pkg, "srv", name+".srv") }
