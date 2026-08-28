package thrift

// types.go projects a proto type onto a Thrift one.
//
// Thrift's type system covers proto's almost exactly — same integer widths, a
// real map, a real list — with two gaps, and both are about *interpretation*
// rather than about bits.
//
// Thrift has no unsigned integers. A uint32 still round-trips its 32 bits, but a
// value above 2147483647 reads back as a negative i32, and nothing on either side
// reports it. Thrift also has no 32-bit floating point type, so a proto float
// widens to a double: lossless in value, and four bytes larger in every message.

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"
)

// scalar returns the Thrift spelling of a proto scalar kind, the note the
// projection deserves in the generated doc comment, and whether the kind is a
// scalar at all.
func scalar(k buffers.Kind) (spelling, note string, ok bool) {
	switch k {
	case buffers.KindDouble:
		return "double", "", true
	case buffers.KindFloat:
		return "double", "proto `float`. Thrift has no 32-bit floating point type, so this widens to " +
			"a double: the value survives, the message grows four bytes.", true
	case buffers.KindInt32, buffers.KindSint32, buffers.KindSfixed32:
		return "i32", "", true
	case buffers.KindInt64, buffers.KindSint64, buffers.KindSfixed64:
		return "i64", "", true
	case buffers.KindUint32, buffers.KindFixed32:
		return "i32", fmt.Sprintf("proto `%s`. Thrift has no unsigned types; the 32 bits round-trip, "+
			"but a value above 2147483647 reads back negative.", k), true
	case buffers.KindUint64, buffers.KindFixed64:
		return "i64", fmt.Sprintf("proto `%s`. Thrift has no unsigned types; the 64 bits round-trip, "+
			"but a value above 9223372036854775807 reads back negative.", k), true
	case buffers.KindBool:
		return "bool", "", true
	}
	return "", "", false
}

// effectiveKind is the proto kind a field is actually projected as.
//
// It differs from Field.Kind for exactly one input, and that input is why this
// exists: a google.protobuf.UInt32Value is a *message* as far as the IR is
// concerned, and qualify.go unwraps it to a bare i32. Keyed on Field.Kind, every
// caveat below would then miss it — a boxed uint32 would land on i32 with no
// note and no diagnostic, while a bare one got both. The box is presence, not a
// change of type.
func effectiveKind(f *buffers.Field) buffers.Kind {
	if wrapped, ok := f.WellKnown.Wrapper(); ok {
		return wrapped
	}
	return f.Kind
}

// unsigned reports whether a kind loses its sign interpretation in Thrift, which
// is the one projection here worth a diagnostic rather than only a comment.
func unsigned(k buffers.Kind) bool {
	switch k {
	case buffers.KindUint32, buffers.KindFixed32, buffers.KindUint64, buffers.KindFixed64:
		return true
	}
	return false
}

// fieldType returns the Thrift type for a field, including its container.
func (r *run) fieldType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if f.Kind == buffers.KindMap {
		// The one construct Thrift holds better than any other target here. A
		// proto map key is always an integral, bool or string type, all of which
		// Thrift accepts as a map key, so nothing is given up.
		key, diag := r.baseType(f.MapKey, from)
		if diag != nil {
			return "", diag
		}
		val, diag := r.baseType(f.MapValue, from)
		return fmt.Sprintf("map<%s, %s>", key, val), diag
	}
	base, diag := r.baseType(f, from)
	if f.Repeated {
		return "list<" + base + ">", diag
	}
	return base, diag
}

// baseType returns the element type, ignoring repeated-ness.
func (r *run) baseType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if got, _, ok := scalar(f.Kind); ok {
		return got, r.unsignedDiag(f)
	}
	switch f.Kind {
	case buffers.KindString:
		return "string", nil
	case buffers.KindBytes:
		return "binary", nil
	case buffers.KindEnum:
		return r.qualify(f.Enum, from), nil
	case buffers.KindMessage:
		return r.messageType(f, from)
	}
	return "", &buffers.Diagnostic{
		Rule:    buffers.RuleTarget,
		Node:    f.Node,
		Message: fmt.Sprintf("no Thrift type for proto kind %s", f.Kind),
	}
}

// unsignedDiag reports a sign reinterpretation, or nil when there is none.
//
// It is a diagnostic and not only a comment because the failure is silent on both
// sides: the producer writes a number Thrift will hand back as a different one,
// and no generated accessor anywhere in the chain says so.
func (r *run) unsignedDiag(f *buffers.Field) *buffers.Diagnostic {
	kind := effectiveKind(f)
	if !unsigned(kind) {
		return nil
	}
	return &buffers.Diagnostic{
		Rule: buffers.RuleTarget,
		Node: f.Node,
		Message: fmt.Sprintf("%s has no Thrift equivalent; it is emitted as %s, which reinterprets "+
			"every value above the signed maximum as negative", kind, signedOf(kind)),
		Hint: "use the signed proto type if the values fit it, so the reinterpretation is at least " +
			"declared on both sides",
	}
}

// signedOf names the Thrift type an unsigned proto kind lands on, for the
// diagnostic.
func signedOf(k buffers.Kind) string {
	switch k {
	case buffers.KindUint32, buffers.KindFixed32:
		return "i32"
	}
	return "i64"
}

// notes returns the doc-comment lines a field's projection deserves: the sign or
// width caveat, and the AIP behavior Thrift declines to enforce.
func (r *run) notes(f *buffers.Field) []string {
	var out []string
	if _, note, ok := scalar(effectiveKind(f)); ok && note != "" {
		out = append(out, note)
	}
	if f.Required() {
		// Thrift's `required` is a permanent wire contract: a reader rejects a
		// message without the field, forever, so a field marked required can never
		// be relaxed without breaking every deployed consumer. AIP-203 REQUIRED is
		// an API-layer contract that services routinely relax, and rendering one as
		// the other would freeze a decision the proto did not make.
		out = append(out, "REQUIRED (AIP-203). Not emitted as Thrift `required`, which can never "+
			"later be relaxed; enforcement stays with the service.")
	}
	return out
}
