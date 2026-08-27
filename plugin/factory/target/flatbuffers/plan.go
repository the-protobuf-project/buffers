package flatbuffers

import (
	"fmt"
	"sort"

	"github.com/the-protobuf-project/protokit/buffers"
)

// plan.go turns a message's neutral ordinals into FlatBuffers `id:` attributes.
//
// The two spaces are not the same, for two reasons.
//
// A oneof is N fields in proto and one union field in FlatBuffers. The arms share
// a single slot, so N neutral ordinals collapse to one entry here.
//
// A union field occupies *two* vtable slots: flatc synthesizes a hidden
// `<name>_type` discriminant alongside the value. When ids are assigned
// explicitly — as they always are here — flatc requires the type to take the id
// immediately before the value's, so a union consumes a pair and the id written
// in the schema is the second of them. Getting that wrong is not a compile error
// in every flatc version; it is a schema whose vtable offsets are one slot off
// from what the previous version wrote.
//
// The plan is a straight walk in neutral-ordinal order, which is what makes it
// append-stable: a field added to the .proto gets a higher field number, so a
// higher ordinal, so a slot at the end.

// slotKind distinguishes the three things a FlatBuffers slot can be.
type slotKind uint8

const (
	// slotField is an ordinary field.
	slotField slotKind = iota
	// slotUnion is a oneof rendered as a union, consuming two ids.
	slotUnion
	// slotPlaceholder holds a slot open for a field that no longer exists.
	slotPlaceholder
)

// fbsSlot is one entry in a table's vtable, as this target will render it.
type fbsSlot struct {
	// Kind distinguishes a field from a union or a placeholder.
	Kind  slotKind
	Field *buffers.Field // the field, or a union's first arm
	Oneof *buffers.Oneof // set when Kind is slotUnion
	ID    int32          // the `id:` attribute; for a union, the value's id

	// Why explains a placeholder, so the emitted schema says what is holding the
	// slot rather than leaving an unexplained `__slot_5:byte (deprecated)`.
	Why string
}

// planSlots assigns FlatBuffers ids across a message's fields, reserved slots and
// skipped fields, in neutral-ordinal order.
//
// Skipped and reserved entries both become deprecated placeholders. FlatBuffers'
// `(deprecated)` is exactly the right primitive: it keeps the vtable slot
// allocated and stops the generated code exposing an accessor, which is what
// "this slot is spoken for and you may not read it" means.
func (t *run) planSlots(msg *buffers.Message) []fbsSlot {
	type entry struct {
		ordinal int32
		field   *buffers.Field
		slot    *buffers.Slot
	}

	entries := make([]entry, 0, len(msg.Fields)+len(msg.Reserved))
	for _, f := range msg.Fields {
		entries = append(entries, entry{ordinal: f.Ordinal, field: f})
	}
	for i := range msg.Reserved {
		entries = append(entries, entry{ordinal: msg.Reserved[i].Ordinal, slot: &msg.Reserved[i]})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].ordinal < entries[j].ordinal })

	var (
		out  []fbsSlot
		next int32
		seen = map[string]bool{} // oneofs already given a slot
	)

	for _, e := range entries {
		switch {
		case e.slot != nil:
			out = append(out, fbsSlot{
				Kind: slotPlaceholder,
				ID:   next,
				Why:  fmt.Sprintf("proto field %d was removed and reserved", e.slot.Number),
			})
			next++

		case e.field.Skip:
			out = append(out, fbsSlot{
				Kind: slotPlaceholder,
				ID:   next,
				Why:  fmt.Sprintf("%q is excluded by (buffers.v1.field).skip, and its slot stays spoken for", e.field.Name),
			})
			next++

		case e.field.Oneof != nil:
			one := e.field.Oneof
			if seen[one.Name] {
				// A later arm of a oneof already given its union slot. The arms
				// share one slot, so this ordinal contributes nothing further.
				continue
			}
			seen[one.Name] = true
			if one.Skip {
				out = append(out, fbsSlot{
					Kind: slotPlaceholder,
					ID:   next,
					Why:  fmt.Sprintf("oneof %q is excluded by (buffers.v1.oneof).skip", one.Name),
				})
				next++
				continue
			}
			// The discriminant takes `next`; the value takes `next+1` and is what
			// the schema names.
			out = append(out, fbsSlot{Kind: slotUnion, Field: e.field, Oneof: one, ID: next + 1})
			next += 2

		default:
			out = append(out, fbsSlot{Kind: slotField, Field: e.field, ID: next})
			next++
		}
	}
	return out
}

// unionArms returns a oneof's arms in ordinal order, which is the order their
// discriminant values are assigned in.
//
// FlatBuffers numbers union members from 1, reserving 0 for NONE. That is a
// second ordinal space, and it is positional: reordering the arms changes what
// every existing payload's discriminant means. Ordinal order is proto field
// number order, so an arm added to the oneof lands at the end and the existing
// discriminants keep their meaning.
func unionArms(one *buffers.Oneof) []*buffers.Field {
	arms := make([]*buffers.Field, 0, len(one.Fields))
	for _, f := range one.Fields {
		if f.Skip {
			continue
		}
		arms = append(arms, f)
	}
	sort.SliceStable(arms, func(i, j int) bool { return arms[i].Ordinal < arms[j].Ordinal })
	return arms
}
