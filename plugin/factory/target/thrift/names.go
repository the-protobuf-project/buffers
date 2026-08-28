package thrift

// names.go enforces Thrift's identifier rules, which are the mildest of any
// target here.
//
// Thrift's grammar accepts `[a-zA-Z_][a-zA-Z0-9._]*` for every kind of name, so
// a proto identifier is almost always already legal: `display_name` stays
// `display_name`, `SENSOR_KIND_LIDAR` stays itself, and `GetSensor` stays
// `GetSensor`. That is deliberate rather than lazy — where a target's grammar
// does not force a rename, the proto name *is* the mapping, and rewriting it
// would put a second spelling between the producer and the consumer for nothing.
// Compare Cap'n Proto, which rewrites every member because a capital initial is a
// parse error there.
//
// Two things do force a change. The Thrift compiler rejects a name that is one of
// its own keywords or one of the host-language words it reserves, and Thrift has
// no nested scope, so a nested proto type has to fold its path into its name.

import (
	"strings"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/names"
)

// reserved are the words the Thrift compiler refuses as an identifier: its own
// IDL keywords, its builtin type names, and the union of the host-language
// keywords it reserves so that generated code compiles everywhere.
//
// The host-language half is the surprising one. `from` is not a Thrift keyword,
// but a field named `from` breaks the generated Python, so thrift rejects it up
// front — which means a .proto with a `from` field fails here and nowhere else.
var reserved = map[string]bool{
	// IDL keywords and builtin types.
	"namespace": true, "include": true, "cpp_include": true, "typedef": true,
	"struct": true, "union": true, "exception": true, "extends": true,
	"throws": true, "service": true, "enum": true, "const": true,
	"required": true, "optional": true, "oneway": true, "senum": true,
	"slist": true, "void": true, "bool": true, "byte": true, "i8": true,
	"i16": true, "i32": true, "i64": true, "double": true, "string": true,
	"binary": true, "map": true, "list": true, "set": true,

	// Host-language words thrift reserves so the generated code compiles.
	"abstract": true, "alias": true, "and": true, "args": true, "as": true,
	"assert": true, "begin": true, "break": true, "case": true, "catch": true,
	"class": true, "clone": true, "continue": true, "declare": true,
	"def": true, "default": true, "del": true, "delete": true, "do": true,
	"dynamic": true, "elif": true, "else": true, "elseif": true, "elsif": true,
	"end": true, "ensure": true, "except": true, "exec": true, "false": true,
	"finally": true, "float": true, "for": true, "foreach": true, "from": true,
	"function": true, "global": true, "goto": true, "if": true,
	"implements": true, "import": true, "in": true, "inline": true,
	"instanceof": true, "interface": true, "is": true, "lambda": true,
	"module": true, "native": true, "new": true, "next": true, "nil": true,
	"not": true, "or": true, "package": true, "pass": true, "print": true,
	"private": true, "protected": true, "public": true, "raise": true,
	"redo": true, "register": true, "rescue": true, "retry": true,
	"return": true, "self": true, "sizeof": true, "static": true,
	"super": true, "switch": true, "synchronized": true, "then": true,
	"this": true, "throw": true, "transient": true, "true": true, "try": true,
	"undef": true, "unless": true, "unsigned": true, "until": true,
	"use": true, "var": true, "virtual": true, "volatile": true, "when": true,
	"while": true, "with": true, "xor": true, "yield": true,
}

// ident passes a proto name through unchanged unless Thrift claims it, in which
// case it takes a trailing underscore.
//
// The suffix rather than a prefix, and an underscore rather than a word, because
// it is the smallest edit that leaves the original name readable — and it is what
// Cap'n Proto's renderer here already does for the same problem, so a reader who
// has seen one recognizes the other.
func ident(protoName string) string {
	if protoName == "" {
		return "field"
	}
	if reserved[strings.ToLower(protoName)] {
		return protoName + "_"
	}
	return protoName
}

// typeName renders a message or enum's flattened name as a Thrift type name.
func typeName(fullName, pkg string) string {
	out := flatten(fullName, pkg)
	if out == "" {
		return "Type"
	}
	if reserved[strings.ToLower(out)] {
		return out + "_"
	}
	return out
}

// unionName is the type name for the union a oneof becomes.
//
// It is derived from the owning message's flattened name rather than read from
// the IR's UnionName, which is the FlatBuffers spelling and is built from the
// message's *short* name — so two oneofs of the same name under two nested
// messages would collide here, where FlatBuffers puts them in different scopes.
// For a top-level message the two agree, which is what keeps one oneof from
// having two names across two targets.
func unionName(owner, oneofName string) string {
	out := owner + names.Pascal(oneofName)
	if reserved[strings.ToLower(out)] {
		return out + "_"
	}
	return out
}

// flatten joins a nested type's path into one Thrift type name, since Thrift has
// a single flat scope per file.
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

// languageAliases maps the spellings the other targets use onto the ones
// `thrift --gen` takes, so `--lang python` does not have to become `--lang py`
// only for this target.
var languageAliases = map[string]string{
	"c++":        "cpp",
	"c#":         "netstd",
	"csharp":     "netstd",
	"erlang":     "erl",
	"haskell":    "hs",
	"javascript": "js",
	"python":     "py",
	"ruby":       "rb",
	"rust":       "rs",
	"smalltalk":  "st",
}

// normalizeLang resolves a language alias to the generator name thrift knows.
func normalizeLang(lang string) string {
	if got, ok := languageAliases[lang]; ok {
		return got
	}
	return lang
}
