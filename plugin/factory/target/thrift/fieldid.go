package thrift

// fieldid.go guards the one place a proto field number does *not* fit a Thrift
// field id, which is the exception to this target's central claim.
//
// The two numbering schemes agree in shape — both 1-based, both sparse, both
// permanent — but not in width. A Thrift field id is a signed 16-bit integer, so
// it stops at 32767; a proto field number runs to 536870911. Anything above the
// Thrift ceiling has no id to land on.
//
// What makes this worth a check rather than a note is how Thrift handles it. It
// does not refuse the schema:
//
//	[WARNING:big.thrift:3] Field key (32768) exceeds allowed range (-32768..32767).
//
// and then exits zero and generates code anyway, with the key truncated into
// sixteen bits. That is a silent wire corruption of exactly the kind this whole
// repository exists to prevent — two fields quietly sharing a slot — arriving as
// a warning most builds never show.
//
// So the field is dropped instead, loudly. A schema missing a field it names is
// recoverable and reported; a schema that writes two fields to one id is neither.

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"
)

// maxFieldID is the largest field id Thrift can hold.
const maxFieldID = 32767

// fitsFieldID reports whether a proto field number is representable as a Thrift
// field id, without recording anything.
func fitsFieldID(number int32) bool { return number >= 1 && number <= maxFieldID }

// checkFieldID reports an unrepresentable field number and returns whether the
// declaration may be emitted.
func (r *run) checkFieldID(node buffers.NodeID, kind string, number int32) bool {
	if fitsFieldID(number) {
		return true
	}
	r.collect(&buffers.Diagnostic{
		Rule: buffers.RuleTarget,
		Node: node,
		Message: fmt.Sprintf("proto field number %d is above the Thrift ceiling of %d — a Thrift "+
			"field id is a signed 16-bit integer — so this %s is omitted; thrift itself only warns "+
			"and then truncates the id into sixteen bits, which would put two fields in one slot",
			number, maxFieldID, kind),
		Hint: fmt.Sprintf("renumber the field below %d, or exclude it from this target with "+
			"(buffers.v1.field).targets so the omission is declared rather than diagnosed", maxFieldID),
	})
	return false
}
