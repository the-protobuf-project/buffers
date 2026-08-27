package capnp

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"
)

// scalar returns the Cap'n Proto spelling of a proto scalar kind, and whether the
// kind is one.
func scalar(k buffers.Kind) (string, bool) {
	switch k {
	case buffers.KindDouble:
		return "Float64", true
	case buffers.KindFloat:
		return "Float32", true
	case buffers.KindInt32, buffers.KindSint32, buffers.KindSfixed32:
		return "Int32", true
	case buffers.KindUint32, buffers.KindFixed32:
		return "UInt32", true
	case buffers.KindInt64, buffers.KindSint64, buffers.KindSfixed64:
		return "Int64", true
	case buffers.KindUint64, buffers.KindFixed64:
		return "UInt64", true
	case buffers.KindBool:
		return "Bool", true
	}
	return "", false
}

// A Cap'n Proto enum is always 16 bits, with no way to declare otherwise, so
// there is no width projection here to match the FlatBuffers target's. A schema
// that narrowed an enum to a byte gets 16 bits regardless; the enum renderer in
// capnp.go says so in a comment on the generated type rather than leaving someone
// to discover it by measuring.

// fieldType returns the Cap'n Proto type for a field.
func (r *run) fieldType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if f.Kind == buffers.KindMap {
		return "List(" + r.mapEntryName(f) + ")", nil
	}
	base, diag := r.baseType(f, from)
	if f.Repeated {
		return "List(" + base + ")", diag
	}
	return base, diag
}

// baseType returns the element type, ignoring repeated-ness.
func (r *run) baseType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if got, ok := scalar(f.Kind); ok {
		return got, nil
	}
	switch f.Kind {
	case buffers.KindString:
		return "Text", nil
	case buffers.KindBytes:
		return "Data", nil
	case buffers.KindEnum:
		return r.qualify(f.Enum, from), nil
	case buffers.KindMessage:
		return r.messageType(f, from)
	}
	return "", &buffers.Diagnostic{
		Rule:    buffers.RuleTarget,
		Node:    f.Node,
		Message: fmt.Sprintf("no Cap'n Proto type for proto kind %s", f.Kind),
	}
}
