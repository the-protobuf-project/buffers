package bufir

// ordinal_assign_test.go covers precedence: which of derivation, the ledger and
// an explicit pin decides a slot when they disagree, and whether the
// disagreement is reported.

import (
	"strings"
	"testing"
)

func TestLedgerOutranksDerivationAndReportsTheDisagreement(t *testing.T) {
	// A field was deleted without being reserved, but the ledger remembers where
	// the survivors sat. The ledger has to win: a deployed consumer is still
	// reading those slots.
	fields := []slotInput{
		{Node: "m.a", Name: "a", Number: 1},
		{Node: "m.b", Name: "b", Number: 4},
	}
	locked := map[int32]int32{1: 0, 4: 3}

	var diags []Diagnostic
	got, _ := assignFieldOrdinals("m", fields, nil, locked, func(d Diagnostic) { diags = append(diags, d) })

	if got[1].Ordinal != 3 {
		t.Errorf("field 4 at ordinal %d, want 3 from the ledger", got[1].Ordinal)
	}
	if got[1].Source != OrdinalLocked {
		t.Errorf("source %v, want %v", got[1].Source, OrdinalLocked)
	}
	if len(diags) == 0 {
		t.Fatal("the ledger silently overrode derivation with no diagnostic")
	}
}

func TestPinOutranksTheLedger(t *testing.T) {
	// The adoption escape hatch: a .capnp that predates this plugin already has
	// slots, and only the author knows them.
	fields := []slotInput{{Node: "m.a", Name: "a", Number: 1, Pin: 7, HasPin: true}}

	var diags []Diagnostic
	got, _ := assignFieldOrdinals("m", fields, nil, map[int32]int32{1: 0},
		func(d Diagnostic) { diags = append(diags, d) })

	if got[0].Ordinal != 7 || got[0].Source != OrdinalPinned {
		t.Errorf("got ordinal %d from %v, want 7 from %v", got[0].Ordinal, got[0].Source, OrdinalPinned)
	}
	if len(diags) == 0 {
		t.Error("a pin contradicting the ledger must still be reported")
	}
}

func TestCollidingPinsAreReported(t *testing.T) {
	fields := []slotInput{
		{Node: "m.a", Name: "a", Number: 1, Pin: 2, HasPin: true},
		{Node: "m.b", Name: "b", Number: 2, Pin: 2, HasPin: true},
	}
	var diags []Diagnostic
	assignFieldOrdinals("m", fields, nil, nil, func(d Diagnostic) { diags = append(diags, d) })

	var found bool
	for _, d := range diags {
		if strings.Contains(d.Message, "already taken") {
			found = true
		}
	}
	if !found {
		t.Errorf("two fields on one slot went unreported; diagnostics: %v", diags)
	}
}

func TestEnumOrdinalsAreContiguousFromZero(t *testing.T) {
	// Cap'n Proto enumerants are positional. A proto enum's numbers need not be.
	values := []slotInput{
		{Node: "e.UNSPECIFIED", Number: 0},
		{Node: "e.LIDAR", Number: 1},
		{Node: "e.LEGACY", Number: 40},
	}
	got := assignEnumOrdinals(values, nil, func(Diagnostic) {})
	for i, want := range []int32{0, 1, 2} {
		if got[i].Ordinal != want {
			t.Errorf("value %d: ordinal %d, want %d", i, got[i].Ordinal, want)
		}
	}
}

func TestJoinNumbersCollapsesRunsIntoReservedSyntax(t *testing.T) {
	// The hint is meant to be pasted straight into the .proto, so it has to be
	// valid `reserved` syntax.
	for _, tc := range []struct {
		in   []int32
		want string
	}{
		{[]int32{3}, "3"},
		{[]int32{3, 4}, "3, 4"},
		{[]int32{3, 4, 5}, "3 to 5"},
		{[]int32{3, 4, 5, 9}, "3 to 5, 9"},
		{[]int32{2, 7, 8, 9, 20}, "2, 7 to 9, 20"},
	} {
		if got := joinNumbers(tc.in); got != tc.want {
			t.Errorf("joinNumbers(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
