package thrift

// prelude.go supplies the records Thrift does not have and proto assumes.
//
// Same reasoning as the FlatBuffers and Cap'n Proto preludes:
// google.protobuf.Timestamp and friends are declared in files this module does
// not own, so emitting a .thrift for them would write Google's types into the
// consumer's output directory, where two modules that both did it would collide.
// They are substituted instead, with records in a namespace this plugin owns.
//
// Fewer are needed here than in FlatBuffers, because Thrift already has two of
// them outright: a FieldMask is a `list<string>`, and a method returning Empty
// returns `void`. Empty is still carried, for the case where it appears as a
// field type rather than as a method's payload.
//
// The field ids match the proto definitions exactly, so a payload converted
// between the two carries the same numbers in the same slots — which for this
// target is not a coincidence to be maintained but the same numbering scheme
// showing through.

import (
	"sort"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// preludePath is where the prelude is written, relative to the output directory.
const preludePath = "buffers/wellknown.thrift"

// preludeNamespace is the package the substituted records live in. It is
// deliberately not `google.protobuf`: these are this plugin's renderings, not
// Google's schema, and naming them as Google's would make a collision with a
// hand-written one look like a duplicate definition of the same thing.
const preludeNamespace = "buffers.wellknown"

// preludePrefix is what an including file writes before a prelude type. Thrift
// scopes an include by its file's base name, so this is fixed by preludePath
// rather than chosen — see includes.go.
const preludePrefix = "wellknown"

// preludeType names one substituted record.
type preludeType uint8

const (
	// preludeTimestamp substitutes google.protobuf.Timestamp.
	preludeTimestamp preludeType = iota
	// preludeDuration substitutes google.protobuf.Duration.
	preludeDuration
	// preludeEmpty substitutes google.protobuf.Empty.
	preludeEmpty
	// preludeAny substitutes google.protobuf.Any.
	preludeAny
)

// needPrelude records that a rendering reached for a substituted record, so the
// prelude carries exactly what is used and the including file names it.
func (r *run) needPrelude(p preludeType) {
	r.needed[p] = true
	r.fileNeeds[p] = true
}

// preludeRef renders a reference to a prelude type from inside a file body.
func (r *run) preludeRef(name string) string { return preludePrefix + "." + name }

// preludeBodies are the record definitions, keyed by type.
var preludeBodies = map[preludeType]string{
	preludeTimestamp: `/**
 * Timestamp is google.protobuf.Timestamp: seconds since the Unix epoch plus a
 * nanosecond adjustment.
 */
struct Timestamp {
  1: i64 seconds
  2: i32 nanos
}`,
	preludeDuration: `/**
 * Duration is google.protobuf.Duration: a signed span.
 */
struct Duration {
  1: i64 seconds
  2: i32 nanos
}`,
	preludeEmpty: `/**
 * Empty is google.protobuf.Empty, for the case where it is a field's type. A
 * method returning it returns Thrift's own void instead, which needs no record.
 */
struct Empty {}`,
	preludeAny: `/**
 * Any is google.protobuf.Any: a type URL and an opaque payload. The payload stays
 * opaque — resolving it needs a descriptor pool, which a Thrift reader does not
 * have.
 */
struct Any {
  1: string type_url
  2: binary value
}`,
}

// prelude renders the shared file, or returns nil when nothing needed it.
func (r *run) prelude() []byte {
	if len(r.needed) == 0 {
		return nil
	}
	kinds := make([]int, 0, len(r.needed))
	for p := range r.needed {
		kinds = append(kinds, int(p))
	}
	sort.Ints(kinds)

	var b emit.Buf
	b.Raw(r.banner(preludePath))
	b.Line("")
	b.Linef("namespace * %s", preludeNamespace)
	b.Linef("namespace java com.%s", preludeNamespace)
	b.Linef("namespace go %s", preludePrefix)

	for _, k := range kinds {
		b.Line("")
		for _, line := range splitLines(preludeBodies[preludeType(k)]) {
			b.Line(line)
		}
	}
	return b.Bytes()
}

// splitLines breaks a literal body into lines for Buf, which indents per line.
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
