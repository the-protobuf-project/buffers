// Package flatbuffers renders the message graph as FlatBuffers schema (.fbs).
//
// # What the target decides
//
// Three things in FlatBuffers have no proto equivalent, and each is decided here
// rather than left to flatc:
//
// Slots. Every table field carries an explicit `id:` derived from the neutral
// ordinal, so the vtable layout is a property of the schema rather than of the
// order someone happened to write the fields in. plan.go owns the derivation,
// including the two-slot pair a union consumes.
//
// Layout. A message is a `table` unless it asked to be a `struct`, and a struct
// that cannot be packed is an error rather than a silent downgrade. bufir's
// layout pass owns the eligibility rules.
//
// Substitution. Maps, oneofs and the google.protobuf well-known types do not
// exist in FlatBuffers and are replaced with the idiomatic equivalents: a vector
// of keyed entry tables, a union with a wrapper table per scalar arm, and the
// prelude records in prelude.go.
package flatbuffers

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/coreir"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// Target renders FlatBuffers schema. It is stateless; one Generate call builds a
// run to hold what that call accumulates.
type Target struct {
	// sink receives each finished file, relative to the output directory.
	sink emit.Sink
	// info is the provenance stamped into every banner.
	info provenance.Info
}

// New returns a FlatBuffers target writing through the given sink.
func New(sink emit.Sink, info provenance.Info) *Target {
	info.Target = "flatbuffers"
	return &Target{sink: sink, info: info}
}

// Name identifies the target in the registry and in `target=` options.
func (t *Target) Name() string { return "flatbuffers" }

// Languages lists what flatc can compile the emitted schema into.
//
// The plugin itself always renders schema and nothing else — a protoc plugin has
// no business shelling out to another compiler mid-run. This list is what the
// `buffers` CLI validates a --lang against before invoking flatc, and what a
// `buffers targets` listing prints.
//
// This is the broadest coverage of the four targets, and the reason to reach for
// it when a project needs a language the others do not have: it is the only one
// here that produces Swift and Dart. Every entry was verified by running flatc
// against this repository's own emitted schema rather than copied from its
// documentation.
func (t *Target) Languages() []string {
	return []string{
		// schemaOnly is the language every target supports: emit the IDL, compile
		// nothing.
		schemaOnly,
		"cpp", "csharp", "dart", "go", "java", "kotlin", "kotlin-kmp",
		"lobster", "lua", "nim", "php", "python", "rust", "swift", "ts",
	}
}

// schemaOnly is the language every target supports: emit the IDL, compile
// nothing.
const schemaOnly = "schema"

// Generate renders one .fbs per generate-flagged proto file, plus the prelude
// when anything reached for a substituted well-known type.
func (t *Target) Generate(_ factory.Ctx, m *coreir.Model, lang string) error {
	if lang != "" && lang != schemaOnly && !t.supports(lang) {
		return fmt.Errorf("flatbuffers cannot emit %q; flatc supports: %s",
			lang, strings.Join(t.Languages()[1:], ", "))
	}

	r := &run{Target: t, schema: m.Schema, needed: map[preludeType]bool{}}
	for _, file := range m.Schema.Files {
		body, err := r.file(file)
		if err != nil {
			return err
		}
		if body == nil {
			continue
		}
		if err := t.sink(fbsPath(file.Path), body); err != nil {
			return err
		}
	}
	if body := r.prelude(); body != nil {
		if err := t.sink(preludePath, body); err != nil {
			return err
		}
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
