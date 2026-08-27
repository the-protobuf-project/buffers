package flatbuffers

// union.go renders the two proto constructs FlatBuffers has no direct form for: a
// map, which becomes a vector of keyed entry tables, and a oneof, which becomes a
// union with a wrapper table per non-table arm.
//
// Both substitutions preserve something beyond the data. The map keeps its
// lookup, because flatc binary-searches a vector sorted on a (key) field; the
// union keeps its discriminant, because that is what a union is.

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/names"
)

// mapEntries emits the keyed entry table each map field is rewritten into.
func (r *run) mapEntries(b *emit.Buf, f *buffers.File, m *buffers.Message) {
	for _, field := range m.Fields {
		if field.Kind != buffers.KindMap || field.Skip || !allows(field.Targets) {
			continue
		}
		keyType, diag := r.baseType(field.MapKey, f)
		r.collect(diag)
		valType, diag := r.baseType(field.MapValue, f)
		r.collect(diag)

		// `(key)` is what gives flatc a binary search over a sorted vector, so
		// the substitution preserves map lookup rather than only map data. It is
		// only legal on a scalar or string key, which is every proto map key
		// type, so no guard is needed here.
		b.Line("")
		b.Linef("/// Entry of %s.%s, which is a proto map.", m.Name, field.Name)
		b.Linef("/// FlatBuffers has no map type; a vector of tables with a (key) field is the")
		b.Linef("/// idiom, and it keeps the lookup rather than only the data.")
		b.Block(fmt.Sprintf("table %s {", r.mapEntryName(field)), "}", func() {
			b.Linef("key:%s (key);", keyType)
			b.Linef("value:%s;", valType)
		})
	}
}

// unionTypes emits each oneof's union declaration and any wrapper tables its arms
// need.
//
// A FlatBuffers union may only hold tables. A proto oneof may hold anything, so
// every arm that is not already a table — a scalar, a string, an enum, or a
// struct — gets a one-field wrapper. Wrapping structs too is deliberate: newer
// flatc accepts a struct arm, older ones do not, and a schema that compiles
// everywhere is worth one indirection on an arm nobody reads in a hot loop.
func (r *run) unionTypes(b *emit.Buf, f *buffers.File, m *buffers.Message) {
	for _, one := range m.Oneofs {
		if one.Skip {
			continue
		}
		arms := unionArms(one)
		if len(arms) == 0 {
			continue
		}

		members := make([]string, 0, len(arms))
		for _, arm := range arms {
			if table, ok := r.armIsTable(arm); ok {
				members = append(members, table)
				continue
			}
			wrapper := one.UnionName + names.Pascal(arm.Name)
			typ, diag := r.fieldType(arm, f)
			r.collect(diag)

			b.Line("")
			b.Linef("/// Wrapper for the %q arm of %s.%s.", arm.Name, m.Name, one.Name)
			b.Linef("/// A FlatBuffers union may only hold tables, so a non-table arm is boxed.")
			b.Block(fmt.Sprintf("table %s {", wrapper), "}", func() {
				b.Linef("value:%s;", typ)
			})
			members = append(members, wrapper)
		}

		b.Line("")
		b.Doc("///", one.Doc)
		b.Linef("/// Arms are ordered by proto field number: a FlatBuffers union's")
		b.Linef("/// discriminants are positional, so appending is safe and reordering is not.")
		b.Linef("union %s { %s }", one.UnionName, strings.Join(members, ", "))
	}
}

// armIsTable reports whether a oneof arm already refers to a table, and its
// FlatBuffers name if so.
func (r *run) armIsTable(arm *buffers.Field) (string, bool) {
	if arm.Kind != buffers.KindMessage || arm.Repeated || arm.WellKnown != buffers.WKNone {
		return "", false
	}
	target := r.schema.Messages[buffers.NodeID(arm.Message)]
	if target == nil || target.Layout != buffers.LayoutTable {
		return "", false
	}
	return r.qualify(arm.Message, target.File), true
}
