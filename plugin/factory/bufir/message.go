package bufir

// message.go holds the record types: a message and the fields it declares.

// Message is one proto message.
type Message struct {
	// Node is the message's fully qualified proto name.
	Node NodeID
	Name string // "Sensor"
	// Package is the proto package declaring it.
	Package string
	// File is the file declaring it.
	File *File
	// Doc is the leading comment, as prose.
	Doc string

	// Layout is resolved — never LayoutUnspecified after a successful build.
	Layout Layout

	// CapnpID is this type's Cap'n Proto ID, declared or derived.
	CapnpID uint64

	// ROSName is the ROS type name, which is Name unless overridden.
	ROSName string

	// Targets restricts the message to the named targets; empty means all.
	Targets []string

	// Skip is set by buffers.v1's skip option. A skipped message is still
	// indexed, because a field elsewhere may still name it and the diagnostic for
	// that is better than a dangling reference.
	Skip bool

	// OriginalOrder emits FlatBuffers' (original_order).
	OriginalOrder bool

	// FBSRoot marks this message as its file's FlatBuffers root_type. At most one
	// message per file may set it; the target reports it when more do.
	FBSRoot bool

	// Fields are in ordinal order, which is ascending proto field number. Fields
	// inside a oneof appear here too, once each, with Oneof set.
	Fields []*Field

	// Oneofs are the declared oneofs, excluding the synthetic ones proto3
	// `optional` generates — those are presence, not a union, and rendering them
	// as one would put a spurious two-armed union in every schema.
	Oneofs []*Oneof

	// Reserved are the ordinals that are occupied by a removed field. They carry
	// no data and exist so that the fields after them do not move. See ordinal.go.
	Reserved []Slot

	// Nested are the messages declared inside it. Targets without nesting flatten
	// these into siblings.
	Nested []*Message
	// Enums are the enums declared inside it.
	Enums []*Enum

	// IsMapEntry marks the synthetic message protoc generates for a map field.
	// Every target rewrites these rather than emitting them literally, since no
	// target has proto's map type.
	IsMapEntry bool

	// Resource is the AIP-123 descriptor when the message declares one.
	Resource *Resource

	// WellKnown classifies the google.protobuf.* types that every target has an
	// opinion about.
	WellKnown WellKnown

	// reserved holds the message's `reserved N to M;` spans, captured during the
	// walk. It is the ranges rather than the descriptor deliberately: retaining a
	// protoreflect.MessageDescriptor on every node would keep the whole
	// descriptor graph alive for the sake of two integers per span.
	reserved []reservedRange
}

// Field returns the field with the given proto name, or nil.
func (m *Message) Field(name string) *Field {
	for _, f := range m.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// Live returns the fields that survive to output: not skipped, and belonging to
// the given target. Pass an empty target to filter on skip alone.
func (m *Message) Live(target string) []*Field {
	out := make([]*Field, 0, len(m.Fields))
	for _, f := range m.Fields {
		if f.Skip || !allows(f.Targets, target) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// Field is one field of a message.
type Field struct {
	// Node is the field's fully qualified proto name.
	Node   NodeID
	Name   string // proto name: "display_name"
	Number int32  // proto field number: 1-based, sparse

	// Ordinal is the target slot: 0-based, contiguous within the message,
	// and stable across builds. It is what a .capnp `@N` and a .fbs `id:` are
	// rendered from. See ordinal.go.
	Ordinal int32

	// OrdinalSource records where Ordinal came from, which is what a diagnostic
	// about a moved slot needs in order to be actionable.
	OrdinalSource OrdinalSource

	// Doc is the leading comment, as prose.
	Doc string
	// Kind is the field's proto type, kept at proto granularity.
	Kind Kind

	// Message is the full proto name when Kind is KindMessage.
	Message string

	// Enum is the full proto name when Kind is KindEnum.
	Enum string

	// WellKnown classifies a google.protobuf.* message field.
	WellKnown WellKnown

	// Repeated reports whether the field is a list.
	Repeated bool

	// Optional is proto3 explicit presence — the `optional` keyword. It is not
	// the same as "not required": AIP-203 REQUIRED is separate and orthogonal.
	Optional bool

	// MapKey and MapValue describe a map field's entry when Kind is KindMap.
	MapKey, MapValue *Field

	// Oneof is the union this field belongs to, or nil.
	Oneof *Oneof

	// Behavior is google.api.field_behavior (AIP-203).
	Behavior []Behavior

	// RefType is google.api.resource_reference's target type (AIP-124), and
	// RefChild reports whether it was declared as a child_type.
	RefType string
	// RefChild reports whether RefType was declared as a child_type.
	RefChild bool

	// Format is google.api.field_info's format, e.g. "UUID4".
	Format string

	// Skip excludes the field, which still consumes its ordinal.
	Skip bool
	// Key marks the FlatBuffers (key) field: the one a vector of this table may
	// be sorted by and binary-searched on.
	Key bool
	// Shared emits FlatBuffers' (shared) on a string, so equal values are written
	// once.
	Shared bool
	// MaxLen bounds a string or repeated field for ROS, where a bound is part of
	// the type.
	MaxLen uint32
	// FixedLen makes a repeated field a fixed-size array. proto cannot express a
	// length, so this is a claim the producer upholds.
	FixedLen uint32
	// ROSDefault is a literal default for the ROS field. proto3 has none.
	ROSDefault string
	// CapnpGroup folds this field, with others naming the same group, into a
	// Cap'n Proto group — presentation over the same slots.
	CapnpGroup string
	// Targets restricts the field to the named targets; empty means all.
	Targets []string
}

// Has reports whether the field carries the given AIP-203 behavior.
func (f *Field) Has(b Behavior) bool {
	for _, got := range f.Behavior {
		if got == b {
			return true
		}
	}
	return false
}

// Required reports whether AIP-203 marks the field REQUIRED, which is what turns
// into a FlatBuffers (required) attribute and a documented Cap'n Proto contract.
func (f *Field) Required() bool { return f.Has(BehaviorRequired) }
