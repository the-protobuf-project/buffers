package bufir

// schema.go is the root of the graph: one build's whole result, and the per-file
// names each target needs.
type Schema struct {
	// Files are the generate-flagged files, in the order protoc supplied them.
	// A file that is only imported contributes its types to the indexes below but
	// does not appear here: emitting a schema for a dependency would put someone
	// else's types in this module's output directory.
	Files []*File

	// Messages indexes every message the build has seen, generated or imported,
	// by full proto name. A field's Message name is resolved through here.
	Messages map[NodeID]*Message

	// Enums indexes every enum the same way.
	Enums map[NodeID]*Enum

	// Lock is the ordinal ledger this build read, and the one it will write.
	// Never nil after a build: an absent buffers.lock yields an empty ledger, not
	// a missing one, so the write path has somewhere to record a first run.
	Lock *Lock

	// Diags collects recoverable problems. Severity is resolved against the
	// plugin's strictness setting, matching protokit.Options.Strict.
	Diags []Diagnostic
}

// Message returns the message with the given full proto name, or nil.
func (s *Schema) Message(name string) *Message { return s.Messages[NodeID(name)] }

// Enum returns the enum with the given full proto name, or nil.
func (s *Schema) Enum(name string) *Enum { return s.Enums[NodeID(name)] }

// File is one .proto file and the per-target names it carries.
type File struct {
	Path    string // "sensors/v1/sensors.proto"
	Package string // "sensors.v1"
	// Doc is the file's leading comment, as prose.
	Doc string

	// Namespace is the FlatBuffers namespace and the Cap'n Proto C++ namespace.
	// Defaults to Package.
	Namespace string

	// ROSPackage is the ROS 2 package the .msg/.srv files belong to. Defaults to
	// the proto package flattened with underscores.
	ROSPackage string

	// JVMPackage is the Kotlin/Java package Wire generates into. Defaults to the
	// file's java_package, then to Package.
	JVMPackage string

	// GoImport is the Go import path from the file's own `option go_package`,
	// with any ";alias" suffix removed. Empty when the file declares none.
	//
	// It is read rather than re-declared because capnpc-go's $Go.import means
	// exactly what go_package already means, and asking an author to state the
	// same import path twice is how the two end up disagreeing.
	GoImport string

	// GoPackage is the Go package name: the go_package alias when one is given,
	// otherwise the last path segment.
	GoPackage string

	// CapnpID is the file's Cap'n Proto 64-bit ID, either declared or derived.
	// See capnpid.go.
	CapnpID uint64

	// Identifier is the FlatBuffers file_identifier: 4 ASCII characters or empty.
	Identifier string

	// Extension is the FlatBuffers file_extension, without a dot.
	Extension string

	// Includes are extra FlatBuffers include lines beyond those derived from
	// Imports.
	Includes []string

	// Generate reports whether protoc asked for this file's output.
	//
	// It gates slots and diagnostics, not indexing. An imported file's types are
	// still indexed — a generated message's field may name one, and resolving that
	// reference is how a target decides whether to qualify it or substitute it —
	// but they get no ordinals, no ledger entries and no diagnostics, because this
	// run does not emit them and the module that does owns those decisions.
	Generate bool

	// Imports are the proto paths this file imports that actually contribute a
	// referenced type. An import whose types are unused is dropped, because an
	// unused `include` in a .fbs is a compile-time dependency the consumer did not
	// need to take.
	Imports []string

	Messages []*Message // top-level only; nested reach through Message.Nested
	Enums    []*Enum    // top-level only
	// Services are the services this file declares.
	Services []*Service
}
