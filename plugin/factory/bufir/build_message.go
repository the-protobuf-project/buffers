package bufir

// build_message.go walks messages and their fields, including the map entries and
// oneofs that every target has to rewrite.

import (
	"sort"

	"github.com/the-protobuf-project/protokit/naming"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// message walks one message and everything declared inside it.
func (b *builder) message(m *protogen.Message, file *File, parent *Message) *Message {
	md := m.Desc
	full := NodeID(md.FullName())
	opts := messageOpts(md)

	msg := &Message{
		Node:          full,
		Name:          string(md.Name()),
		Package:       string(md.ParentFile().Package()),
		File:          file,
		Doc:           docOf(m.Comments.Leading),
		Layout:        layoutOf(opts.GetLayout()),
		CapnpID:       resolveCapnpID(opts.GetCapnpId(), func() uint64 { return typeCapnpID(string(full)) }),
		ROSName:       orDefault(opts.GetRosType(), string(md.Name())),
		Targets:       append([]string(nil), opts.GetTargets()...),
		Skip:          opts.GetSkip(),
		OriginalOrder: opts.GetFbsOriginalOrder(),
		FBSRoot:       opts.GetFbsRoot(),
		IsMapEntry:    md.IsMapEntry(),
		Resource:      resourceOf(md),
		WellKnown:     classifyWellKnown(string(full)),
		reserved:      reservedRangesOf(md),
	}

	b.schema.Messages[full] = msg
	b.owner[full] = file.Path
	if parent == nil {
		file.Messages = append(file.Messages, msg)
	} else {
		parent.Nested = append(parent.Nested, msg)
	}

	// Oneofs first: a field needs the oneof it belongs to, and the synthetic ones
	// proto3 `optional` generates are presence rather than a union — rendering
	// them as one would put a spurious single-armed union in every schema.
	byOneof := map[protoreflect.FullName]*Oneof{}
	for _, o := range m.Oneofs {
		if o.Desc.IsSynthetic() {
			continue
		}
		oopts := oneofOpts(o.Desc)
		one := &Oneof{
			Node:      NodeID(o.Desc.FullName()),
			Name:      string(o.Desc.Name()),
			Doc:       docOf(o.Comments.Leading),
			Parent:    msg,
			UnionName: orDefault(oopts.GetUnionType(), msg.Name+naming.PascalGo(string(o.Desc.Name()))),
			Skip:      oopts.GetSkip(),
		}
		msg.Oneofs = append(msg.Oneofs, one)
		byOneof[o.Desc.FullName()] = one
	}

	for _, f := range m.Fields {
		field := b.field(f, msg, byOneof)
		msg.Fields = append(msg.Fields, field)
		if field.Oneof != nil {
			field.Oneof.Fields = append(field.Oneof.Fields, field)
		}
	}

	// Fields are stored in ordinal order, which is ascending proto field number.
	// Doing it here rather than at render time means every target sees one order
	// and none of them has to remember to sort.
	sort.SliceStable(msg.Fields, func(i, j int) bool { return msg.Fields[i].Number < msg.Fields[j].Number })

	if msg.Resource != nil {
		for _, f := range msg.Fields {
			if f.Has(BehaviorIdentifier) {
				msg.Resource.NameField = f.Name
				break
			}
		}
		if msg.Resource.NameField == "" {
			// AIP-122 says the field is `name`; IDENTIFIER is the authority, but a
			// resource that declares neither still has to be addressable somehow.
			msg.Resource.NameField = "name"
		}
	}

	for _, n := range m.Messages {
		b.message(n, file, msg)
	}
	for _, e := range m.Enums {
		b.enum(e, file, msg)
	}
	return msg
}

// field walks one field, unfolding a map entry into its key and value.
func (b *builder) field(f *protogen.Field, parent *Message, oneofs map[protoreflect.FullName]*Oneof) *Field {
	fd := f.Desc
	opts := fieldOpts(fd)
	refType, refChild := referenceOf(fd)

	field := &Field{
		Node:       NodeID(fd.FullName()),
		Name:       string(fd.Name()),
		Number:     int32(fd.Number()),
		Doc:        docOf(f.Comments.Leading),
		Kind:       classifyKind(fd),
		Repeated:   fd.Cardinality() == protoreflect.Repeated,
		Optional:   fd.HasOptionalKeyword(),
		Behavior:   behaviors(fd),
		RefType:    refType,
		RefChild:   refChild,
		Format:     formatOf(fd),
		Skip:       opts.GetSkip(),
		Key:        opts.GetKey(),
		Shared:     opts.GetShared(),
		MaxLen:     nonNegative(opts.GetMaxLen()),
		FixedLen:   nonNegative(opts.GetFixedLen()),
		ROSDefault: opts.GetRosDefault(),
		CapnpGroup: opts.GetCapnpGroup(),
		Targets:    append([]string(nil), opts.GetTargets()...),
	}
	if opts != nil && opts.Ordinal != 0 {
		field.Ordinal = opts.GetOrdinal()
	}

	switch {
	case fd.IsMap():
		// protoreflect spells a map as a repeated message field over a synthetic
		// entry type. Every target rewrites that — none of them has proto's map —
		// so the entry is unfolded here into a key and a value rather than left
		// for four renderers to unfold identically.
		field.Kind = KindMap
		field.Repeated = false
		field.MapKey = b.entryField(fd.MapKey())
		field.MapValue = b.entryField(fd.MapValue())
	case field.Kind == KindMessage:
		field.Message = string(fd.Message().FullName())
		field.WellKnown = classifyWellKnown(field.Message)
	case field.Kind == KindEnum:
		field.Enum = string(fd.Enum().FullName())
	}

	if od := fd.ContainingOneof(); od != nil && !od.IsSynthetic() {
		field.Oneof = oneofs[od.FullName()]
	}
	return field
}

// entryField builds the key or value half of a map entry. It is a Field so that
// targets can reuse their normal type projection on it, but it never gets an
// ordinal: a map entry's two slots are fixed at 0 and 1 by every target that has
// to synthesize one.
func (b *builder) entryField(fd protoreflect.FieldDescriptor) *Field {
	entry := &Field{
		Name:   string(fd.Name()),
		Number: int32(fd.Number()),
		Kind:   classifyKind(fd),
	}
	switch entry.Kind {
	case KindMessage:
		entry.Message = string(fd.Message().FullName())
		entry.WellKnown = classifyWellKnown(entry.Message)
	case KindEnum:
		entry.Enum = string(fd.Enum().FullName())
	}
	return entry
}
