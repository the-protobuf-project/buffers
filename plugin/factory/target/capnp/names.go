package capnp

import (
	"strings"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/names"
)

// names.go enforces Cap'n Proto's identifier rules, which are stricter than
// proto's and not a matter of style.
//
// capnp requires a type name to begin with an uppercase letter and a field,
// enumerant or method name to begin with a lowercase one. It is a parse error,
// not a lint: `display_name @1 :Text;` does not compile. Since proto names are
// snake_case throughout, every field name in every emitted schema goes through
// here.
//
// The renaming is a real consequence to understand, not an implementation
// detail. A Cap'n Proto consumer sees `displayName` where the proto author wrote
// `display_name`, so the two names for one field differ by convention on each
// side — exactly as they already do between proto and protojson. What must never
// differ is the *slot*, and that is what buffers.lock protects.

// keywords are the words capnp's grammar claims. A field named after one is a
// parse error, so it is suffixed rather than emitted.
//
// The type names are included because capnp resolves a bare identifier against
// the builtin scope: a struct named `List` shadows the list type and produces
// errors nowhere near the declaration.
var keywords = map[string]bool{
	"struct": true, "enum": true, "interface": true, "union": true,
	"group": true, "const": true, "annotation": true, "using": true,
	"import": true, "extends": true, "in": true, "of": true, "on": true,
	"as": true, "with": true, "from": true, "fixed": true,

	"Void": true, "Bool": true, "Int8": true, "Int16": true, "Int32": true,
	"Int64": true, "UInt8": true, "UInt16": true, "UInt32": true,
	"UInt64": true, "Float32": true, "Float64": true, "Text": true,
	"Data": true, "List": true, "AnyPointer": true,
}

// member renders a proto snake_case name as a capnp member name: camelCase with
// a lowercase initial.
func member(protoName string) string {
	out := names.Camel(protoName)
	if out == "" {
		return "field"
	}
	if keywords[out] {
		// capnp identifiers may carry a trailing underscore, which is the least
		// surprising disambiguation available and matches what capnp's own
		// generators do for host-language keywords.
		return out + "_"
	}
	return out
}

// typeName renders a proto type name as a capnp type name: PascalCase with an
// uppercase initial.
//
// Proto type names are already PascalCase, so this is usually the identity. It
// exists for the cases that are not — a message named `IMU` stays `IMU`, and one
// named `point_cloud` becomes `PointCloud`.
func typeName(protoName string) string {
	out := names.Pascal(protoName)
	if out == "" {
		return "Type"
	}
	if keywords[out] {
		return out + "_"
	}
	return out
}

// enumerant renders a proto enum value as a capnp enumerant.
//
// Proto enum values are SCREAMING_SNAKE and, per AIP-126, prefixed with the enum
// name: `SENSOR_KIND_LIDAR` on `SensorKind`. Carrying that through would produce
// `sensorKindLidar`, which restates the enum's name at every use site —
// `SensorKind.sensorKindLidar`. The prefix is stripped where it is present, which
// is what makes the result read the way a capnp author would have written it.
//
// The prefix is only stripped when something legal remains: an enumerant may not
// begin with a digit, and stripping `KIND_` from `KIND_2D` would leave one.
func enumerant(enumName, valueName string) string {
	prefix := names.ScreamingSnake(enumName) + "_"
	trimmed := valueName
	if strings.HasPrefix(valueName, prefix) {
		if rest := valueName[len(prefix):]; rest != "" && !startsWithDigit(rest) {
			trimmed = rest
		}
	}
	out := names.Camel(strings.ToLower(trimmed))
	if out == "" {
		return "value"
	}
	if keywords[out] {
		return out + "_"
	}
	return out
}

// startsWithDigit reports whether a name would begin with a digit, which is not
// a legal Cap'n Proto identifier.
func startsWithDigit(s string) bool { return s != "" && s[0] >= '0' && s[0] <= '9' }

// cxxNamespace renders a dotted proto package as a C++ namespace, which is what
// $Cxx.namespace takes.
func cxxNamespace(pkg string) string { return strings.ReplaceAll(pkg, ".", "::") }
