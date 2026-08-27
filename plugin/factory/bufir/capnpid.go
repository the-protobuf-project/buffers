package bufir

import (
	"crypto/sha256"
	"encoding/binary"
)

// capnpid.go derives the 64-bit identifiers Cap'n Proto puts on files and types.
//
// # Why derive rather than let capnp assign
//
// `capnp id` mints a random ID and the compiler assigns one to any declaration
// that lacks one. Both are wrong for generated output. A random ID means
// regenerating an unchanged .proto produces a .capnp that differs from the last
// one, which defeats every golden test and makes every regeneration a diff. A
// compiler-assigned ID means the ID depends on the compiler's derivation rule,
// which this plugin would then have to match exactly or silently disagree with.
//
// So every emitted declaration carries an explicit ID derived here, and capnp is
// never asked to invent one.
//
// # What the derivation is, and what it is not
//
// It is SHA-256 over a stable name, truncated to 64 bits, with the high bit set
// because Cap'n Proto requires IDs to be at least 2^63.
//
// It is deliberately *not* Cap'n Proto's own parent-scope derivation. Matching
// that would mean reimplementing an MD5-over-hex-of-parent-id rule and staying
// bug-for-bug compatible with it forever, to gain nothing: the IDs are written
// out explicitly, so the compiler never derives anything and never has an opinion
// to disagree with.
//
// # What each ID is derived from, and why it matters
//
// A file's ID comes from its proto path. A type's ID comes from its fully
// qualified proto name — not from its file's ID — so that moving a message
// between two files in the same package leaves its identity alone. That is the
// right call because a Cap'n Proto type ID is what a consumer's generated code
// carries: it survives a schema being reorganized, and it should.
//
// The consequence to know about is the other direction: renaming a message, or
// renaming the file a schema lives in, changes the derived ID. Both are genuinely
// breaking in Cap'n Proto terms, so surfacing them is right — but when a rename
// is meant to be transparent, (buffers.v1.file).capnp_id and
// (buffers.v1.message).capnp_id pin the old value.

// capnpIDBit is the high bit Cap'n Proto requires every ID to carry.
const capnpIDBit uint64 = 1 << 63

// deriveCapnpID hashes a stable name into a valid Cap'n Proto ID.
//
// The domain prefix keeps the two namespaces apart: without it a file at path
// "sensors.v1" and a type named "sensors.v1" would collide, which is unlikely and
// free to prevent.
func deriveCapnpID(domain, name string) uint64 {
	sum := sha256.Sum256([]byte(domain + "\x00" + name))
	return binary.BigEndian.Uint64(sum[:8]) | capnpIDBit
}

// fileCapnpID derives a file's ID from its proto path.
func fileCapnpID(protoPath string) uint64 { return deriveCapnpID("file", protoPath) }

// typeCapnpID derives a type's ID from its fully qualified proto name. Used for
// messages, enums and interfaces alike, since Cap'n Proto draws them from one
// space.
func typeCapnpID(fullName string) uint64 { return deriveCapnpID("type", fullName) }

// DeriveFileID and DeriveTypeID expose the derivation to the Cap'n Proto target.
//
// Nearly every ID it needs is already on the IR node — File.CapnpID,
// Message.CapnpID, Service.CapnpID — because a declared override has to be
// honoured and only the build sees the options. The exception is the prelude,
// whose types have no proto declaration and therefore no node, and which still
// has to get its IDs from the same function as everything else. Two derivations
// that agree today is not a property worth relying on.
func DeriveFileID(protoPath string) uint64 { return fileCapnpID(protoPath) }

// DeriveTypeID derives a type ID from a fully qualified name. See DeriveFileID.
func DeriveTypeID(fullName string) uint64 { return typeCapnpID(fullName) }

// resolveCapnpID applies a declared override, treating 0 as unset.
//
// The declared value arrives as an int64 because AIP-141 forbids uint64 in an API
// surface, and a Cap'n Proto ID always has its high bit set — so every valid one
// reads as a negative int64. That is a bit pattern, not a quantity, and the
// conversion here is the only place that has to know it.
func resolveCapnpID(declared int64, derive func() uint64) uint64 {
	if declared == 0 {
		return derive()
	}
	return uint64(declared) | capnpIDBit
}
