package flatbuffers

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"
)

// scalar returns the FlatBuffers spelling of a proto scalar kind, and whether the
// kind is one.
func scalar(k buffers.Kind) (string, bool) {
	switch k {
	case buffers.KindDouble:
		return "double", true
	case buffers.KindFloat:
		return "float", true
	case buffers.KindInt32, buffers.KindSint32, buffers.KindSfixed32:
		return "int", true
	case buffers.KindUint32, buffers.KindFixed32:
		return "uint", true
	case buffers.KindInt64, buffers.KindSint64, buffers.KindSfixed64:
		return "long", true
	case buffers.KindUint64, buffers.KindFixed64:
		return "ulong", true
	case buffers.KindBool:
		return "bool", true
	}
	return "", false
}

// width returns the FlatBuffers spelling of an enum's underlying integer type.
func width(w buffers.IntWidth) string {
	switch w {
	case buffers.IntWidthInt8:
		return "byte"
	case buffers.IntWidthUint8:
		return "ubyte"
	case buffers.IntWidthInt16:
		return "short"
	case buffers.IntWidthUint16:
		return "ushort"
	case buffers.IntWidthUint32:
		return "uint"
	case buffers.IntWidthInt64:
		return "long"
	case buffers.IntWidthUint64:
		return "ulong"
	}
	return "int" // IntWidthInt32 and unspecified, matching proto's own encoding
}

// fieldType returns the FlatBuffers type for a field as it appears inside a
// table or struct, and any diagnostic the projection produced.
//
// It does not handle oneof arms: a oneof becomes one union field covering every
// arm, so the arms are never rendered as fields of their own. See plan.go.
func (t *run) fieldType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if f.Kind == buffers.KindMap {
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
func (t *run) baseType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if got, ok := scalar(f.Kind); ok {
		return got, nil
	}
	switch f.Kind {
	case buffers.KindString:
		return "string", nil
	case buffers.KindBytes:
		// FlatBuffers has no bytes type; a vector of ubyte is the idiom, and
		// flatc generates a direct pointer accessor for it.
		return "[ubyte]", nil
	case buffers.KindEnum:
		return t.qualify(f.Enum, from), nil
	case buffers.KindMessage:
		return t.messageType(f, from)
	}
	return "", &buffers.Diagnostic{
		Rule:    buffers.RuleTarget,
		Node:    f.Node,
		Message: fmt.Sprintf("no FlatBuffers type for proto kind %s", f.Kind),
	}
}
