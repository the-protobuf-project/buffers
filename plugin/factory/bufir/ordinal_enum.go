package bufir

// ordinal_enum.go assigns enum value ordinals, and renders a run of field numbers
// as proto `reserved` syntax for the diagnostics that suggest one.
//
// Enum values are separate from fields because they have no reserved-range
// machinery to reconcile: a removed enumerant is rarer, and the ledger is the only
// thing that remembers it.

import (
	"fmt"
	"sort"
)

// assignEnumOrdinals resolves an enum's values to the contiguous 0-based
// positions Cap'n Proto requires.
//
// Proto enum values are neither contiguous nor necessarily 0-based — AIP-126
// guarantees a zero value exists, and nothing more — so the mapping is position
// within the sorted values. Unlike fields, there is no reserved-range machinery:
// a removed enumerant is rarer, and the ledger covers it.
func assignEnumOrdinals(values []slotInput, locked map[int32]int32, report func(Diagnostic)) []assignment {
	sorted := make([]slotInput, len(values))
	copy(sorted, values)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Number < sorted[j].Number })

	derived := make(map[int32]int32, len(sorted))
	for i, v := range sorted {
		// A proto enum may declare the same number twice when allow_alias is set.
		// The first declaration owns the ordinal; an alias shares it, which is
		// what an alias means.
		if _, seen := derived[v.Number]; !seen {
			derived[v.Number] = int32(i)
		}
	}

	out := make([]assignment, len(values))
	for i, v := range values {
		ordinal, source := derived[v.Number], OrdinalDerived
		if got, ok := locked[v.Number]; ok {
			if got != ordinal {
				report(Diagnostic{
					Rule: RuleOrdinal,
					Node: v.Node,
					Message: fmt.Sprintf("buffers.lock records ordinal %d for enum value %d, but this build derives %d; the ledger wins",
						got, v.Number, ordinal),
					Hint: "an enum value was probably removed; Cap'n Proto enumerants are positional, so removing one moves every later value",
				})
			}
			ordinal, source = got, OrdinalLocked
		}
		if v.HasPin {
			ordinal, source = v.Pin, OrdinalPinned
		}
		out[i] = assignment{Node: v.Node, Ordinal: ordinal, Source: source}
	}
	return out
}

// joinNumbers renders a list of field numbers as a proto `reserved` body,
// collapsing runs into ranges so the hint can be pasted straight in.
func joinNumbers(ns []int32) string {
	if len(ns) == 0 {
		return ""
	}
	var out string
	start, prev := ns[0], ns[0]
	flush := func() {
		if out != "" {
			out += ", "
		}
		switch {
		case start == prev:
			out += fmt.Sprint(start)
		case prev == start+1:
			out += fmt.Sprintf("%d, %d", start, prev)
		default:
			out += fmt.Sprintf("%d to %d", start, prev)
		}
	}
	for _, n := range ns[1:] {
		if n == prev+1 {
			prev = n
			continue
		}
		flush()
		start, prev = n, n
	}
	flush()
	return out
}
