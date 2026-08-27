package ros

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"
)

// scalar returns the ROS spelling of a proto scalar kind, and whether the kind
// is one.
func scalar(k buffers.Kind) (string, bool) {
	switch k {
	case buffers.KindDouble:
		return "float64", true
	case buffers.KindFloat:
		return "float32", true
	case buffers.KindInt32, buffers.KindSint32, buffers.KindSfixed32:
		return "int32", true
	case buffers.KindUint32, buffers.KindFixed32:
		return "uint32", true
	case buffers.KindInt64, buffers.KindSint64, buffers.KindSfixed64:
		return "int64", true
	case buffers.KindUint64, buffers.KindFixed64:
		return "uint64", true
	case buffers.KindBool:
		return "bool", true
	}
	return "", false
}

// width returns the ROS integer type holding an enum's declared underlying
// width. ROS constants are typed, and this is the type the constant block uses.
func width(w buffers.IntWidth) string {
	switch w {
	case buffers.IntWidthInt8:
		return "int8"
	case buffers.IntWidthUint8:
		return "uint8"
	case buffers.IntWidthInt16:
		return "int16"
	case buffers.IntWidthUint16:
		return "uint16"
	case buffers.IntWidthUint32:
		return "uint32"
	case buffers.IntWidthInt64:
		return "int64"
	case buffers.IntWidthUint64:
		return "uint64"
	}
	return "int32"
}

// fieldType returns the ROS type for a field, including any array or length
// bound.
func (r *run) fieldType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if f.Kind == buffers.KindMap {
		// ROS has no map. A list of two-field messages is the only shape
		// available, and unlike FlatBuffers there is no keyed-lookup convention
		// to preserve alongside the data.
		return r.mapEntryName(f) + "[]", nil
	}

	base, diag := r.baseType(f, from)

	// A bound belongs to the type in ROS, so it is applied here rather than as a
	// trailing annotation.
	if f.Kind == buffers.KindString && !f.Repeated && f.MaxLen > 0 {
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
func (r *run) baseType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if got, ok := scalar(f.Kind); ok {
		return got, nil
	}
	switch f.Kind {
	case buffers.KindString:
		return "string", nil
	case buffers.KindBytes:
		// ROS has no bytes type. uint8[] is the conventional stand-in and is what
		// sensor_msgs/Image uses for pixel data.
		return "uint8[]", nil
	case buffers.KindEnum:
		return r.qualify(r.enumRosName(f.Enum), from), nil
	case buffers.KindMessage:
		return r.messageType(f, from)
	}
	return "", &buffers.Diagnostic{
		Rule:    buffers.RuleTarget,
		Node:    f.Node,
		Message: fmt.Sprintf("no ROS type for proto kind %s", f.Kind),
	}
}
