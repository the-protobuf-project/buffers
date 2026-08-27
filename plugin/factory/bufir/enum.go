package bufir

// enum.go holds enums, their values, and the integer width a target that needs
// one projects them onto.
//
// The width lives here rather than in vocab.go with the other buffers.v1 mirrors
// because it carries behaviour — Bits, Signed and Range are what reject a value
// that does not fit — and that behaviour is only about enums.

// Enum is one proto enum.
type Enum struct {
	// Node is the enum's fully qualified proto name.
	Node NodeID
	// Name is the enum's own name, unqualified.
	Name string
	// Package is the proto package declaring it.
	Package string
	// File is the file declaring it.
	File *File
	// Doc is the leading comment, as prose.
	Doc string

	// Underlying is the integer width a target that needs one should use.
	Underlying IntWidth

	// BitFlags renders the enum as a FlatBuffers bitmask. See buffers.v1's
	// EnumOptions.bit_flags for what it does to the AIP-126 zero value.
	BitFlags bool

	// Skip excludes the enum from every target.
	Skip bool

	// Values are the enumerants, in declaration order.
	Values []*EnumValue
}

// EnumValue is one enumerant.
type EnumValue struct {
	// Node is the value's fully qualified proto name.
	Node NodeID
	// Name is the value's own name, as proto spells it.
	Name   string
	Number int32 // the proto value, which may be sparse and need not start at 0

	// Ordinal is the 0-based contiguous position Cap'n Proto requires.
	Ordinal int32
	// OrdinalSource records where Ordinal came from.
	OrdinalSource OrdinalSource

	// Doc is the leading comment, as prose.
	Doc string
	// Skip excludes the value, which still consumes its ordinal.
	Skip bool
}

// IntWidth mirrors buffers.v1.IntWidth.
type IntWidth uint8

const (
	// IntWidthUnspecified means the default, which is proto's own signed int32.
	IntWidthUnspecified IntWidth = iota
	// IntWidthInt8 is a signed byte.
	IntWidthInt8
	// IntWidthUint8 is an unsigned byte, the usual choice for a small closed enum
	// in a high-rate message.
	IntWidthUint8
	// IntWidthInt16 is a signed 16-bit integer.
	IntWidthInt16
	// IntWidthUint16 is an unsigned 16-bit integer.
	IntWidthUint16
	// IntWidthInt32 is a signed 32-bit integer, matching proto's own encoding.
	IntWidthInt32
	// IntWidthUint32 is an unsigned 32-bit integer.
	IntWidthUint32
	// IntWidthInt64 is a signed 64-bit integer.
	IntWidthInt64
	// IntWidthUint64 is an unsigned 64-bit integer. Legal, and almost never what
	// an enum needs.
	IntWidthUint64
)

// Bits returns the width in bits, and Signed whether it carries a sign.
func (w IntWidth) Bits() int {
	switch w {
	case IntWidthInt8, IntWidthUint8:
		return 8
	case IntWidthInt16, IntWidthUint16:
		return 16
	case IntWidthInt32, IntWidthUint32:
		return 32
	case IntWidthInt64, IntWidthUint64:
		return 64
	}
	return 32 // IntWidthUnspecified defaults to proto's own int32
}

// Signed reports whether the width carries a sign.
func (w IntWidth) Signed() bool {
	switch w {
	case IntWidthInt8, IntWidthInt16, IntWidthInt32, IntWidthInt64:
		return true
	case IntWidthUnspecified:
		// The default is proto's own encoding, which is a signed int32.
		return true
	}
	return false
}

// Range returns the inclusive value range the width can hold, which is what
// rejects an enum whose values do not fit rather than truncating them.
func (w IntWidth) Range() (lo, hi int64) {
	bits := w.Bits()
	if w.Signed() {
		hi = int64(1)<<(bits-1) - 1
		return -hi - 1, hi
	}
	if bits == 64 {
		// A proto enum value is an int32, so an unsigned 64-bit width can hold
		// every value it could possibly be asked to; returning the int64 max
		// avoids overflowing the return type for a bound nothing can exceed.
		return 0, 1<<63 - 1
	}
	return 0, int64(1)<<bits - 1
}
