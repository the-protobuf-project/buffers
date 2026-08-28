package thrift

// struct.go renders a message as a Thrift struct.
//
// The ids are the proto field numbers, unmodified. Both schemes are 1-based,
// sparse, and permanent, so there is no mapping to derive and nothing that can
// drift — which is why this is the one target that does not read Field.Ordinal
// and the one whose output buffers.lock has no authority over.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// structOf renders one message.
func (r *run) structOf(b *emit.Buf, f *buffers.File, m *buffers.Message) {
	var notes []string
	if m.Layout == buffers.LayoutStruct {
		notes = append(notes, "Declared LAYOUT_STRUCT. Thrift has one record kind and no fixed-offset "+
			"variant, so the layout option changes nothing here; the FlatBuffers target honours it.")
	}
	// Before the doc comment, not between it and the struct: Thrift attaches a
	// `/** */` block to the declaration that immediately follows it, and a `#`
	// line in between would break that.
	r.reservedNote(b, m)
	r.doc(b, m.Doc, notes...)

	b.Block(fmt.Sprintf("struct %s {", typeName(string(m.Node), m.Package)), "}", func() {
		r.structFields(b, f, m)
	})
}

// structFields renders a struct's fields in proto field number order.
func (r *run) structFields(b *emit.Buf, f *buffers.File, m *buffers.Message) {
	fields := append([]*buffers.Field(nil), m.Fields...)
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].Number < fields[j].Number })

	unions := map[string]*buffers.Oneof{}
	for _, one := range liveOneofs(m) {
		unions[one.Name] = one
	}
	emitted := map[string]bool{}

	for _, fl := range fields {
		switch {
		case fl.Skip || !allows(fl.Targets):
			// No placeholder is needed and none is emitted: a Thrift id is the
			// proto field number, so an unused id is a gap that nothing slides
			// into. The comment is there so a reader does not reuse it by hand.
			b.Linef("# proto field %d (%s) is excluded from this target; the id stays unused.",
				fl.Number, fl.Name)

		case fl.Oneof != nil && unions[fl.Oneof.Name] != nil:
			one := unions[fl.Oneof.Name]
			if emitted[one.Name] {
				continue
			}
			emitted[one.Name] = true
			r.unionField(b, m, one)

		case fl.Oneof != nil && fl.Oneof.Skip:
			continue

		default:
			// A one-armed oneof lands here, as an ordinary optional field. Thrift
			// requires two members in a union, and a lone arm says no more than
			// "this field, or nothing" — which `optional` already says.
			r.field(b, f, fl)
		}
	}
}

// field renders one ordinary field.
func (r *run) field(b *emit.Buf, f *buffers.File, fl *buffers.Field) {
	typ, diag := r.fieldType(fl, f)
	r.collect(diag)
	if typ == "" {
		return
	}

	r.doc(b, fl.Doc, r.notes(fl)...)
	b.Linef("%d:%s %s %s", fl.Number, modifier(fl), typ, ident(fl.Name))
}

// modifier returns the Thrift requiredness modifier for a field, with a leading
// space, or the empty string when the field takes none.
//
// The mapping is proto3's presence rules, not AIP's. A bare proto3 scalar has no
// presence — it is always written and defaults to zero — which is exactly what a
// Thrift field with no modifier does. Explicit `optional`, a message field and a
// oneof arm all do have presence, and `optional` is what gives Thrift's generated
// code the isset bit that carries it.
//
// `required` is never emitted. It is a permanent wire contract — a reader rejects
// any message lacking the field, forever — and AIP-203 REQUIRED is an API-layer
// rule that services relax routinely. Rendering one as the other would freeze a
// decision the proto did not make, in a format where it cannot be unfrozen.
func modifier(f *buffers.Field) string {
	switch {
	case f.Repeated || f.Kind == buffers.KindMap:
		// An empty list or map is the absent state; there is nothing for a
		// presence bit to add.
		return ""
	case f.Optional, f.Oneof != nil, f.Kind == buffers.KindMessage:
		return " optional"
	}
	return ""
}

// reservedNote records the ids a removed field left behind.
//
// Thrift has no `reserved` declaration and does not need one: an id is the proto
// field number, so a gap stays a gap and nothing after it moves. The note exists
// only so that someone editing the .proto later does not reuse the number — which
// is the one thing that would still break a deployed consumer.
func (r *run) reservedNote(b *emit.Buf, m *buffers.Message) {
	if len(m.Reserved) == 0 {
		return
	}
	slots := append([]buffers.Slot(nil), m.Reserved...)
	sort.SliceStable(slots, func(i, j int) bool { return slots[i].Number < slots[j].Number })
	numbers := make([]string, len(slots))
	for i, slot := range slots {
		numbers[i] = fmt.Sprint(slot.Number)
	}

	b.Linef("# Reserved proto field ids: %s. Thrift has no `reserved` declaration and needs",
		strings.Join(numbers, ", "))
	b.Line("# none: an id here is the proto field number, so an unused one is simply unused")
	b.Line("# and nothing after it shifts. Do not reassign them.")
}
