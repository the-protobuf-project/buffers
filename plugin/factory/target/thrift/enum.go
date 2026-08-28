package thrift

// enum.go renders a proto enum.
//
// This is the cleanest correspondence in the whole target and needs no policy at
// all: a Thrift enum is a named set of i32 constants, which is exactly what a
// proto enum is, down to the width and the sparseness. The proto value is emitted
// verbatim rather than the derived ordinal, for the same reason a field id is —
// the numbering schemes already agree.

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// enum renders one enum and its values.
func (r *run) enum(b *emit.Buf, e *buffers.Enum) {
	var notes []string
	if e.Underlying.Bits() != 32 {
		notes = append(notes, fmt.Sprintf("(buffers.v1.enumeration).underlying asked for %d bits. A "+
			"Thrift enum is always i32, with no way to declare otherwise; the narrowing is honoured "+
			"by the FlatBuffers target.", e.Underlying.Bits()))
	}
	if e.BitFlags {
		notes = append(notes, "Declared (bit_flags). Thrift has no bitmask enum; the values are "+
			"emitted as ordinary constants and a reader must not treat them as bit positions.")
	}
	r.doc(b, e.Doc, notes...)

	b.Block(fmt.Sprintf("enum %s {", typeName(string(e.Node), e.Package)), "}", func() {
		for _, v := range e.Values {
			if v.Skip {
				b.Linef("# %s = %d is excluded from this target.", v.Name, v.Number)
				continue
			}
			r.doc(b, v.Doc)
			b.Linef("%s = %d", ident(v.Name), v.Number)
		}
	})
}
