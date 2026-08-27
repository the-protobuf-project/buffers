package flatbuffers

// enum.go renders enums and the file footer — the root type, identifier and
// extension that make a .fbs readable as a self-describing payload.

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// enum renders a proto enum.
func (r *run) enum(b *emit.Buf, e *buffers.Enum) {
	b.Doc("///", e.Doc)

	attrs := ""
	values := e.Values
	if e.BitFlags {
		attrs = " (bit_flags)"
		b.Linef("/// Declared (bit_flags), so the AIP-126 zero value is dropped: in a bitmask")
		b.Linef("/// position 0 is a real bit, and keeping it would make it mean \"unspecified\"")
		b.Linef("/// for every consumer.")
		values = withoutZero(values)
	}

	b.Block(fmt.Sprintf("enum %s : %s%s {", e.Name, width(e.Underlying), attrs), "}", func() {
		for i, v := range values {
			if v.Skip {
				continue
			}
			b.Doc("///", v.Doc)
			sep := ","
			if i == len(values)-1 {
				sep = ""
			}
			b.Linef("%s = %d%s", v.Name, v.Number, sep)
		}
	})
}

// withoutZero drops the AIP-126 zero value, which a bit_flags enum cannot have
// because position 0 is a real bit.
func withoutZero(values []*buffers.EnumValue) []*buffers.EnumValue {
	out := make([]*buffers.EnumValue, 0, len(values))
	for _, v := range values {
		if v.Number == 0 {
			continue
		}
		out = append(out, v)
	}
	return out
}

// fileFooter emits root_type, file_identifier and file_extension, in the order
// flatc requires them.
func (r *run) fileFooter(b *emit.Buf, f *buffers.File, msgs []*buffers.Message) {
	var roots []*buffers.Message
	for _, m := range msgs {
		if m.FBSRoot {
			roots = append(roots, m)
		}
	}

	switch {
	case len(roots) > 1:
		names := make([]string, len(roots))
		for i, m := range roots {
			names[i] = m.Name
		}
		r.collect(&buffers.Diagnostic{
			Rule:    buffers.RuleTarget,
			Node:    buffers.NodeID(f.Path),
			Message: fmt.Sprintf("%d messages set (buffers.v1.message).fbs_root (%s); a .fbs has one root_type", len(roots), strings.Join(names, ", ")),
			Hint:    "keep fbs_root on the message that is actually the payload and remove it from the others",
		})
		return
	case len(roots) == 0:
		if f.Identifier != "" {
			r.collect(&buffers.Diagnostic{
				Rule:    buffers.RuleTarget,
				Node:    buffers.NodeID(f.Path),
				Message: "file_id is set but no message sets (buffers.v1.message).fbs_root; flatc rejects a file_identifier without a root_type",
				Hint:    "mark the payload message with fbs_root, or clear file_id",
			})
		}
		return
	}

	b.Line("")
	b.Linef("root_type %s;", roots[0].Name)
	if f.Identifier != "" {
		b.Linef("file_identifier %q;", f.Identifier)
	}
	if f.Extension != "" {
		b.Linef("file_extension %q;", f.Extension)
	}
}
