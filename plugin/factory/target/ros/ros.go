// Package ros renders the message graph as ROS 2 interface definitions: .msg for
// messages, .srv for calls, and a topic manifest for publications.
//
// # What ROS cannot hold, and what is done about it
//
// This is the least expressive target, and the design decisions are mostly about
// loss. ROS has no enums, no unions, no maps, no optionals, and no nested type
// declarations — one message per file, named after the file.
//
// Enums become their own message carrying typed constants and a value field,
// which is what rosidl users write by hand for a shared enum. Inlining the
// constants into each using message would be more idiomatic for a single use and
// collides the moment two fields share an enum.
//
// Oneofs become a discriminant constant block plus every arm as an ordinary
// field. That is genuinely lossy — nothing stops a writer setting two arms — so
// it is reported rather than done quietly.
//
// Maps become an array of two-field entry messages. Unlike the FlatBuffers
// substitution there is no keyed-lookup convention to preserve, so the lookup is
// the reader's problem.
//
// What ROS has that no other target here does is a bound as part of the type.
// `float64[<=64]` and `float64[]` are different types, and in a real-time system
// that is the difference that matters — which is what
// (buffers.v1.field).max_len exists for.
package ros

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/coreir"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// Target renders ROS 2 interface definitions.
type Target struct {
	// sink receives each finished file, relative to the output directory.
	sink emit.Sink
	// info is the provenance stamped into every banner.
	info provenance.Info
}

// New returns a ROS target writing through the given sink.
func New(sink emit.Sink, info provenance.Info) *Target {
	info.Target = "ros"
	return &Target{sink: sink, info: info}
}

// Name identifies the target in the registry and in `target=` options.
func (t *Target) Name() string { return "ros" }

// Languages lists what can be generated from the emitted definitions.
//
// Unlike the other targets this is not a compiler flag: rosidl generates C, C++
// and Python from a .msg as part of a colcon build, driven by the package's
// CMakeLists rather than by a one-shot command. The list is what
// `buffers targets` reports; the CLI does not invoke rosidl.
func (t *Target) Languages() []string { return []string{schemaOnly, "c", "cpp", "python"} }

// schemaOnly is the language every target supports: emit the IDL, compile
// nothing.
const schemaOnly = "schema"

// Generate renders the ROS interface tree.
func (t *Target) Generate(_ factory.Ctx, m *coreir.Model, lang string) error {
	if lang != "" && lang != schemaOnly && !t.supports(lang) {
		return fmt.Errorf("ros cannot emit %q; rosidl produces: %s", lang, strings.Join(t.Languages()[1:], ", "))
	}

	r := &run{Target: t, schema: m.Schema}
	for _, file := range m.Schema.Files {
		if err := r.file(file); err != nil {
			return err
		}
	}
	if err := r.manifest(m.Schema); err != nil {
		return err
	}

	m.Schema.Diags = append(m.Schema.Diags, r.diags...)
	return nil
}

// supports reports whether a language is one this target can emit.
func (t *Target) supports(lang string) bool {
	for _, got := range t.Languages() {
		if got == lang {
			return true
		}
	}
	return false
}
