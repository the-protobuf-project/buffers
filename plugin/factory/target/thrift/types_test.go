package thrift

import (
	"strings"
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
)

// TestScalarKeepsWidthAndSign is the check that matters most here. Thrift has no
// unsigned types, and folding uint64 onto i64 without saying so is exactly the
// silent reinterpretation this repository exists to prevent elsewhere.
func TestScalarKeepsWidthAndSign(t *testing.T) {
	cases := map[buffers.Kind]string{
		buffers.KindDouble:   "double",
		buffers.KindFloat:    "double",
		buffers.KindInt32:    "i32",
		buffers.KindSint32:   "i32",
		buffers.KindSfixed32: "i32",
		buffers.KindInt64:    "i64",
		buffers.KindSint64:   "i64",
		buffers.KindSfixed64: "i64",
		buffers.KindUint32:   "i32",
		buffers.KindFixed32:  "i32",
		buffers.KindUint64:   "i64",
		buffers.KindFixed64:  "i64",
		buffers.KindBool:     "bool",
	}
	for kind, want := range cases {
		got, _, ok := scalar(kind)
		if !ok {
			t.Errorf("scalar(%s): not reported as a scalar", kind)
			continue
		}
		if got != want {
			t.Errorf("scalar(%s) = %q, want %q", kind, got, want)
		}
	}
}

// TestLossyKindsCarryANote checks that every projection that changes what a value
// means says so in the generated doc, and that the lossless ones stay quiet.
func TestLossyKindsCarryANote(t *testing.T) {
	lossy := []buffers.Kind{
		buffers.KindFloat, buffers.KindUint32, buffers.KindFixed32,
		buffers.KindUint64, buffers.KindFixed64,
	}
	for _, kind := range lossy {
		if _, note, _ := scalar(kind); note == "" {
			t.Errorf("scalar(%s): projection loses something and carries no note", kind)
		}
	}
	for _, kind := range []buffers.Kind{buffers.KindInt32, buffers.KindInt64, buffers.KindBool, buffers.KindDouble} {
		if _, note, _ := scalar(kind); note != "" {
			t.Errorf("scalar(%s): exact projection carries note %q", kind, note)
		}
	}
}

// TestUnsignedIsDiagnosed checks that a sign reinterpretation is a diagnostic and
// not only a comment, since it is silent on both sides of the wire.
func TestUnsignedIsDiagnosed(t *testing.T) {
	r := &run{}
	for _, kind := range []buffers.Kind{buffers.KindUint32, buffers.KindUint64, buffers.KindFixed32, buffers.KindFixed64} {
		d := r.unsignedDiag(&buffers.Field{Kind: kind, Node: "pkg.M.f"})
		if d == nil {
			t.Errorf("%s: no diagnostic", kind)
			continue
		}
		if d.Rule != buffers.RuleTarget {
			t.Errorf("%s: rule = %q, want %q", kind, d.Rule, buffers.RuleTarget)
		}
	}
	if d := r.unsignedDiag(&buffers.Field{Kind: buffers.KindInt64}); d != nil {
		t.Errorf("int64 diagnosed as unsigned: %v", d)
	}
}

// TestModifierFollowsProto3Presence checks the requiredness mapping, including
// the one thing this target never emits.
func TestModifierFollowsProto3Presence(t *testing.T) {
	cases := []struct {
		name  string
		field buffers.Field
		want  string
	}{
		{"bare scalar", buffers.Field{Kind: buffers.KindInt32}, ""},
		{"explicit optional", buffers.Field{Kind: buffers.KindInt32, Optional: true}, " optional"},
		{"message", buffers.Field{Kind: buffers.KindMessage}, " optional"},
		{"repeated message", buffers.Field{Kind: buffers.KindMessage, Repeated: true}, ""},
		{"map", buffers.Field{Kind: buffers.KindMap}, ""},
		{"oneof arm", buffers.Field{Kind: buffers.KindString, Oneof: &buffers.Oneof{}}, " optional"},
	}
	for _, c := range cases {
		if got := modifier(&c.field); got != c.want {
			t.Errorf("%s: modifier = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestRequiredIsNeverEmitted guards the decision that Thrift's `required` is a
// permanent wire contract and AIP-203 REQUIRED is not, so one must never be
// rendered as the other.
func TestRequiredIsNeverEmitted(t *testing.T) {
	f := &buffers.Field{Kind: buffers.KindString, Behavior: []buffers.Behavior{buffers.BehaviorRequired}}
	if got := modifier(f); strings.Contains(got, "required") {
		t.Fatalf("modifier = %q; AIP REQUIRED must not become Thrift required", got)
	}

	r := &run{}
	notes := r.notes(f)
	if len(notes) == 0 || !strings.Contains(strings.Join(notes, " "), "AIP-203") {
		t.Errorf("a REQUIRED field carries no note explaining the omission: %v", notes)
	}
}

// TestWrapperCarriesTheWrappedKindsCaveats is the case that reads as a message
// and projects as a scalar.
//
// google.protobuf.UInt32Value is KindMessage in the IR and unwraps to a bare i32,
// so anything keyed on Field.Kind misses it — and a boxed uint32 would land on a
// signed i32 with no note and no diagnostic while a bare one got both. The box is
// presence, not a change of type.
func TestWrapperCarriesTheWrappedKindsCaveats(t *testing.T) {
	r := &run{}
	boxed := &buffers.Field{
		Node:      "p.M.f",
		Kind:      buffers.KindMessage,
		Message:   "google.protobuf.UInt32Value",
		WellKnown: buffers.WKUint32Value,
	}

	if got := effectiveKind(boxed); got != buffers.KindUint32 {
		t.Fatalf("effectiveKind = %s, want uint32", got)
	}
	if d := r.unsignedDiag(boxed); d == nil {
		t.Error("a boxed uint32 produced no sign-reinterpretation diagnostic")
	}
	notes := r.notes(boxed)
	if len(notes) == 0 || !strings.Contains(strings.Join(notes, " "), "unsigned") {
		t.Errorf("a boxed uint32 carries no caveat in its doc: %v", notes)
	}

	// A boxed float keeps the widening note, and is not diagnosed — widening is
	// lossless in value.
	f := &buffers.Field{Node: "p.M.g", Kind: buffers.KindMessage, WellKnown: buffers.WKFloatValue}
	if d := r.unsignedDiag(f); d != nil {
		t.Errorf("a boxed float was diagnosed as unsigned: %v", d)
	}
	if notes := r.notes(f); len(notes) == 0 || !strings.Contains(notes[0], "32-bit floating point") {
		t.Errorf("a boxed float lost its widening note: %v", notes)
	}
}

// TestUnsignedNoteNamesTheActualKind guards a note that used to say "uint32" for
// a fixed32 field, which is a different proto type with the same projection.
func TestUnsignedNoteNamesTheActualKind(t *testing.T) {
	for kind, want := range map[buffers.Kind]string{
		buffers.KindUint32:  "uint32",
		buffers.KindFixed32: "fixed32",
		buffers.KindUint64:  "uint64",
		buffers.KindFixed64: "fixed64",
	} {
		_, note, _ := scalar(kind)
		if !strings.Contains(note, "`"+want+"`") {
			t.Errorf("scalar(%s) note does not name %s: %s", kind, want, note)
		}
	}
}
