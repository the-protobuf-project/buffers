package bufir

// kind_classify.go answers the questions a target asks about a proto type before
// projecting it: is this fixed-width, is it a number, how wide, and is it signed.
//
// They are predicates rather than a type table because each target has its own
// type names and none of them agree — what they share is needing to know whether
// a field can sit inline in a packed record, or whether picking the wrong
// signedness would reinterpret its values.

// Scalar reports whether the kind is a fixed-width value with no indirection —
// no length prefix, no pointer, no vtable.
//
// It is the eligibility test for a packed layout: a FlatBuffers struct and a
// Cap'n Proto data-section field may hold exactly these, plus enums and other
// packed records. Note that String and Bytes are excluded even though a database
// would call them scalars, because both are variable-length and neither can sit
// inline.
func (k Kind) Scalar() bool {
	switch k {
	case KindDouble, KindFloat, KindInt32, KindInt64, KindUint32, KindUint64,
		KindSint32, KindSint64, KindFixed32, KindFixed64, KindSfixed32,
		KindSfixed64, KindBool:
		return true
	}
	return false
}

// Numeric reports whether the kind is a number in proto terms, regardless of how
// any particular target spells it.
func (k Kind) Numeric() bool {
	switch k {
	case KindDouble, KindFloat:
		return true
	}
	return k.Integer()
}

// Integer reports whether the kind is an integer of some width and signedness.
func (k Kind) Integer() bool {
	switch k {
	case KindInt32, KindInt64, KindUint32, KindUint64, KindSint32, KindSint64,
		KindFixed32, KindFixed64, KindSfixed32, KindSfixed64:
		return true
	}
	return false
}

// Signed reports whether an integer kind carries a sign. Meaningless, and false,
// for a kind that is not an integer.
func (k Kind) Signed() bool {
	switch k {
	case KindInt32, KindInt64, KindSint32, KindSint64, KindSfixed32, KindSfixed64:
		return true
	}
	return false
}

// Bits returns the width of a numeric kind in bits, or 0 for a kind that has no
// fixed width.
func (k Kind) Bits() int {
	switch k {
	case KindFloat, KindInt32, KindUint32, KindSint32, KindFixed32, KindSfixed32:
		return 32
	case KindDouble, KindInt64, KindUint64, KindSint64, KindFixed64, KindSfixed64:
		return 64
	case KindBool:
		return 8
	}
	return 0
}
