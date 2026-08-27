package flatbuffers

import (
	"sort"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// prelude.go supplies the records FlatBuffers does not have and proto assumes.
//
// google.protobuf.Timestamp, Duration, Any and Empty are ordinary messages in the
// descriptor set, so passing them through would have been possible — and wrong.
// They live in files this module does not own, and emitting a .fbs for them would
// write Google's types into the consumer's output directory, where two modules
// that both did it would collide.
//
// Instead they are substituted with records in a namespace this plugin owns,
// emitted once into a shared prelude file, and included by any schema that needs
// one. The field layouts match the proto definitions exactly, so a payload
// converted between the two carries the same numbers.

// preludeNamespace is where the substituted records live. It is deliberately not
// `google.protobuf`: these are this plugin's renderings, not Google's schema, and
// naming them as if they were Google's would make a collision with a hand-written
// google.protobuf.fbs look like a duplicate definition of the same thing.
const preludeNamespace = "buffers.wellknown"

// preludePath is where the prelude is written, relative to the output directory.
const preludePath = "buffers/wellknown.fbs"

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
// prelude is emitted with exactly what is used and no more.
func (t *run) needPrelude(p preludeType) {
	t.needed[p] = true
	t.fileNeeds[p] = true
}

// preludeBodies are the record definitions, keyed by type.
//
// Timestamp and Duration are structs rather than tables: both are two fixed-width
// scalars, both appear in high-rate messages, and a table would cost a vtable
// indirection per read to buy an evolvability that proto has already frozen.
var preludeBodies = map[preludeType]string{
	preludeTimestamp: `// Timestamp is google.protobuf.Timestamp: seconds since the Unix epoch plus a
// nanosecond adjustment. A struct, because the proto definition is frozen and
// both fields are always present.
struct Timestamp {
  seconds:long;
  nanos:int;
}`,
	preludeDuration: `// Duration is google.protobuf.Duration: a signed span. A struct, for the same
// reason Timestamp is.
struct Duration {
  seconds:long;
  nanos:int;
}`,
	preludeEmpty: `// Empty is google.protobuf.Empty. A table rather than a struct because a
// FlatBuffers struct may not be empty, and because a method returning Empty today
// may return something later — a table can grow a field, a struct cannot.
table Empty {}`,
	preludeAny: `// Any is google.protobuf.Any: a type URL and an opaque payload. The payload stays
// opaque here — resolving it needs a descriptor pool, which a FlatBuffers reader
// does not have.
table Any {
  type_url:string;
  value:[ubyte];
}`,
}

// prelude renders the shared file, or returns nil when nothing needed it.
func (t *run) prelude() []byte {
	if len(t.needed) == 0 {
		return nil
	}
	kinds := make([]int, 0, len(t.needed))
	for p := range t.needed {
		kinds = append(kinds, int(p))
	}
	sort.Ints(kinds)

	var b emit.Buf
	b.Raw(t.banner(preludePath))
	b.Line("")
	b.Linef("namespace %s;", preludeNamespace)
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
