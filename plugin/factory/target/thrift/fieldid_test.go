package thrift

import (
	"strings"
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
)

// TestFieldIDCeiling covers the one place a proto field number does not fit a
// Thrift field id.
//
// The boundary is the whole test: 32767 is the largest signed 16-bit value and
// must be accepted, 32768 is the first that cannot be and must be refused.
func TestFieldIDCeiling(t *testing.T) {
	cases := []struct {
		number int32
		want   bool
	}{
		{1, true},
		{32767, true},
		{32768, false},
		{536870911, false}, // the largest proto field number there is
	}
	for _, c := range cases {
		if got := fitsFieldID(c.number); got != c.want {
			t.Errorf("fitsFieldID(%d) = %v, want %v", c.number, got, c.want)
		}
	}
}

// TestOversizedFieldIsDiagnosed checks that an unrepresentable field is reported
// rather than emitted.
//
// Emitting it is the dangerous option and the one thrift itself takes: it warns,
// exits zero, and truncates the id into sixteen bits, which silently puts two
// fields in one slot.
func TestOversizedFieldIsDiagnosed(t *testing.T) {
	r := &run{}
	if r.checkFieldID("p.M.f", "field", 40000) {
		t.Fatal("a field number above the Thrift ceiling was accepted")
	}
	if len(r.diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(r.diags))
	}
	d := r.diags[0]
	if d.Rule != buffers.RuleTarget {
		t.Errorf("rule = %q, want %q", d.Rule, buffers.RuleTarget)
	}
	if !strings.Contains(d.Message, "40000") || !strings.Contains(d.Message, "32767") {
		t.Errorf("the message names neither the offending number nor the ceiling: %s", d.Message)
	}

	// A representable one must stay silent.
	r = &run{}
	if !r.checkFieldID("p.M.f", "field", 32767) {
		t.Error("32767 was refused")
	}
	if len(r.diags) != 0 {
		t.Errorf("a valid field number produced %d diagnostics", len(r.diags))
	}
}
