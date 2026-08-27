package ros

import (
	"fmt"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
)

// scalar returns the ROS spelling of a proto scalar kind, and whether the kind
// is one.
func scalar(k bufir.Kind) (string, bool) {
	switch k {
	case bufir.KindDouble:
		return "float64", true
	case bufir.KindFloat:
		return "float32", true
	case bufir.KindInt32, bufir.KindSint32, bufir.KindSfixed32:
		return "int32", true
	case bufir.KindUint32, bufir.KindFixed32:
		return "uint32", true
	case bufir.KindInt64, bufir.KindSint64, bufir.KindSfixed64:
		return "int64", true
	case bufir.KindUint64, bufir.KindFixed64:
		return "uint64", true
	case bufir.KindBool:
		return "bool", true
	}
	return "", false
}

// width returns the ROS integer type holding an enum's declared underlying
// width. ROS constants are typed, and this is the type the constant block uses.
func width(w bufir.IntWidth) string {
	switch w {
	case bufir.IntWidthInt8:
		return "int8"
	case bufir.IntWidthUint8:
		return "uint8"
	case bufir.IntWidthInt16:
		return "int16"
	case bufir.IntWidthUint16:
		return "uint16"
	case bufir.IntWidthUint32:
		return "uint32"
	case bufir.IntWidthInt64:
		return "int64"
	case bufir.IntWidthUint64:
		return "uint64"
	}
	return "int32"
}

// fieldType returns the ROS type for a field, including any array or length
// bound.
func (r *run) fieldType(f *bufir.Field, from *bufir.File) (string, *bufir.Diagnostic) {
	if f.Kind == bufir.KindMap {
		// ROS has no map. A list of two-field messages is the only shape
		// available, and unlike FlatBuffers there is no keyed-lookup convention
		// to preserve alongside the data.
		return r.mapEntryName(f) + "[]", nil
	}

	base, diag := r.baseType(f, from)

	// A bound belongs to the type in ROS, so it is applied here rather than as a
	// trailing annotation.
	if f.Kind == bufir.KindString && !f.Repeated && f.MaxLen > 0 {
		base = fmt.Sprintf("string<=%d", f.MaxLen)
	}
	if !f.Repeated {
		return base, diag
	}

	switch {
	case f.FixedLen > 0:
		return fmt.Sprintf("%s[%d]", base, f.FixedLen), diag
	case f.MaxLen > 0:
		return fmt.Sprintf("%s[<=%d]", base, f.MaxLen), diag
	}
	return base + "[]", diag
}

// baseType returns the element type, ignoring arrays and bounds.
func (r *run) baseType(f *bufir.Field, from *bufir.File) (string, *bufir.Diagnostic) {
	if got, ok := scalar(f.Kind); ok {
		return got, nil
	}
	switch f.Kind {
	case bufir.KindString:
		return "string", nil
	case bufir.KindBytes:
		// ROS has no bytes type. uint8[] is the conventional stand-in and is what
		// sensor_msgs/Image uses for pixel data.
		return "uint8[]", nil
	case bufir.KindEnum:
		return r.qualify(r.enumRosName(f.Enum), from), nil
	case bufir.KindMessage:
		return r.messageType(f, from)
	}
	return "", &bufir.Diagnostic{
		Rule:    bufir.RuleTarget,
		Node:    f.Node,
		Message: fmt.Sprintf("no ROS type for proto kind %s", f.Kind),
	}
}
