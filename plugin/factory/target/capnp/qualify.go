package capnp

// qualify.go turns a proto type name into the reference a Cap'n Proto file writes
// for it, and substitutes the google.protobuf types that have no equivalent here.
//
// The two belong together because they answer one question — what do I write
// where this type is used — and the answer differs by whether the type is the
// user's own, another file's, or one this plugin supplies on proto's behalf.

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
)

// messageType projects a message-typed field, substituting the well-known types.
func (r *run) messageType(f *bufir.Field, from *bufir.File) (string, *bufir.Diagnostic) {
	if wrapped, ok := f.WellKnown.Wrapper(); ok {
		// A proto3 wrapper exists to express presence. Cap'n Proto gives every
		// pointer field a natural absent state and every scalar a default, so the
		// box buys nothing and is unwrapped.
		if got, ok := scalar(wrapped); ok {
			return got, nil
		}
		switch wrapped {
		case bufir.KindString:
			return "Text", nil
		case bufir.KindBytes:
			return "Data", nil
		}
	}

	switch f.WellKnown {
	case bufir.WKTimestamp:
		r.needPrelude(preludeTimestamp)
		return r.preludeRef("Timestamp"), nil
	case bufir.WKDuration:
		r.needPrelude(preludeDuration)
		return r.preludeRef("Duration"), nil
	case bufir.WKEmpty:
		// Cap'n Proto has Void, which is exactly "no value" and occupies no bits.
		// Emitting an empty struct instead would allocate a pointer to say the
		// same nothing.
		return "Void", nil
	case bufir.WKAny:
		r.needPrelude(preludeAny)
		return r.preludeRef("Any"), nil
	case bufir.WKFieldMask:
		return "List(Text)", nil
	case bufir.WKStruct, bufir.WKValue, bufir.WKListValue:
		// AnyPointer is Cap'n Proto's escape hatch and is the honest rendering:
		// the schema genuinely does not know the shape. It is not a good outcome,
		// so it is reported.
		return "AnyPointer", &bufir.Diagnostic{
			Rule: bufir.RuleTarget,
			Node: f.Node,
			Message: fmt.Sprintf("%s is dynamically typed; Cap'n Proto has no equivalent and the field "+
				"is emitted as AnyPointer, which no generated accessor can interpret", f.WellKnown),
			Hint: "model the payload as a message if its shape is known",
		}
	}
	return r.qualify(f.Message, from), nil
}

// qualify renders a reference to a named type.
//
// A type in another file is reached through the `using` alias the importing file
// declares for it — Cap'n Proto has no implicit cross-file scope — so this
// returns `Geometry.Vector3` where FlatBuffers would have used a dotted
// namespace. imports.go owns the aliases.
func (r *run) qualify(fullName string, from *bufir.File) string {
	owner := r.ownerOf(fullName)
	local := typeName(shortName(fullName))

	// A nested proto message is a nested capnp struct, so a reference to it has to
	// name the enclosing type as well.
	if owner != nil {
		if path := r.nestedPath(fullName); path != "" {
			local = path
		}
	}
	if owner == nil || owner.Path == from.Path {
		return local
	}
	return r.aliasFor(owner) + "." + local
}

// nestedPath renders a nested type's dotted path within its file, or "" when the
// type is top-level.
func (r *run) nestedPath(fullName string) string {
	msg := r.schema.Messages[bufir.NodeID(fullName)]
	var pkg string
	switch {
	case msg != nil:
		pkg = msg.Package
	default:
		e := r.schema.Enums[bufir.NodeID(fullName)]
		if e == nil {
			return ""
		}
		pkg = e.Package
	}

	rest := strings.TrimPrefix(fullName, pkg+".")
	parts := strings.Split(rest, ".")
	if len(parts) == 1 {
		return ""
	}
	for i, p := range parts {
		parts[i] = typeName(p)
	}
	return strings.Join(parts, ".")
}

// mapEntryName is the generated entry struct's name for a map field.
func (r *run) mapEntryName(f *bufir.Field) string {
	owner := shortName(parentOf(string(f.Node)))
	return typeName(owner) + typeName(f.Name) + "Entry"
}

// shortName returns the last dotted segment of a full proto name.
func shortName(fullName string) string {
	if i := strings.LastIndex(fullName, "."); i >= 0 {
		return fullName[i+1:]
	}
	return fullName
}

// parentOf returns a full proto name with its last segment removed.
func parentOf(fullName string) string {
	if i := strings.LastIndex(fullName, "."); i >= 0 {
		return fullName[:i]
	}
	return fullName
}
