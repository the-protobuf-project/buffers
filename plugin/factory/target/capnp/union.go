package capnp

// union.go renders the two constructs Cap'n Proto expresses natively but proto
// spells differently: a oneof, which is a union here, and a map, which is not.
//
// The pairing is deliberate. A oneof is the case where Cap'n Proto fits proto
// better than any other target — same construct, discriminant and all — and a map
// is the case where it fits worst, having neither a map type nor a keyed-list
// convention to preserve the lookup.

import (
	"fmt"
	"sort"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// union renders a proto oneof as a named Cap'n Proto union.
//
// This is the one place Cap'n Proto is a better fit than any other target: a
// proto oneof and a capnp union are the same construct, discriminant and all, and
// the arms keep their own ordinals in the enclosing struct's space.
func (r *run) union(b *emit.Buf, f *buffers.File, one *buffers.Oneof, used map[int32]bool) {
	arms := make([]*buffers.Field, 0, len(one.Fields))
	for _, arm := range one.Fields {
		if arm.Skip || !allows(arm.Targets) {
			continue
		}
		arms = append(arms, arm)
	}
	sort.SliceStable(arms, func(i, j int) bool { return arms[i].Ordinal < arms[j].Ordinal })

	b.Doc("#", one.Doc)

	if len(arms) < 2 {
		// A Cap'n Proto union must have at least two members. A one-armed proto
		// oneof is legal and means "this field, or nothing", which is what a plain
		// optional field already says here.
		if len(arms) == 1 {
			b.Linef("# oneof %q has a single arm; Cap'n Proto requires two in a union, and a", one.Name)
			b.Line("# lone arm carries no more information than a plain field.")
			r.field(b, f, arms[0])
			used[arms[0].Ordinal] = true
		}
		return
	}

	b.Block(fmt.Sprintf("%s :union {", member(one.Name)), "}", func() {
		for _, arm := range arms {
			typ, diag := r.fieldType(arm, f)
			r.collect(diag)
			b.Doc("#", arm.Doc)
			b.Linef("%s @%d :%s;", member(arm.Name), arm.Ordinal, typ)
			used[arm.Ordinal] = true
		}
	})
}

// mapEntries returns the map fields of a message that need an entry struct.
func (r *run) mapEntries(m *buffers.Message) []*buffers.Field {
	var out []*buffers.Field
	for _, f := range m.Fields {
		if f.Kind == buffers.KindMap && !f.Skip && allows(f.Targets) {
			out = append(out, f)
		}
	}
	return out
}

// mapEntry renders the two-field struct a proto map is rewritten into.
func (r *run) mapEntry(b *emit.Buf, f *buffers.File, field *buffers.Field) {
	keyType, diag := r.baseType(field.MapKey, f)
	r.collect(diag)
	valType, diag := r.baseType(field.MapValue, f)
	r.collect(diag)

	b.Linef("# Entry of %s, which is a proto map. Cap'n Proto has no map type and no", member(field.Name))
	b.Line("# keyed-list convention, so a list of pairs is the whole of what it can say:")
	b.Line("# uniqueness and lookup are the reader's responsibility.")
	b.Block(fmt.Sprintf("struct %s @0x%016x {", r.mapEntryName(field), buffers.DeriveTypeID(string(field.Node)+".entry")), "}", func() {
		b.Linef("key @0 :%s;", keyType)
		b.Linef("value @1 :%s;", valType)
	})
}
