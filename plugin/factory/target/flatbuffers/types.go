package flatbuffers

import (
	"fmt"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
)

// scalar returns the FlatBuffers spelling of a proto scalar kind, and whether the
// kind is one.
func scalar(k bufir.Kind) (string, bool) {
	switch k {
	case bufir.KindDouble:
		return "double", true
	case bufir.KindFloat:
		return "float", true
	case bufir.KindInt32, bufir.KindSint32, bufir.KindSfixed32:
		return "int", true
	case bufir.KindUint32, bufir.KindFixed32:
		return "uint", true
	case bufir.KindInt64, bufir.KindSint64, bufir.KindSfixed64:
		return "long", true
	case bufir.KindUint64, bufir.KindFixed64:
		return "ulong", true
	case bufir.KindBool:
		return "bool", true
	}
	return "", false
}

// width returns the FlatBuffers spelling of an enum's underlying integer type.
func width(w bufir.IntWidth) string {
	switch w {
	case bufir.IntWidthInt8:
		return "byte"
	case bufir.IntWidthUint8:
		return "ubyte"
	case bufir.IntWidthInt16:
		return "short"
	case bufir.IntWidthUint16:
		return "ushort"
	case bufir.IntWidthUint32:
		return "uint"
	case bufir.IntWidthInt64:
		return "long"
	case bufir.IntWidthUint64:
		return "ulong"
	}
	return "int" // IntWidthInt32 and unspecified, matching proto's own encoding
}

// fieldType returns the FlatBuffers type for a field as it appears inside a
// table or struct, and any diagnostic the projection produced.
//
// It does not handle oneof arms: a oneof becomes one union field covering every
// arm, so the arms are never rendered as fields of their own. See plan.go.
func (t *run) fieldType(f *bufir.Field, from *bufir.File) (string, *bufir.Diagnostic) {
	if f.Kind == bufir.KindMap {
		// FlatBuffers has no map. The conventional replacement is a vector of
		// two-field tables with the key marked `(key)`, which flatc gives a
		// binary-search lookup over — so the substitution keeps the lookup, not
		// just the data.
		return "[" + t.mapEntryName(f) + "]", nil
	}

	base, diag := t.baseType(f, from)
	if f.Repeated {
		return "[" + base + "]", diag
	}
	return base, diag
}

// baseType returns the element type, ignoring repeated-ness.
func (t *run) baseType(f *bufir.Field, from *bufir.File) (string, *bufir.Diagnostic) {
	if got, ok := scalar(f.Kind); ok {
		return got, nil
	}
	switch f.Kind {
	case bufir.KindString:
		return "string", nil
	case bufir.KindBytes:
		// FlatBuffers has no bytes type; a vector of ubyte is the idiom, and
		// flatc generates a direct pointer accessor for it.
		return "[ubyte]", nil
	case bufir.KindEnum:
		return t.qualify(f.Enum, from), nil
	case bufir.KindMessage:
		return t.messageType(f, from)
	}
	return "", &bufir.Diagnostic{
		Rule:    bufir.RuleTarget,
		Node:    f.Node,
		Message: fmt.Sprintf("no FlatBuffers type for proto kind %s", f.Kind),
	}
}
