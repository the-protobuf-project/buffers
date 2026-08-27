package capnp

// struct.go renders a message as a Cap'n Proto struct, and lays out its slots.

import (
	"fmt"
	"sort"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// structOf renders a message, its nested types, and the entry structs its map
// fields need.
func (r *run) structOf(b *emit.Buf, f *bufir.File, m *bufir.Message) {
	b.Doc("#", m.Doc)
	if m.Layout == bufir.LayoutStruct {
		b.Line("# Declared LAYOUT_STRUCT. Cap'n Proto has no table/struct distinction —")
		b.Line("# every struct is already a flat, fixed-offset record — so the layout option")
		b.Line("# changes nothing here. It is honoured by the FlatBuffers target.")
	}

	b.Block(fmt.Sprintf("struct %s @0x%016x {", typeName(m.Name), m.CapnpID), "}", func() {
		r.structFields(b, f, m)

		for _, nested := range m.Nested {
			if nested.Skip || nested.IsMapEntry || !allows(nested.Targets) {
				continue
			}
			b.Line("")
			r.structOf(b, f, nested)
		}
		for _, e := range m.Enums {
			if e.Skip {
				continue
			}
			b.Line("")
			r.enum(b, e)
		}
		for _, entry := range r.mapEntries(m) {
			b.Line("")
			r.mapEntry(b, f, entry)
		}
	})
}

// structFields renders a struct's fields, unions and reserved placeholders in
// ordinal order, then checks the result is a legal Cap'n Proto ordinal space.
func (r *run) structFields(b *emit.Buf, f *bufir.File, m *bufir.Message) {
	type entry struct {
		ordinal int32
		field   *bufir.Field
		slot    *bufir.Slot
	}

	entries := make([]entry, 0, len(m.Fields)+len(m.Reserved))
	for _, fl := range m.Fields {
		entries = append(entries, entry{ordinal: fl.Ordinal, field: fl})
	}
	for i := range m.Reserved {
		entries = append(entries, entry{ordinal: m.Reserved[i].Ordinal, slot: &m.Reserved[i]})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].ordinal < entries[j].ordinal })

	used := map[int32]bool{}
	seenUnion := map[string]bool{}

	for _, e := range entries {
		switch {
		case e.slot != nil:
			// Cap'n Proto has no `reserved`. A Void field occupies no data bits
			// and no pointer, so it holds the ordinal open at zero cost — which is
			// exactly what the slot needs to do.
			b.Linef("# proto field %d was removed and reserved; this holds its ordinal.", e.slot.Number)
			b.Linef("removed%d @%d :Void;", e.slot.Number, e.ordinal)
			used[e.ordinal] = true

		case e.field.Skip || !allows(e.field.Targets):
			b.Linef("# %q is excluded from this target; this holds its ordinal.", e.field.Name)
			b.Linef("excluded%d @%d :Void;", e.field.Number, e.ordinal)
			used[e.ordinal] = true

		case e.field.Oneof != nil:
			one := e.field.Oneof
			if seenUnion[one.Name] || one.Skip {
				if one.Skip {
					b.Linef("excluded%d @%d :Void;", e.field.Number, e.ordinal)
					used[e.ordinal] = true
				}
				continue
			}
			seenUnion[one.Name] = true
			r.union(b, f, one, used)

		default:
			r.field(b, f, e.field)
			used[e.ordinal] = true
		}
	}

	r.checkContiguous(m, used)
}

// field renders one ordinary field.
func (r *run) field(b *emit.Buf, f *bufir.File, fl *bufir.Field) {
	typ, diag := r.fieldType(fl, f)
	r.collect(diag)

	b.Doc("#", fl.Doc)
	// AIP REQUIRED has no Cap'n Proto expression — every field is always readable
	// and carries a default — so it is noted rather than dropped silently. One
	// line, not three: on an AIP-shaped schema most fields carry a behavior, and
	// a three-line caveat on each buries the field docs that matter.
	if fl.Required() {
		b.Line("# REQUIRED (AIP-203); Cap'n Proto cannot enforce it.")
	}
	b.Linef("%s @%d :%s;", member(fl.Name), fl.Ordinal, typ)
}

// checkContiguous verifies the struct's ordinals run 0..N-1 with no gaps.
//
// capnp enforces this itself, but its message names a line in a generated file
// and this one names the message and the missing ordinal. The check can only fail
// when explicit pins are in play; derivation is dense by construction.
func (r *run) checkContiguous(m *bufir.Message, used map[int32]bool) {
	if len(used) == 0 {
		return
	}
	var maxOrd int32
	for ord := range used {
		if ord > maxOrd {
			maxOrd = ord
		}
	}
	var missing []int32
	for i := int32(0); i <= maxOrd; i++ {
		if !used[i] {
			missing = append(missing, i)
		}
	}
	if len(missing) == 0 {
		return
	}
	r.collect(&bufir.Diagnostic{
		Rule: bufir.RuleOrdinal,
		Node: m.Node,
		Message: fmt.Sprintf("Cap'n Proto requires ordinals 0..%d with no gaps; %v %s unused",
			maxOrd, missing, plural(len(missing))),
		Hint: "an explicit (buffers.v1.field).ordinal left a hole; remove the pin and let the ordinal be derived",
	})
}

// plural picks the verb agreeing with a count, so a diagnostic reads as prose.
func plural(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}
