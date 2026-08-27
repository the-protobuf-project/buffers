package bufir

import (
	"sort"
)

// ordinal.go maps sparse proto field numbers onto the dense, 0-based slots every
// target format actually uses.
//
// # Why a mapping is needed at all
//
// A proto field number is 1-based, may be any value up to 536,870,911, and is
// routinely sparse — deleting a field leaves a hole and adding one appends past
// the end. Cap'n Proto ordinals are 0-based and contiguous: a struct with N
// fields uses exactly 0..N-1, and the compiler rejects anything else. FlatBuffers
// ids are the same shape. So a target cannot simply write the proto number, and
// every target needs the same mapping — which is why it is computed once here
// rather than three times in three renderers that would eventually disagree.
//
// # Why it must be stable, and what makes it so
//
// The mapping is a wire format. A consumer that compiled against a schema where
// `rate_hz` was ordinal 5 will read slot 5 forever; if a later build moves
// `rate_hz` to 4 and puts something else at 5, that consumer silently reads the
// wrong field with no error at any layer. Nothing in proto, Cap'n Proto or
// FlatBuffers detects it.
//
// So the derivation is defined to be append-stable, and a deleted field is
// required to be `reserved`:
//
//	message Sensor {          // fields 1,2,3,4,5,7 with `reserved 6;`
//	  ...                     // ordinals    0,1,2,3,4,6, and 5 held by the
//	}                         // reserved slot so field 7 does not move
//
// Without the `reserved 6;`, field 7 would derive ordinal 5 — the slot the
// deleted field used to own — and every deployed consumer would misread it. That
// is why an unreserved gap is a RuleOrdinal diagnostic rather than a silent
// renumber, and why a repository with published schemas should run with
// `strict=ordinal:error`.
//
// # Why derivation is still not enough
//
// Derivation reconstructs history from what the .proto says today, and a .proto
// only remembers a deletion if someone wrote `reserved`. buffers.lock is the
// ledger that does not depend on anyone remembering: it records the ordinal each
// field was actually assigned, and a later build that would assign a different
// one reports the disagreement instead of shipping it. See lock.go.
//
// Precedence is: an explicit `(buffers.v1.field).ordinal` pin, then the ledger,
// then derivation. A pin outranks the ledger deliberately — it is the escape
// hatch for adopting a schema that predates this plugin, which is exactly the
// case the ledger cannot know about — but a pin that contradicts the ledger is
// reported, because the ledger's whole job is to notice a slot moving.

// maxOrdinalSpace bounds how many slots one message may occupy.
//
// The bound exists because reserved-range expansion is driven by numbers a proto
// author chose, and `reserved 1000 to max;` is a perfectly ordinary way to fence
// off a range. Expanding that literally would mint half a billion placeholder
// slots. Past this bound the build stops expanding, says so, and derives from the
// live fields alone — still deterministic, just no longer able to reconstruct
// deletions on its own, which is what the ledger is for.
const maxOrdinalSpace = 8192

// OrdinalSource records where an assigned ordinal came from. A diagnostic about
// a moved slot is only actionable if it can say whether the old value came from
// the ledger or from a pin.
type OrdinalSource uint8

const (
	// OrdinalDerived was computed from the proto field number. The default, and
	// the only source available on a first run.
	OrdinalDerived OrdinalSource = iota

	// OrdinalLocked was read from buffers.lock.
	OrdinalLocked

	// OrdinalPinned was declared by (buffers.v1.field).ordinal.
	OrdinalPinned
)

// String names the source, for a diagnostic that has to say where a slot came
// from.
func (s OrdinalSource) String() string {
	switch s {
	case OrdinalLocked:
		return "ledger"
	case OrdinalPinned:
		return "pinned"
	}
	return "derived"
}

// reservedRange is one inclusive `reserved N to M;` span.
type reservedRange struct {
	// Start and End bound one inclusive `reserved N to M;` span. Inclusive
	// because that is how the proto source reads and how a diagnostic
	// suggesting a `reserved` line has to phrase it — protoreflect reports the
	// end exclusive and the build converts on the way in.
	Start, End int32
}

// slotInput is one declared field as the assignment sees it. It is deliberately
// not *Field: the assignment is the part most worth testing in isolation, and a
// test that has to build a descriptor to exercise it does not get written.
type slotInput struct {
	// Node identifies the field, for diagnostics.
	Node NodeID
	// Name is the field's proto name, for the ledger's commentary column.
	Name string
	// Number is the proto field number the ordinal is derived from.
	Number int32
	// Pin is an explicitly declared ordinal, meaningful only when HasPin.
	Pin int32
	// HasPin reports whether Pin was declared, since 0 is a legal ordinal.
	HasPin bool
	// Skip reports whether the field is excluded but still holds its slot.
	Skip bool
}

// assignment is one resolved slot.
type assignment struct {
	// Node identifies the field the slot was assigned to.
	Node NodeID
	// Ordinal is the resolved slot.
	Ordinal int32
	// Source records which of derivation, the ledger or a pin decided it.
	Source OrdinalSource
}

// deriveOrdinals maps proto field numbers onto dense 0-based ordinals.
//
// The space is every live field number plus every reserved number that falls
// below the highest live one — the reserved numbers above it cannot affect any
// existing field's slot, and expanding them would only waste ordinals. The
// mapping is then position within that sorted space.
//
// It returns the mapping, the reserved numbers that took a slot, and whether the
// space had to be truncated at maxOrdinalSpace.
func deriveOrdinals(numbers []int32, reserved []reservedRange) (ord map[int32]int32, held []int32, truncated bool) {
	ord = make(map[int32]int32, len(numbers))
	if len(numbers) == 0 {
		return ord, nil, false
	}

	space := make(map[int32]bool, len(numbers))
	var maxLive int32
	for _, n := range numbers {
		space[n] = true
		if n > maxLive {
			maxLive = n
		}
	}

	for _, r := range reserved {
		if r.Start > maxLive {
			// Entirely above the live fields: it cannot displace anything that
			// exists, so expanding it would only burn slots.
			continue
		}
		end := min(r.End, maxLive)
		for n := r.Start; n <= end; n++ {
			if space[n] {
				continue
			}
			if len(space) >= maxOrdinalSpace {
				truncated = true
				break
			}
			space[n] = true
			held = append(held, n)
		}
		if truncated {
			break
		}
	}

	all := make([]int32, 0, len(space))
	for n := range space {
		all = append(all, n)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	for i, n := range all {
		ord[n] = int32(i)
	}
	sort.Slice(held, func(i, j int) bool { return held[i] < held[j] })
	return ord, held, truncated
}

// assignFieldOrdinals resolves every field of one message to a final slot,
// reconciling derivation, the ledger and any pins, and reports what it had to
// reconcile.
//
// It returns the assignments in the order the inputs were given and the reserved
// slots that carry no live field, which the targets emit as placeholders so the
