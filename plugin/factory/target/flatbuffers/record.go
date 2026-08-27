package flatbuffers

// record.go renders a message as a table or a struct, and decides the attributes
// each field carries.

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// record renders one message as a table or a struct.
func (r *run) record(b *emit.Buf, f *buffers.File, m *buffers.Message) error {
	b.Doc("///", m.Doc)
	if m.Layout == buffers.LayoutStruct {
		return r.structBody(b, f, m)
	}
	return r.tableBody(b, f, m)
}

// structBody renders a packed record.
//
// A struct carries no ids, no defaults and no attributes: its fields are
// positional and all of them are always present. That is the whole trade, and it
// is why the IR's layout pass has to have proved eligibility before we get here.
func (r *run) structBody(b *emit.Buf, f *buffers.File, m *buffers.Message) error {
	var err error
	b.Block(fmt.Sprintf("struct %s {", m.Name), "}", func() {
		for _, field := range m.Fields {
			if field.Skip || !allows(field.Targets) {
				continue
			}
			typ, diag := r.fieldType(field, f)
			r.collect(diag)
			b.Doc("///", field.Doc)
			b.Linef("%s:%s;", field.Name, typ)
		}
	})
	return err
}

// tableBody renders an evolvable record, one line per planned slot.
func (r *run) tableBody(b *emit.Buf, f *buffers.File, m *buffers.Message) error {
	attrs := ""
	if m.OriginalOrder {
		attrs = " (original_order)"
	}

	b.Block(fmt.Sprintf("table %s%s {", m.Name, attrs), "}", func() {
		for _, slot := range r.planSlots(m) {
			switch slot.Kind {
			case slotPlaceholder:
				// `(deprecated)` keeps the vtable slot allocated and suppresses
				// the accessor, which is exactly "spoken for, do not read".
				b.Linef("/// %s", slot.Why)
				b.Linef("__slot_%d:byte (id: %d, deprecated);", slot.ID, slot.ID)

			case slotUnion:
				b.Doc("///", slot.Oneof.Doc)
				b.Linef("%s:%s (id: %d);", slot.Oneof.Name, slot.Oneof.UnionName, slot.ID)

			default:
				field := slot.Field
				typ, diag := r.fieldType(field, f)
				r.collect(diag)
				b.Doc("///", field.Doc)
				b.Linef("%s:%s%s;", field.Name, typ, r.attributes(field, slot.ID))
			}
		}
	})
	return nil
}

// attributes renders a table field's trailing attribute list.
func (r *run) attributes(f *buffers.Field, id int32) string {
	attrs := []string{fmt.Sprintf("id: %d", id)}

	// `required` is only legal on a field stored as an offset. Applying it to a
	// scalar is a flatc error, because a scalar is always present — it has a
	// default rather than an absence — so AIP REQUIRED on an int cannot be
	// expressed and is left to the doc comment.
	if f.Required() && offsetTyped(f) {
		attrs = append(attrs, "required")
	}
	if f.Key {
		attrs = append(attrs, "key")
	}
	if f.Shared && f.Kind == buffers.KindString {
		attrs = append(attrs, "shared")
	}

	suffix := ""
	if f.Optional {
		if _, ok := scalar(f.Kind); ok {
			// proto3 explicit presence on a scalar. FlatBuffers spells the same
			// idea `= null`, which makes the generated accessor optional rather
			// than defaulting to zero.
			suffix = " = null"
		}
	}
	return suffix + " (" + strings.Join(attrs, ", ") + ")"
}

// offsetTyped reports whether a field is stored as an offset rather than inline,
// which is what `required` may be applied to.
func offsetTyped(f *buffers.Field) bool {
	if f.Repeated || f.Kind == buffers.KindMap {
		return true
	}
	switch f.Kind {
	case buffers.KindString, buffers.KindBytes, buffers.KindMessage:
		return true
	}
	return false
}
