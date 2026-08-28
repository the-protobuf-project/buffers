package thrift

// qualify.go turns a proto type name into the reference a .thrift writes for it,
// and substitutes the google.protobuf types Thrift has no form for.
//
// The two belong together because they answer one question — what do I write
// where this type is used — and the answer differs by whether the type is the
// file's own, another file's, or one this plugin supplies on proto's behalf.

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"
)

// messageType projects a message-typed field, substituting the well-known types.
func (r *run) messageType(f *buffers.Field, from *buffers.File) (string, *buffers.Diagnostic) {
	if wrapped, ok := f.WellKnown.Wrapper(); ok {
		// A proto3 wrapper exists to express presence, and Thrift expresses that
		// with the `optional` modifier struct.go already applies to it. Emitting a
		// one-field struct instead would keep the box and lose the point.
		if got, _, ok := scalar(wrapped); ok {
			// The unwrapped kind's caveats travel with it: a boxed uint32 is
			// still a uint32 landing on a signed i32.
			return got, r.unsignedDiag(f)
		}
		switch wrapped {
		case buffers.KindString:
			return "string", nil
		case buffers.KindBytes:
			return "binary", nil
		}
	}

	switch f.WellKnown {
	case buffers.WKTimestamp:
		r.needPrelude(preludeTimestamp)
		return r.preludeRef("Timestamp"), nil
	case buffers.WKDuration:
		r.needPrelude(preludeDuration)
		return r.preludeRef("Duration"), nil
	case buffers.WKEmpty:
		r.needPrelude(preludeEmpty)
		return r.preludeRef("Empty"), nil
	case buffers.WKAny:
		r.needPrelude(preludeAny)
		return r.preludeRef("Any"), nil
	case buffers.WKFieldMask:
		// A FieldMask is a list of paths, and Thrift has a list of strings. No
		// record is needed and none is emitted.
		return "list<string>", nil
	case buffers.WKStruct, buffers.WKValue, buffers.WKListValue:
		// Dynamically typed JSON. Thrift is a static schema language and has no
		// equivalent, so the honest rendering is the JSON text itself — which is
		// a real cost and therefore reported.
		return "string", &buffers.Diagnostic{
			Rule: buffers.RuleTarget,
			Node: f.Node,
			Message: fmt.Sprintf("%s is dynamically typed and has no Thrift equivalent; the field is "+
				"emitted as a string carrying its JSON encoding", f.WellKnown),
			Hint: "model the payload as a message if its shape is known, or accept the JSON round trip",
		}
	}
	return r.qualify(f.Message, from), nil
}

// qualify renders a reference to a named type.
//
// Thrift scopes an included file's types under the *base name* of the file, not
// under any namespace it declares — so a type from `sensors/v1/geometry.thrift`
// is written `geometry.Vector3` regardless of what that file's `namespace` says.
// That is also why includes.go has to check for two includes sharing a base name:
// the prefix is the file name, and two files cannot both own it.
func (r *run) qualify(fullName string, from *buffers.File) string {
	owner := r.ownerOf(fullName)
	if owner == nil {
		// A type the graph does not index — a dependency outside the generate
		// set. The short name is the best available reference, and the include
		// this needs is reported by includes.go rather than guessed at here.
		return shortName(fullName)
	}

	local := typeName(fullName, owner.Package)
	if owner.Path == from.Path {
		return local
	}
	return r.prefixFor(owner) + "." + local
}
