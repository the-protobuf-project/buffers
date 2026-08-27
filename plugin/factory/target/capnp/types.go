package capnp

import (
	"fmt"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
)

// scalar returns the Cap'n Proto spelling of a proto scalar kind, and whether the
// kind is one.
func scalar(k bufir.Kind) (string, bool) {
	switch k {
	case bufir.KindDouble:
		return "Float64", true
	case bufir.KindFloat:
		return "Float32", true
	case bufir.KindInt32, bufir.KindSint32, bufir.KindSfixed32:
		return "Int32", true
	case bufir.KindUint32, bufir.KindFixed32:
		return "UInt32", true
	case bufir.KindInt64, bufir.KindSint64, bufir.KindSfixed64:
		return "Int64", true
	case bufir.KindUint64, bufir.KindFixed64:
		return "UInt64", true
	case bufir.KindBool:
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
func (r *run) fieldType(f *bufir.Field, from *bufir.File) (string, *bufir.Diagnostic) {
	if f.Kind == bufir.KindMap {
		return "List(" + r.mapEntryName(f) + ")", nil
	}
	base, diag := r.baseType(f, from)
	if f.Repeated {
		return "List(" + base + ")", diag
	}
	return base, diag
}

// baseType returns the element type, ignoring repeated-ness.
func (r *run) baseType(f *bufir.Field, from *bufir.File) (string, *bufir.Diagnostic) {
	if got, ok := scalar(f.Kind); ok {
		return got, nil
	}
	switch f.Kind {
	case bufir.KindString:
		return "Text", nil
	case bufir.KindBytes:
		return "Data", nil
	case bufir.KindEnum:
		return r.qualify(f.Enum, from), nil
	case bufir.KindMessage:
		return r.messageType(f, from)
	}
	return "", &bufir.Diagnostic{
		Rule:    bufir.RuleTarget,
		Node:    f.Node,
		Message: fmt.Sprintf("no Cap'n Proto type for proto kind %s", f.Kind),
	}
}
