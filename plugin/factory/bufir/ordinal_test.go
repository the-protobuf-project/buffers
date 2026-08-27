package bufir

// ordinal_test.go covers the derivation: sparse proto field numbers in, dense
// 0-based slots out.
//
// Each case is written as a before/after pair where the "after" is an ordinary
// edit to a .proto, and the assertion is about what happened to the slots that
// already existed.

import (
	"strings"
	"testing"
)

func TestDeriveOrdinalsIsDenseAndZeroBased(t *testing.T) {
	// Proto numbers are 1-based; every target's slots are 0-based.
	ord, held, truncated := deriveOrdinals([]int32{1, 2, 3}, nil)
	if truncated {
		t.Fatal("truncated a three-field message")
	}
	if len(held) != 0 {
		t.Fatalf("held %v, want none", held)
	}
	for number, want := range map[int32]int32{1: 0, 2: 1, 3: 2} {
		if got := ord[number]; got != want {
			t.Errorf("field %d: ordinal %d, want %d", number, got, want)
		}
	}
}

func TestDeriveOrdinalsSkipsLeadingNumbers(t *testing.T) {
	// Starting at 5 is legal and self-consistent: nothing below it ever existed,
	// so nothing can have moved.
	ord, _, _ := deriveOrdinals([]int32{5, 6, 7}, nil)
	for number, want := range map[int32]int32{5: 0, 6: 1, 7: 2} {
		if got := ord[number]; got != want {
			t.Errorf("field %d: ordinal %d, want %d", number, got, want)
		}
	}
}

func TestReservedHoldsTheSlotOfADeletedField(t *testing.T) {
	// This is the whole point of the reserved expansion. Before: fields 1..5.
	// After: field 3 deleted and reserved.
	before, _, _ := deriveOrdinals([]int32{1, 2, 3, 4, 5}, nil)
	after, held, _ := deriveOrdinals([]int32{1, 2, 4, 5}, []reservedRange{{Start: 3, End: 3}})

	for _, number := range []int32{1, 2, 4, 5} {
		if before[number] != after[number] {
			t.Errorf("field %d moved from ordinal %d to %d despite being reserved",
				number, before[number], after[number])
		}
	}
	if len(held) != 1 || held[0] != 3 {
		t.Fatalf("held %v, want [3] — the deleted field's slot must stay occupied", held)
	}
	if after[3] != 2 {
		t.Errorf("reserved 3 holds ordinal %d, want 2", after[3])
	}
}

func TestUnreservedDeletionMovesEveryLaterField(t *testing.T) {
	// The failure this package exists to make visible. Same edit as above, but
	// without the `reserved 3;`.
	before, _, _ := deriveOrdinals([]int32{1, 2, 3, 4, 5}, nil)
	after, _, _ := deriveOrdinals([]int32{1, 2, 4, 5}, nil)

	if before[4] == after[4] {
		t.Fatal("expected field 4 to shift; if it did not, this test no longer covers the hazard")
	}
	if after[4] != 2 || after[5] != 3 {
		t.Errorf("fields 4,5 at ordinals %d,%d, want 2,3", after[4], after[5])
	}

	// And the build must say so rather than shipping it.
	var diags []Diagnostic
	reportUnreservedGaps("m", []int32{1, 2, 4, 5}, after, func(d Diagnostic) { diags = append(diags, d) })
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(diags))
	}
	if !strings.Contains(diags[0].Hint, "reserved 3;") {
		t.Errorf("hint %q does not name the reserved line to add", diags[0].Hint)
	}
}

func TestReservedAboveTheLiveFieldsIsNotExpanded(t *testing.T) {
	// `reserved 100 to max;` is an ordinary way to fence off a range. Expanding it
	// literally would mint half a billion slots, and it cannot affect any field
	// that exists.
	ord, held, truncated := deriveOrdinals([]int32{1, 2, 3}, []reservedRange{{Start: 100, End: 536870911}})
	if truncated {
		t.Error("truncated on a range that should have been skipped outright")
	}
	if len(held) != 0 {
		t.Errorf("held %d slots, want 0", len(held))
	}
	if len(ord) != 3 {
		t.Errorf("ordinal space has %d entries, want 3", len(ord))
	}
}

func TestOrdinalSpaceIsBounded(t *testing.T) {
	// A reserved range interleaved with a very high field number is the case that
	// can actually blow up, since the expansion is bounded by the live maximum.
	_, held, truncated := deriveOrdinals([]int32{1, 60000}, []reservedRange{{Start: 2, End: 59999}})
	if !truncated {
		t.Fatal("expected truncation")
	}
	if len(held) > maxOrdinalSpace {
		t.Errorf("held %d slots, want at most %d", len(held), maxOrdinalSpace)
	}
}
