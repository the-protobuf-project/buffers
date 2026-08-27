package ros

// names.go maps proto names onto ROS ones, and substitutes the google.protobuf
// types ROS has no form for.
//
// The sanitizing is the part that matters: ROS's identifier rules are stricter
// than proto's — lowercase only, single underscores between words, never leading
// or trailing — so a perfectly legal proto field name can be an illegal ROS one,
// and the failure lands in a consumer's colcon build rather than anywhere here.

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/names"
)

// messageType projects a message-typed field, substituting the well-known types.
func (r *run) messageType(f *bufir.Field, from *bufir.File) (string, *bufir.Diagnostic) {
	if wrapped, ok := f.WellKnown.Wrapper(); ok {
		// ROS has no optional. Unwrapping loses the presence the wrapper carried,
		// which is worth saying once per field rather than never.
		got, _ := scalar(wrapped)
		if got == "" {
			switch wrapped {
			case bufir.KindString:
				got = "string"
			case bufir.KindBytes:
				got = "uint8[]"
			}
		}
		return got, &bufir.Diagnostic{
			Rule: bufir.RuleLint,
			Node: f.Node,
			Message: fmt.Sprintf("%s is unwrapped to %s; ROS has no optional, so a reader cannot "+
				"distinguish an unset value from the zero value", f.WellKnown, got),
		}
	}

	switch f.WellKnown {
	case bufir.WKTimestamp:
		// An exact match: both are seconds plus nanoseconds since the Unix epoch.
		return "builtin_interfaces/Time", nil
	case bufir.WKDuration:
		return "builtin_interfaces/Duration", nil
	case bufir.WKFieldMask:
		return "string[]", nil
	case bufir.WKEmpty:
		return "std_msgs/Empty", nil
	case bufir.WKAny, bufir.WKStruct, bufir.WKValue, bufir.WKListValue:
		return "string", &bufir.Diagnostic{
			Rule: bufir.RuleTarget,
			Node: f.Node,
			Message: fmt.Sprintf("%s has no ROS equivalent; the field is emitted as a string "+
				"carrying its JSON encoding", f.WellKnown),
			Hint: "model the payload as a message if its shape is known",
		}
	}
	return r.qualify(r.messageRosName(f.Message), from), nil
}

// qualify renders a type reference. ROS names a type by package and type, and
// omits the package within the same one.
func (r *run) qualify(name rosName, from *bufir.File) string {
	if name.Package == "" || name.Package == from.ROSPackage {
		return name.Type
	}
	return name.Package + "/" + name.Type
}

// rosName is a ROS type reference: a package and a type name.
type rosName struct {
	// Package is the ROS package declaring the type, or empty when unqualified.
	Package string
	// Type is the ROS type name.
	Type string
}

// messageRosName returns the ROS name of a proto message.
//
// ROS has no nested types, so a nested proto message is flattened by joining the
// enclosing names — Outer.Inner becomes OuterInner — which keeps two different
// nested types from colliding on one file name.
func (r *run) messageRosName(fullName string) rosName {
	m := r.schema.Messages[bufir.NodeID(fullName)]
	if m == nil {
		return rosName{Type: names.Pascal(shortName(fullName))}
	}
	if m.ROSName != "" && m.ROSName != m.Name {
		return rosName{Package: m.File.ROSPackage, Type: names.Pascal(m.ROSName)}
	}
	return rosName{Package: m.File.ROSPackage, Type: flatten(fullName, m.Package)}
}

// enumRosName returns the ROS name of a proto enum, which is emitted as its own
// constant-carrying message.
func (r *run) enumRosName(fullName string) rosName {
	e := r.schema.Enums[bufir.NodeID(fullName)]
	if e == nil {
		return rosName{Type: names.Pascal(shortName(fullName))}
	}
	return rosName{Package: e.File.ROSPackage, Type: flatten(fullName, e.Package)}
}

// mapEntryName is the generated entry message's name for a map field.
func (r *run) mapEntryName(f *bufir.Field) string {
	owner := shortName(parentOf(string(f.Node)))
	return names.Pascal(owner) + names.Pascal(f.Name) + "Entry"
}

// flatten joins a nested type's path into one ROS type name.
func flatten(fullName, pkg string) string {
	rest := strings.TrimPrefix(fullName, pkg+".")
	parts := strings.Split(rest, ".")
	for i, p := range parts {
		parts[i] = names.Pascal(p)
	}
	return strings.Join(parts, "")
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

// fieldName sanitizes a proto field name into a legal ROS one.
//
// ROS field names must be lowercase alphanumeric with single underscores, and may
// not begin or end with one. Proto names are already snake_case, so this almost
// never changes anything — but `__internal` and `value_` are both legal proto and
// neither is legal ROS.
func fieldName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		isUnderscore := r == '_'
		if isUnderscore && (lastUnderscore || b.Len() == 0) {
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', isUnderscore:
			b.WriteRune(r)
			lastUnderscore = isUnderscore
		default:
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.TrimRight(b.String(), "_")

	// A name that survives sanitizing but cannot stand alone gets a prefix. The
	// two cases are different and the fallback has to leave a legal name in both:
	// "2d_pose" becomes field_2d_pose, but an input that sanitized away to nothing
	// must not become "field_", which ends in an underscore and is exactly what
	// rosidl rejects.
	switch {
	case out == "":
		return "field"
	case out[0] >= '0' && out[0] <= '9':
		return "field_" + out
	}
	return out
}
