package flatbuffers

// plan_union_test.go covers what a oneof does to the slot space: where the union
// lands when its arms are not contiguous, what a skipped one holds, and the order
// its arms are numbered in.

import (
	"testing"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
)

func TestArmsInterleavedWithPlainFields(t *testing.T) {
	// A proto oneof's arms need not be contiguous in field number, so the union
	// lands at its *first* arm's position and the later arms contribute nothing.
	// Appending an arm therefore does not move the union or anything after it.
	one := &bufir.Oneof{Node: "m.p", Name: "p", UnionName: "MP"}
	x, y := field("x", 2, 1), field("y", 4, 3)
	x.Oneof, y.Oneof = one, one
	one.Fields = []*bufir.Field{x, y}

	b := field("b", 3, 2)
	m := &bufir.Message{
		Name:   "M",
		Fields: []*bufir.Field{field("a", 1, 0), x, b, y},
		Oneofs: []*bufir.Oneof{one},
	}

	got := plan(m)
	if len(got) != 3 {
		t.Fatalf("got %d slots, want 3 (a, union, b)", len(got))
	}
	if got[1].Kind != slotUnion || got[1].ID != 2 {
		t.Errorf("union = %+v, want id 2 at the first arm's position", got[1])
	}
	if got[2].Kind != slotField || got[2].Field.Name != "b" || got[2].ID != 3 {
		t.Errorf("slot 2 = %+v, want field b at id 3 — after both of the union's ids", got[2])
	}
}

func TestSkippedOneofHoldsOneId(t *testing.T) {
	one := &bufir.Oneof{Node: "m.p", Name: "p", UnionName: "MP", Skip: true}
	x := field("x", 2, 1)
	x.Oneof = one
	one.Fields = []*bufir.Field{x}

	m := &bufir.Message{
		Name:   "M",
		Fields: []*bufir.Field{field("a", 1, 0), x, field("c", 3, 2)},
		Oneofs: []*bufir.Oneof{one},
	}

	got := plan(m)
	if got[1].Kind != slotPlaceholder {
		t.Errorf("a skipped oneof yielded %v, want a placeholder", got[1].Kind)
	}
	// One id, not two: nothing is emitted, so there is no discriminant to hold a
	// slot for.
	if got[2].ID != 2 {
		t.Errorf("the field after a skipped oneof has id %d, want 2", got[2].ID)
	}
}

func TestUnionArmsAreOrderedByOrdinal(t *testing.T) {
	// FlatBuffers numbers union members from 1, positionally. Reordering the arms
	// changes what every existing payload's discriminant means; ordinal order is
	// proto field number order, so appending is safe.
	one := &bufir.Oneof{Node: "m.p", Name: "p"}
	late, early := field("late", 9, 8), field("early", 2, 1)
	one.Fields = []*bufir.Field{late, early}

	got := unionArms(one)
	if len(got) != 2 || got[0].Name != "early" || got[1].Name != "late" {
		t.Errorf("arms = %v, want them sorted by ordinal", armNames(got))
	}
}

func TestUnionArmsSkipExcluded(t *testing.T) {
	one := &bufir.Oneof{Node: "m.p", Name: "p"}
	keep, drop := field("keep", 2, 1), field("drop", 3, 2)
	drop.Skip = true
	one.Fields = []*bufir.Field{keep, drop}

	if got := unionArms(one); len(got) != 1 || got[0].Name != "keep" {
		t.Errorf("arms = %v, want only the kept one", armNames(got))
	}
}

func armNames(fs []*bufir.Field) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}
