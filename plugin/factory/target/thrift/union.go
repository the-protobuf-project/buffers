package thrift

// union.go renders a proto oneof as a Thrift union.
//
// The construct matches: a Thrift union is a struct in which exactly one member
// is set, with the discriminant maintained by the generated code, which is what a
// proto oneof is. Two differences follow from Thrift having no nested scope.
//
// First, the union is a top-level declaration rather than an inline group, so it
// needs a name and it has to be emitted before the struct that uses it — see
// order.go for why declaration order matters here.
//
// Second, the arms leave the parent struct entirely. The parent gets one field of
// the union type, and that field needs an id: it takes the lowest arm's proto
// field number, which is free in the parent's id space precisely because the arm
// no longer occupies it, and stable because it does not move when a later arm is
// added.

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// union renders the union type a oneof becomes.
func (r *run) union(b *emit.Buf, f *buffers.File, m *buffers.Message, one *buffers.Oneof) {
	arms := liveArms(one)

	r.doc(b, one.Doc, fmt.Sprintf("The arms of %s.%s. Exactly one member is set; the ids are the "+
		"proto field numbers the arms already had.", m.Name, one.Name))

	b.Block(fmt.Sprintf("union %s {", r.unionTypeName(m, one)), "}", func() {
		for _, arm := range arms {
			typ, diag := r.fieldType(arm, f)
			r.collect(diag)
			if typ == "" {
				continue
			}
			// No modifier: a union member is implicitly optional, and Thrift
			// rejects `required` on one outright.
			r.doc(b, arm.Doc, r.notes(arm)...)
			b.Linef("%d: %s %s", arm.Number, typ, ident(arm.Name))
		}
	})
}

// unionField renders the parent struct's field holding the union.
func (r *run) unionField(b *emit.Buf, m *buffers.Message, one *buffers.Oneof) {
	arms := liveArms(one)
	if len(arms) == 0 {
		return
	}

	r.doc(b, one.Doc, fmt.Sprintf("A proto oneof. The arms live in %s, because Thrift declares a "+
		"union at file scope rather than inside the struct.", r.unionTypeName(m, one)))
	b.Linef("%d: optional %s %s", arms[0].Number, r.unionTypeName(m, one), ident(one.Name))
}

// unionTypeName is the generated union's type name, derived from the owning
// message's flattened name so that two nested messages declaring a oneof of the
// same name do not collide in Thrift's single flat scope.
func (r *run) unionTypeName(m *buffers.Message, one *buffers.Oneof) string {
	return unionName(typeName(string(m.Node), m.Package), one.Name)
}
