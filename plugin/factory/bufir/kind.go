package bufir

import "google.golang.org/protobuf/reflect/protoreflect"

// Kind is a proto field's type at proto granularity.
//
// It is spelled the same way as protokit's service.Kind, on purpose: a reader who
// knows one should not have to learn the other. It is a separate type rather than
// an alias because this package's graph is built without service.Build — see
// doc.go — and depending on that package for a constant would import a route
// table into a plugin that emits none.
//
// The granularity is the point. A database backend is right to fold the four
// 64-bit widths into one integer type; a serialization schema is not. Cap'n Proto
// spells Int64 and UInt64 differently and a schema that picks the wrong one
// reinterprets every negative value without a word of complaint.
type Kind uint8

const (
	// KindDouble is proto's `double`: 64-bit IEEE 754.
	KindDouble Kind = iota
	// KindFloat is proto's `float`: 32-bit IEEE 754.
	KindFloat
	// KindInt32 is a signed 32-bit varint. Negative values cost ten bytes,
	// which is what sint32 exists to avoid.
	KindInt32
	// KindInt64 is a signed 64-bit varint, with the same negative-value cost.
	KindInt64
	// KindUint32 is an unsigned 32-bit varint.
	KindUint32
	// KindUint64 is an unsigned 64-bit varint. Folding it in with int64 is the
	// mistake that silently reinterprets every value above 2^63.
	KindUint64
	// KindSint32 is a signed 32-bit zigzag varint, which encodes small negative
	// values compactly.
	KindSint32
	// KindSint64 is the 64-bit zigzag equivalent.
	KindSint64
	// KindFixed32 is an unsigned 32-bit value in a fixed four bytes.
	KindFixed32
	// KindFixed64 is an unsigned 64-bit value in a fixed eight bytes.
	KindFixed64
	// KindSfixed32 is the signed counterpart of KindFixed32.
	KindSfixed32
	// KindSfixed64 is the signed counterpart of KindFixed64.
	KindSfixed64
	// KindBool is a single-bit value, encoded as a varint.
	KindBool
	// KindString is UTF-8 text with a length prefix.
	KindString
	// KindBytes is an opaque length-prefixed blob.
	KindBytes
	// KindEnum is a proto enum, which is varint-encoded as an int32 whatever
	// width a target later projects it onto.
	KindEnum
	// KindMessage is a nested message. Field.Message carries which one.
	KindMessage
	// KindMap is a proto map. protoreflect spells it as a repeated message over a
	// synthetic entry type; the build unfolds that into MapKey and MapValue,
	// because no target here has a map and all of them must rewrite it.
	KindMap
)

// classifyKind maps a descriptor's type onto a Kind. Map fields are detected by
// the caller, which has the repeated-ness and the entry message to look at;
// protoreflect spells a map as a repeated message field and this function cannot
// tell the difference on its own.
func classifyKind(fd protoreflect.FieldDescriptor) Kind {
	switch fd.Kind() {
	case protoreflect.DoubleKind:
		return KindDouble
	case protoreflect.FloatKind:
		return KindFloat
	case protoreflect.Int32Kind:
		return KindInt32
	case protoreflect.Int64Kind:
		return KindInt64
	case protoreflect.Uint32Kind:
		return KindUint32
	case protoreflect.Uint64Kind:
		return KindUint64
	case protoreflect.Sint32Kind:
		return KindSint32
	case protoreflect.Sint64Kind:
		return KindSint64
	case protoreflect.Fixed32Kind:
		return KindFixed32
	case protoreflect.Fixed64Kind:
		return KindFixed64
	case protoreflect.Sfixed32Kind:
		return KindSfixed32
	case protoreflect.Sfixed64Kind:
		return KindSfixed64
	case protoreflect.BoolKind:
		return KindBool
	case protoreflect.StringKind:
		return KindString
	case protoreflect.BytesKind:
		return KindBytes
	case protoreflect.EnumKind:
		return KindEnum
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return KindMessage
	}
	return KindMessage
}

// String returns the proto spelling of the kind, so a diagnostic naming a field's
// type reads as the author wrote it rather than as an enum number.
func (k Kind) String() string {
	switch k {
	case KindDouble:
		return "double"
	case KindFloat:
		return "float"
	case KindInt32:
		return "int32"
	case KindInt64:
		return "int64"
	case KindUint32:
		return "uint32"
	case KindUint64:
		return "uint64"
	case KindSint32:
		return "sint32"
	case KindSint64:
		return "sint64"
	case KindFixed32:
		return "fixed32"
	case KindFixed64:
		return "fixed64"
	case KindSfixed32:
		return "sfixed32"
	case KindSfixed64:
		return "sfixed64"
	case KindBool:
		return "bool"
	case KindString:
		return "string"
	case KindBytes:
		return "bytes"
	case KindEnum:
		return "enum"
	case KindMessage:
		return "message"
	case KindMap:
		return "map"
	}
	return "unknown"
}
