package flatbuffers

// plan_test.go covers the mapping from neutral ordinals to FlatBuffers ids.
//
// It is the subtlest logic in this target and the least visible in a golden file:
// `id: 4` looks equally correct whichever rule produced it. The rule that matters
// is that a union consumes *two* ids — flatc synthesizes a hidden discriminant —
// and that getting it wrong is not a compile error but a vtable whose offsets are
// one slot off from what the previous build wrote.

import (
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
)

// field builds a minimal field for planning.
func field(name string, number, ordinal int32) *buffers.Field {
	return &buffers.Field{
		Node:    buffers.NodeID("m." + name),
		Name:    name,
		Number:  number,
		Ordinal: ordinal,
		Kind:    buffers.KindString,
	}
}

// plan runs planSlots over a message with no schema behind it, which is all the
// slot assignment needs.
func plan(m *buffers.Message) []fbsSlot {
	r := &run{}
	return r.planSlots(m)
}

func TestPlainFieldsTakeConsecutiveIds(t *testing.T) {
	m := &buffers.Message{
		Name:   "M",
		Fields: []*buffers.Field{field("a", 1, 0), field("b", 2, 1), field("c", 3, 2)},
	}

	got := plan(m)
	if len(got) != 3 {
		t.Fatalf("got %d slots, want 3", len(got))
	}
	for i, s := range got {
		if s.Kind != slotField || s.ID != int32(i) {
			t.Errorf("slot %d: kind=%v id=%d, want a field at id %d", i, s.Kind, s.ID, i)
		}
	}
}

func TestUnionConsumesTwoIdsAndNamesTheSecond(t *testing.T) {
	// flatc requires the hidden discriminant to take the id immediately before the
	// value's when ids are explicit. So the union occupies a pair, and the id
	// written into the schema is the second of them.
	one := &buffers.Oneof{Node: "m.p", Name: "payload", UnionName: "MPayload"}
	x, y := field("x", 2, 1), field("y", 3, 2)
	x.Oneof, y.Oneof = one, one
	one.Fields = []*buffers.Field{x, y}

	m := &buffers.Message{
		Name:   "M",
		Fields: []*buffers.Field{field("a", 1, 0), x, y, field("b", 4, 3)},
		Oneofs: []*buffers.Oneof{one},
	}

	got := plan(m)
	if len(got) != 3 {
		t.Fatalf("got %d slots, want 3 (a, the union, b) — the arms share one slot", len(got))
	}

	if got[0].Kind != slotField || got[0].ID != 0 {
		t.Errorf("slot 0 = %+v, want field a at id 0", got[0])
	}
	if got[1].Kind != slotUnion {
		t.Fatalf("slot 1 kind = %v, want a union", got[1].Kind)
	}
	if got[1].ID != 2 {
		t.Errorf("union id = %d, want 2 — the discriminant takes 1 and the value takes 2", got[1].ID)
	}
	// The field after the union must clear both of its ids.
	if got[2].Kind != slotField || got[2].ID != 3 {
		t.Errorf("slot after the union = %+v, want field b at id 3", got[2])
	}
}

func TestReservedHoldsAnIdOpen(t *testing.T) {
	// A removed field's slot must stay occupied, or every field after it shifts
	// down one and existing readers misread them.
	m := &buffers.Message{
		Name:     "M",
		Fields:   []*buffers.Field{field("a", 1, 0), field("c", 3, 2)},
		Reserved: []buffers.Slot{{Ordinal: 1, Number: 2}},
	}

	got := plan(m)
	if len(got) != 3 {
		t.Fatalf("got %d slots, want 3", len(got))
	}
	if got[1].Kind != slotPlaceholder || got[1].ID != 1 {
		t.Errorf("slot 1 = %+v, want a placeholder at id 1", got[1])
	}
	if got[2].ID != 2 {
		t.Errorf("field after the placeholder has id %d, want 2", got[2].ID)
	}
	if got[1].Why == "" {
		t.Error("the placeholder carries no explanation; the emitted schema would show an unexplained slot")
	}
}

func TestSkippedFieldStillConsumesItsId(t *testing.T) {
	// Excluding a field does not free its slot: a later run that unskips it has to
	// land on the same one.
	skipped := field("b", 2, 1)
	skipped.Skip = true

	m := &buffers.Message{
		Name:   "M",
		Fields: []*buffers.Field{field("a", 1, 0), skipped, field("c", 3, 2)},
	}

	got := plan(m)
	if got[1].Kind != slotPlaceholder {
		t.Errorf("a skipped field yielded %v, want a placeholder holding its id", got[1].Kind)
	}
	if got[2].ID != 2 {
		t.Errorf("the field after a skipped one has id %d, want 2", got[2].ID)
	}
}
