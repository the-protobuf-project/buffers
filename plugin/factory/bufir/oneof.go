package bufir

// oneof.go holds the two things that describe a message's slots rather than its
// data: the union grouping some of its fields, and the placeholder a removed
// field leaves behind.
//
// Both are views over Message.Fields rather than storage of their own — a oneof's
// arms appear in that list too — which is why they are separate from the field
// itself.

// Oneof is a proto oneof, which every target with a union renders as one.
type Oneof struct {
	// Node is the oneof's fully qualified proto name.
	Node NodeID
	// Name is the oneof's own name, as proto spells it.
	Name string
	// Doc is the leading comment, as prose.
	Doc string
	// Parent is the message declaring it.
	Parent *Message

	// UnionName is the generated FlatBuffers union type name. FlatBuffers unions
	// are named top-level types, unlike Cap'n Proto's, which are anonymous groups
	// inside the struct.
	UnionName string

	// Skip excludes the oneof and every arm in it.
	Skip bool

	// Fields are the arms, in ordinal order.
	Fields []*Field
}

// Slot is one occupied ordinal with no live field behind it: a field that was
// removed and reserved. It is emitted as a placeholder so that the fields after
// it keep the slots they already had.
//
// It carries no name, and cannot: proto records `reserved 6;` and
// `reserved "sample_period_ms";` as two independent lists with nothing tying a
// number to a name. Only the number is load-bearing here anyway.
type Slot struct {
	// Ordinal is the slot held open.
	Ordinal int32
	Number  int32 // the proto field number that was reserved
}
