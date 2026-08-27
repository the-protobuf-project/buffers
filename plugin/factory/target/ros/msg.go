package ros

// msg.go renders .msg files: one per message, one per enum, and one per map field.
//
// ROS has no nested types and no enums, so each of those becomes a file of its
// own — which is why a single proto file can produce a dozen of them.

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// run is one Generate call's mutable state.
type run struct {
	*Target
	// schema is the graph being rendered.
	schema *buffers.Schema
	// topics accumulates publications for the manifest, which is the only
	// schema-derived record of the topic-to-type binding.
	topics []topic
	// diags accumulates problems found while projecting types.
	diags []buffers.Diagnostic
}

// file renders every artifact one proto file contributes.
func (r *run) file(f *buffers.File) error {
	for _, m := range flattenMessages(f) {
		if err := r.message(f, m); err != nil {
			return err
		}
	}
	for _, e := range flattenEnums(f) {
		if err := r.enum(f, e); err != nil {
			return err
		}
	}
	for _, s := range f.Services {
		if s.Skip || !allows(s.Targets) {
			continue
		}
		if err := r.service(f, s); err != nil {
			return err
		}
	}
	return nil
}

// message renders one .msg file.
func (r *run) message(f *buffers.File, m *buffers.Message) error {
	var b emit.Buf
	b.Raw(r.banner(f.Path))
	b.Line("")
	b.Doc("#", m.Doc)
	if m.Doc != "" {
		b.Line("")
	}

	r.fields(&b, f, m)

	name := r.messageRosName(string(m.Node))
	if err := r.sink(msgPath(f.ROSPackage, name.Type), b.Bytes()); err != nil {
		return err
	}
	return r.mapEntries(f, m)
}

// fields renders a message's fields, its oneof discriminants, and the notes that
// explain what ROS dropped.
func (r *run) fields(b *emit.Buf, f *buffers.File, m *buffers.Message) {
	// A oneof has no ROS form. The arms are emitted as ordinary fields alongside a
	// constant block naming which one is set, which is the convention ROS users
	// reach for — and is advisory: nothing prevents a writer setting two.
	for _, one := range m.Oneofs {
		if one.Skip {
			continue
		}
		arms := liveArms(one)
		if len(arms) == 0 {
			continue
		}
		r.collect(&buffers.Diagnostic{
			Rule: buffers.RuleTarget,
			Node: one.Node,
			Message: fmt.Sprintf("oneof %q has no ROS equivalent; its arms are emitted as ordinary fields "+
				"with a %s_CASE constant block, and nothing enforces that only one is set",
				one.Name, strings.ToUpper(fieldName(one.Name))),
			Hint: "readers must check the case field; writers must set exactly one arm and the matching case",
		})

		b.Linef("# Discriminant for the %q oneof. ROS has no union, so this says which arm", one.Name)
		b.Line("# is meaningful; the arms themselves are ordinary fields below.")
		prefix := strings.ToUpper(fieldName(one.Name))
		b.Linef("uint8 %s_NOT_SET=0", prefix)
		for i, arm := range arms {
			b.Linef("uint8 %s_%s=%d", prefix, strings.ToUpper(fieldName(arm.Name)), i+1)
		}
		b.Linef("uint8 %s_case", fieldName(one.Name))
		b.Line("")
	}

	for _, field := range m.Fields {
		if field.Skip || !allows(field.Targets) {
			continue
		}
		typ, diag := r.fieldType(field, f)
		r.collect(diag)

		b.Doc("#", field.Doc)
		if field.Required() {
			b.Line("# REQUIRED (AIP-203); ROS cannot enforce it.")
		}
		if field.Repeated && field.FixedLen == 0 && field.MaxLen == 0 {
			b.Line("# Unbounded sequence. Consider (buffers.v1.field).max_len if this message is")
			b.Line("# published on a real-time path.")
		}

		line := fmt.Sprintf("%s %s", typ, fieldName(field.Name))
		if field.ROSDefault != "" {
			line += " " + field.ROSDefault
		}
		b.Line(line)
	}
}

// mapEntries emits the entry message each map field is rewritten into.
func (r *run) mapEntries(f *buffers.File, m *buffers.Message) error {
	for _, field := range m.Fields {
		if field.Kind != buffers.KindMap || field.Skip || !allows(field.Targets) {
			continue
		}
		keyType, diag := r.baseType(field.MapKey, f)
		r.collect(diag)
		valType, diag := r.baseType(field.MapValue, f)
		r.collect(diag)

		var b emit.Buf
		b.Raw(r.banner(f.Path))
		b.Line("")
		b.Linef("# Entry of %s.%s, which is a proto map.", m.Name, field.Name)
		b.Line("# ROS has no map type and no keyed-array convention, so uniqueness and lookup")
		b.Line("# are the reader's responsibility.")
		b.Line("")
		b.Linef("%s key", keyType)
		b.Linef("%s value", valType)

		if err := r.sink(msgPath(f.ROSPackage, r.mapEntryName(field)), b.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

// enum renders a proto enum as its own constant-carrying message.
func (r *run) enum(f *buffers.File, e *buffers.Enum) error {
	var b emit.Buf
	b.Raw(r.banner(f.Path))
	b.Line("")
	b.Doc("#", e.Doc)
	b.Line("#")
	b.Line("# ROS has no enum type. The values are typed constants and `value` carries one")
	b.Line("# of them; a field elsewhere referring to this enum has this message as its")
	b.Line("# type. Nothing constrains `value` to a declared constant.")
	b.Line("")

	typ := width(e.Underlying)
	for _, v := range e.Values {
		if v.Skip {
			continue
		}
		b.Doc("#", v.Doc)
		b.Linef("%s %s=%d", typ, v.Name, v.Number)
	}
	b.Line("")
	b.Linef("%s value", typ)

	name := r.enumRosName(string(e.Node))
	return r.sink(msgPath(f.ROSPackage, name.Type), b.Bytes())
}
