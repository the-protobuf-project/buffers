package flatbuffers

// qualify.go turns a proto type name into what a .fbs writes where the type is
// used, and substitutes the google.protobuf types FlatBuffers has no form for.

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/names"
)

// messageType projects a message-typed field, substituting the well-known types
// FlatBuffers cannot represent directly.
func (t *run) messageType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if wrapped, ok := f.WellKnown.Wrapper(); ok {
		// A wrapper's whole purpose in proto3 is to express presence, and
		// FlatBuffers expresses that natively with a `= null` default. Emitting a
		// one-field table instead would keep the box and lose the point.
		got, _ := scalar(wrapped)
		if got == "" {
			// StringValue and BytesValue box types that are already nullable in
			// FlatBuffers, since both are offsets.
			switch wrapped {
			case buffers.KindString:
				return "string", nil
			case buffers.KindBytes:
				return "[ubyte]", nil
			}
		}
		return got, nil
	}

	switch f.WellKnown {
	case buffers.WKTimestamp:
		t.needPrelude(preludeTimestamp)
		return preludeNamespace + ".Timestamp", nil
	case buffers.WKDuration:
		t.needPrelude(preludeDuration)
		return preludeNamespace + ".Duration", nil
	case buffers.WKEmpty:
		t.needPrelude(preludeEmpty)
		return preludeNamespace + ".Empty", nil
	case buffers.WKAny:
		t.needPrelude(preludeAny)
		return preludeNamespace + ".Any", nil
	case buffers.WKFieldMask:
		// A FieldMask is a list of paths. FlatBuffers has a vector of strings,
		// which is exactly that, so no record is needed.
		return "[string]", nil
	case buffers.WKStruct, buffers.WKValue, buffers.WKListValue:
		// These are dynamically typed JSON. FlatBuffers is a static schema
		// format and has no equivalent; anything emitted here would be a lie
		// about what the consumer gets.
		return "string", &buffers.Diagnostic{
			Rule: buffers.RuleTarget,
			Node: f.Node,
			Message: fmt.Sprintf("%s is dynamically typed and has no FlatBuffers equivalent; "+
				"the field is emitted as a string carrying its JSON encoding", f.WellKnown),
			Hint: "model the payload as a message if its shape is known, or accept the JSON round trip",
		}
	}
	return t.qualify(f.Message, from), nil
}

// qualify renders a reference to a named type, using the bare name when the
// referent shares the referring file's namespace and the fully qualified one
// otherwise.
//
// Always qualifying would be simpler and always legal, but it reads badly: every
// field in a single-namespace schema would carry a redundant prefix, and a schema
// people have to read is worth the conditional.
func (t *run) qualify(fullName string, from *buffers.File) string {
	name := fullName
	if i := strings.LastIndex(fullName, "."); i >= 0 {
		name = fullName[i+1:]
	}

	owner := t.ownerOf(fullName)
	if owner == nil || owner.Namespace == from.Namespace {
		return name
	}
	return owner.Namespace + "." + name
}

// mapEntryName is the generated entry table's name for a map field. It is
// derived from the owning message and the field so that two maps in one schema
// cannot collide.
func (t *run) mapEntryName(f *buffers.Field) string {
	owner := string(f.Node)
	if i := strings.LastIndex(owner, "."); i >= 0 {
		owner = owner[:i]
	}
	if i := strings.LastIndex(owner, "."); i >= 0 {
		owner = owner[i+1:]
	}
	return owner + names.Pascal(f.Name) + "Entry"
}
