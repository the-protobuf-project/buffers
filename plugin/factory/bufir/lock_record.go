package bufir

// lock_record.go accumulates a build's assignments into a fresh ledger, renders
// it, and compares two of them.
//
// Recording is separate from reading so the two never alias: a build reads the
// old ledger to resolve ordinals and writes a new one describing what it actually
// assigned. Mutating the source mid-build would make the second half of a run
// disagree with the first.

import (
	"bytes"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Recorder accumulates a build's assignments into a fresh ledger.
//
// It is separate from the ledger being read so that the two never alias: a build
// reads the old ledger to resolve ordinals and writes a new one describing what
// it actually assigned, and mutating the source mid-build would make the second
// half of the run disagree with the first.
type Recorder struct {
	// out is the ledger being accumulated.
	out *Lock
}

// NewRecorder starts an empty ledger to record into.
func NewRecorder() *Recorder { return &Recorder{out: NewLock()} }

// Message records one message's assignments.
func (r *Recorder) Message(node NodeID, fields []SlotEntry) {
	if len(fields) == 0 {
		return
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Number < fields[j].Number })
	r.out.Messages = append(r.out.Messages, MessageSlots{Node: node, Fields: fields})
}

// Enum records one enum's assignments.
func (r *Recorder) Enum(node NodeID, values []SlotEntry) {
	if len(values) == 0 {
		return
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Number < values[j].Number })
	r.out.Enums = append(r.out.Enums, EnumSlots{Node: node, Values: values})
}

// Service records one service's assignments. Methods keep declaration order,
// which is also ordinal order, because sorting them by name would put the file's
// most-diffed section into an order unrelated to the source.
func (r *Recorder) Service(node NodeID, methods []MethodEntry) {
	if len(methods) == 0 {
		return
	}
	r.out.Services = append(r.out.Services, ServiceSlots{Node: node, Methods: methods})
}

// Lock returns the recorded ledger with its sections in a stable order.
func (r *Recorder) Lock() *Lock {
	sort.Slice(r.out.Messages, func(i, j int) bool { return r.out.Messages[i].Node < r.out.Messages[j].Node })
	sort.Slice(r.out.Enums, func(i, j int) bool { return r.out.Enums[i].Node < r.out.Enums[j].Node })
	sort.Slice(r.out.Services, func(i, j int) bool { return r.out.Services[i].Node < r.out.Services[j].Node })
	r.out.reindex()
	return r.out
}

// Marshal renders the ledger as YAML, with a header explaining what the file is
// to whoever meets it first in a pull request.
func (l *Lock) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(lockHeader)

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(l); err != nil {
		return nil, fmt.Errorf("marshal ledger: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("marshal ledger: %w", err)
	}
	return buf.Bytes(), nil
}

// Equal reports whether two ledgers record the same assignments, ignoring the
// Name fields, which are commentary.
//
// It is what a verify-only run compares, and what a test asserts to show that a
// rebuild of unchanged protos moved nothing.
func (l *Lock) Equal(other *Lock) bool {
	if other == nil {
		return false
	}
	if !sameSlots(l.msgIdx, other.msgIdx) || !sameSlots(l.enumIdx, other.enumIdx) {
		return false
	}
	if len(l.svcIdx) != len(other.svcIdx) {
		return false
	}
	for node, mine := range l.svcIdx {
		theirs, ok := other.svcIdx[node]
		if !ok || len(mine) != len(theirs) {
			return false
		}
		for name, ordinal := range mine {
			if theirs[name] != ordinal {
				return false
			}
		}
	}
	return true
}

// sameSlots compares two node-keyed slot indexes.
func sameSlots(a, b map[NodeID]map[int32]int32) bool {
	if len(a) != len(b) {
		return false
	}
	for node, mine := range a {
		theirs, ok := b[node]
		if !ok || len(mine) != len(theirs) {
			return false
		}
		for number, ordinal := range mine {
			if theirs[number] != ordinal {
				return false
			}
		}
	}
	return true
}

// lockHeader is the preamble written above the ledger, explaining the file to
// whoever meets it first in a pull request.
const lockHeader = `# buffers.lock — the ordinal ledger. Generated; commit it.
#
# Every field, enum value and method this plugin has assigned a target slot to is
# recorded here by its proto field number. A later build that would assign a
# different slot reports the disagreement instead of shipping it.
#
# That matters because the slot is a wire format. A consumer compiled against a
# schema where a field sat at ordinal 5 reads ordinal 5 forever, and if a rebuild
# moves something else there, nothing — not protoc, not capnp, not flatc, not the
# consumer — reports an error. It just reads the wrong field.
#
# Do not edit by hand, and do not resolve a merge conflict in this file textually.
# A conflict here means two branches each assigned a slot; deciding which one wins
# is a compatibility decision, so .gitattributes marks the file -merge to keep git
# from guessing.
`
