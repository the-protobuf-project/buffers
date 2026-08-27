package capnp

import (
	"fmt"
	"sort"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// prelude.go supplies the records Cap'n Proto does not have and proto assumes.
//
// Same reasoning as the FlatBuffers prelude: google.protobuf.Timestamp and
// friends are declared in files this module does not own, so they are substituted
// with records in a namespace this plugin owns rather than passed through.
//
// Fewer of them are needed here than in FlatBuffers, because Cap'n Proto already
// has the two that matter most: Void is exactly google.protobuf.Empty, and
// List(Text) is exactly a FieldMask.

// preludePath is where the prelude is written, relative to the output directory.
const preludePath = "buffers/wellknown.capnp"

// preludeGoImport is the Go import path stamped into the prelude's $Go.import.
//
// It is a placeholder in the sense that no such package is published — the
// generated code lands wherever the consumer's `capnp compile -o go:<dir>` put
// it. capnpc-go uses the value only to decide whether two types share a package,
// and every prelude type is in one file, so any stable string does that job. A
// stable one is still required: capnpc-go emits imports keyed on it.
const preludeGoImport = "buffers/wellknown"

// preludeAlias is the `using` name importing files bind the prelude to. It is
// fixed rather than derived so that it can be referenced while rendering a body,
// before the header that declares it has been assembled.
const preludeAlias = "Wellknown"

// preludeType names one substituted well-known record.
type preludeType uint8

const (
	// preludeTimestamp substitutes google.protobuf.Timestamp.
	preludeTimestamp preludeType = iota
	// preludeDuration substitutes google.protobuf.Duration.
	preludeDuration
	// preludeAny substitutes google.protobuf.Any.
	preludeAny
)

// needPrelude records that a rendering reached for a substituted record, so the
// prelude carries exactly what is used and the importing file includes it.
func (r *run) needPrelude(p preludeType) {
	r.needed[p] = true
	r.fileNeeds[p] = true
}

// preludeRef renders a reference to a prelude type from inside a file body.
func (r *run) preludeRef(name string) string { return preludeAlias + "." + name }

// preludeBodies are the record definitions, keyed by type. Field layouts match
// the proto definitions exactly, so a payload converted between the two carries
// the same numbers.
var preludeBodies = map[preludeType]string{
	preludeTimestamp: `# Timestamp is google.protobuf.Timestamp: seconds since the Unix epoch plus a
# nanosecond adjustment.
struct Timestamp @0x%016x {
  seconds @0 :Int64;
  nanos @1 :Int32;
}`,
	preludeDuration: `# Duration is google.protobuf.Duration: a signed span.
struct Duration @0x%016x {
  seconds @0 :Int64;
  nanos @1 :Int32;
}`,
	preludeAny: `# Any is google.protobuf.Any: a type URL and an opaque payload. The payload stays
# opaque — resolving it needs a descriptor pool, which a Cap'n Proto reader does
# not have.
struct Any @0x%016x {
  typeUrl @0 :Text;
  value @1 :Data;
}`,
}

// preludeNames maps each record to the name its ID is derived from, so the
// prelude's type IDs are as stable as every other type's.
var preludeNames = map[preludeType]string{
	preludeTimestamp: "buffers.wellknown.Timestamp",
	preludeDuration:  "buffers.wellknown.Duration",
	preludeAny:       "buffers.wellknown.Any",
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
	b.Linef("@0x%016x;", bufir.DeriveFileID(preludePath))
	b.Line("")
	b.Raw(r.banner(preludePath))
	b.Line("")
	b.Line(`using Cxx = import "/capnp/c++.capnp";`)
	b.Linef("$Cxx.namespace(%q);", "buffers::wellknown")

	// The prelude needs the same annotations every other file does: capnpc-go
	// rejects *any* file without a package annotation, and a schema whose
	// substituted Timestamp fails to generate is no more usable than one whose
	// own types do. It is synthetic, so the values are this package's rather than
	// read from a .proto.
	r.annotationHeader(&b, &bufir.File{
		Path:       preludePath,
		Package:    "buffers.wellknown",
		GoPackage:  "wellknown",
		GoImport:   preludeGoImport,
		JVMPackage: "com.buffers.wellknown",
	})

	for _, k := range kinds {
		p := preludeType(k)
		b.Line("")
		body := fmt.Sprintf(preludeBodies[p], bufir.DeriveTypeID(preludeNames[p]))
		for _, line := range splitLines(body) {
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
