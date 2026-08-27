package bufir

// ordinal_assign.go reconciles the three things that can decide a field's slot:
// what derivation computes, what the ledger remembers, and what an author pinned.
//
// The derivation in ordinal.go knows about none of them — it maps numbers to
// positions and stops. Precedence is applied here, and a disagreement between the
// three becomes a diagnostic rather than a silent choice.

import (
	"fmt"
	"sort"
)

// assignFieldOrdinals resolves every field of one message to a final slot,
// reconciling derivation, the ledger and any pins, and reports what it had to
// reconcile.
//
// It returns the assignments in the order the inputs were given, and the
// reserved slots carrying no live field — which the targets emit as
// placeholders so the fields after them do not move.
func assignFieldOrdinals(
	owner NodeID,
	fields []slotInput,
	reserved []reservedRange,
	locked map[int32]int32,
	report func(Diagnostic),
) ([]assignment, []Slot) {
	numbers := make([]int32, len(fields))
	for i, f := range fields {
		numbers[i] = f.Number
	}
	derived, held, truncated := deriveOrdinals(numbers, reserved)

	if truncated {
		report(Diagnostic{
			Rule: RuleOrdinal,
			Node: owner,
			Message: fmt.Sprintf("reserved ranges would occupy more than %d ordinals, so they were not expanded; "+
				"ordinals are derived from the live fields alone and cannot reconstruct a deleted field's slot", maxOrdinalSpace),
			Hint: "narrow the reserved ranges to the numbers actually used, or rely on buffers.lock to hold the slots",
		})
	}

	reportUnreservedGaps(owner, numbers, derived, report)

	out := make([]assignment, len(fields))
	byOrdinal := map[int32]NodeID{}

	for i, f := range fields {
		ordinal, source := derived[f.Number], OrdinalDerived

		if got, ok := locked[f.Number]; ok {
			if got != ordinal {
				report(Diagnostic{
					Rule: RuleOrdinal,
					Node: f.Node,
					Message: fmt.Sprintf("buffers.lock records ordinal %d for field number %d, but this build derives %d; "+
						"the ledger wins, because a consumer compiled against %d is still reading it", got, f.Number, ordinal, got),
					Hint: "a field was probably removed without a `reserved` declaration — add it, and the two will agree again",
				})
			}
			ordinal, source = got, OrdinalLocked
		}

		if f.HasPin {
			if source == OrdinalLocked && f.Pin != ordinal {
				report(Diagnostic{
					Rule: RuleOrdinal,
					Node: f.Node,
					Message: fmt.Sprintf("(buffers.v1.field).ordinal pins %d, but buffers.lock records %d; "+
						"the pin wins and the ledger will be rewritten", f.Pin, ordinal),
					Hint: "if the pin is correct this is a one-time adoption; if it is not, every consumer reading slot " +
						fmt.Sprint(ordinal) + " will start reading the wrong field",
				})
			}
			ordinal, source = f.Pin, OrdinalPinned
		}

		if ordinal < 0 {
			report(Diagnostic{
				Rule:    RuleOrdinal,
				Node:    f.Node,
				Message: fmt.Sprintf("ordinal %d is negative; target ordinals are 0-based", ordinal),
				Hint:    "proto field numbers start at 1 and ordinals at 0 — a pin is not a field number",
			})
			ordinal = 0
		}

		if other, clash := byOrdinal[ordinal]; clash {
			report(Diagnostic{
				Rule:    RuleOrdinal,
				Node:    f.Node,
				Message: fmt.Sprintf("ordinal %d is already taken by %s", ordinal, other),
				Hint:    "two fields on one slot cannot both be read; remove one of the (buffers.v1.field).ordinal pins",
			})
		}
		byOrdinal[ordinal] = f.Node

		out[i] = assignment{Node: f.Node, Ordinal: ordinal, Source: source}
	}

	slots := make([]Slot, 0, len(held))
	for _, number := range held {
		ordinal := derived[number]
		if _, taken := byOrdinal[ordinal]; taken {
			// A pin landed on the slot a reserved number was holding. The pin
			// wins — it was reported above — and the placeholder would collide.
			continue
		}
		slots = append(slots, Slot{Ordinal: ordinal, Number: number})
	}
	return out, slots
}

// reportUnreservedGaps flags a hole in the field numbers that no `reserved`
// declaration accounts for.
//
// A hole is not itself wrong — an author may simply have started at 1 and skipped
// 4 on a whim — but it is indistinguishable from a field that was deleted without
// being reserved, and that case silently moves every later field's slot. Naming
// the specific `reserved` line to add is the whole value of the diagnostic.
func reportUnreservedGaps(owner NodeID, numbers []int32, derived map[int32]int32, report func(Diagnostic)) {
	if len(numbers) < 2 {
		return
	}
	sorted := append([]int32(nil), numbers...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var gaps []int32
	for i := 1; i < len(sorted); i++ {
		prev, cur := sorted[i-1], sorted[i]
		for n := prev + 1; n < cur; n++ {
			if _, expanded := derived[n]; expanded {
				continue // a reserved range already accounts for it
			}
			gaps = append(gaps, n)
			if len(gaps) >= 16 {
				break
			}
		}
		if len(gaps) >= 16 {
			break
		}
	}
	if len(gaps) == 0 {
		return
	}
	report(Diagnostic{
		Rule: RuleOrdinal,
		Node: owner,
		Message: fmt.Sprintf("field numbers %s are unused and not reserved, so ordinals are assigned as if they never existed",
			joinNumbers(gaps)),
		Hint: fmt.Sprintf("if a field was removed there, add `reserved %s;` — without it every field after the hole "+
			"has shifted down one slot and existing consumers now read the wrong one", joinNumbers(gaps)),
	})
}
