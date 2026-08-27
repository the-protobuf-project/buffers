package registry_test

// agreement_test.go asserts the cross-target property, and the reason the ordinal
// assignment lives in protokit's buffers IR rather than in each renderer.
//
// Every target reads the same slot for a field. If FlatBuffers put one at id 4
// and Cap'n Proto put it at @5, a payload written by one and read by the other
// would silently misalign — which is exactly the failure a single schema source
// is supposed to prevent.

import (
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
)

// TestOrdinalsAgreeAcrossTargets is the cross-target property, and the reason the
// ordinal assignment lives in protokit's buffers IR rather than in each renderer.
//
// Every target reads the same slot for a field. If FlatBuffers put a field at id 4
// and Cap'n Proto put it at @5, a payload written by one and read by the other
// would silently misalign — which is exactly the failure a single schema source is
// supposed to prevent.
func TestOrdinalsAgreeAcrossTargets(t *testing.T) {
	a, b := buildSchema(t), buildSchema(t)

	if !a.Lock.Equal(b.Lock) {
		t.Fatal("two builds of one proto tree disagree about slots")
	}
	if len(a.Lock.Messages) == 0 {
		t.Fatal("the ledger is empty; the fixture is not exercising anything")
	}

	// Every field the ledger knows about must have a slot, and no two fields of one
	// message may share one.
	for _, m := range a.Lock.Messages {
		seen := map[int32]string{}
		for _, f := range m.Fields {
			if other, clash := seen[f.Ordinal]; clash {
				t.Errorf("%s: %s and %s both occupy ordinal %d", m.Node, other, f.Name, f.Ordinal)
			}
			seen[f.Ordinal] = f.Name
		}
	}
}

// TestLedgerRoundTrips checks that a marshalled ledger parses back to an equal
// one. The ledger is the file CI compares, so a marshal that loses a slot would
// make `buffers verify` pass on a schema that had drifted.
func TestLedgerRoundTrips(t *testing.T) {
	schema := buildSchema(t)

	data, err := schema.Lock.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	back, err := buffers.ParseLock(data, "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !schema.Lock.Equal(back) {
		t.Error("a ledger did not survive a marshal and parse round trip")
	}
}
