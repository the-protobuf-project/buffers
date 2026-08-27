// Package capnp renders the message graph as Cap'n Proto schema (.capnp),
// including RPC interfaces derived from the protos' services.
//
// # What the target decides
//
// Identifiers. Cap'n Proto's grammar requires a lowercase initial on members and
// an uppercase one on types, so every proto snake_case name is rewritten. That is
// a parse requirement, not a style choice — see names.go.
//
// Identity. Every file, struct, enum and interface carries an explicit 64-bit ID
// so that regenerating an unchanged .proto produces a byte-identical .capnp.
// Leaving them out would make capnp mint them, and its derivation is not this
// plugin's to depend on. See bufir/capnpid.go.
//
// Ordinals. Cap'n Proto requires a struct's ordinals to run 0..N-1 with no gaps
// and no repeats, counting union members. bufir's assignment already produces
// exactly that; this target verifies it before emitting, because a violation is a
// capnp parse error and the message naming the field is more useful than the one
// naming the line.
//
// Streaming. Cap'n Proto RPC has no server-streaming method. The idiom is a
// capability the caller passes in and the callee pushes to, so a server-streaming
// proto method becomes a call taking a generated sink interface.
package capnp

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/coreir"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// Target renders Cap'n Proto schema.
type Target struct {
	// sink receives each finished file, relative to the output directory.
	sink emit.Sink
	// info is the provenance stamped into every banner.
	info provenance.Info

	// annotations are the languages whose generators need file-level annotation
	// blocks emitted. See annotations.go for why this is a per-run choice rather
	// than something always emitted.
	annotations []string

	// goModule is the Go module the generated code will live in, needed for
	// capnpc-go's $Go.import. Empty omits the import annotation.
	goModule string
}

// New returns a Cap'n Proto target writing through the given sink.
func New(sink emit.Sink, info provenance.Info, goModule string, annotations ...string) *Target {
	info.Target = "capnp"
	return &Target{sink: sink, info: info, goModule: goModule, annotations: annotations}
}

// Name identifies the target in the registry and in `target=` options.
func (t *Target) Name() string { return "capnp" }

// Languages lists what `capnp compile` can produce from the emitted schema.
//
// The plugin renders schema only; this is what the `buffers` CLI validates a
// --lang against, and what `buffers targets` prints.
//
// Only c++ ships with Cap'n Proto. Every other entry is a separate capnpc-<lang>
// binary the user installs, so appearing here is a claim that the emitted schema
// is *acceptable to* that generator — which for Go and Java means carrying their
// annotation blocks; see annotations.go — not that the generator is present.
// langs.Run reports a missing one with its install line.
//
// Swift and Dart are absent because no Cap'n Proto generator for them exists.
// Both are covered by the FlatBuffers target, which is the honest answer for a
// project that needs those languages.
func (t *Target) Languages() []string {
	return []string{
		// schemaOnly is the language every target supports: emit the IDL, compile
		// nothing.
		schemaOnly,
		"c++", "cpp", "go", "rust", "java", "kotlin", "python", "ts", "csharp",
	}
}

// schemaOnly is the language every target supports: emit the IDL, compile
// nothing.
const schemaOnly = "schema"

// Generate renders one .capnp per generate-flagged proto file.
func (t *Target) Generate(_ factory.Ctx, m *coreir.Model, lang string) error {
	if lang != "" && lang != schemaOnly && !t.supports(lang) {
		return fmt.Errorf("capnp cannot emit %q; supported: %s", lang, strings.Join(t.Languages()[1:], ", "))
	}

	r := &run{
		Target:      t,
		schema:      m.Schema,
		needed:      map[preludeType]bool{},
		annotations: annotationSet(t.annotations, lang),
		goModule:    t.goModule,
	}
	for _, file := range m.Schema.Files {
		body, err := r.file(file)
		if err != nil {
			return err
		}
		if body == nil {
			continue
		}
		if err := t.sink(capnpPath(file.Path), body); err != nil {
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

// annotationSet normalizes the languages a run should emit annotations for.
//
// The requested language is folded in alongside the explicit set, so that a
// caller who only says `--lang go` gets a schema Go can actually compile without
// having to also know that Go needs annotations.
func annotationSet(configured []string, lang string) map[string]bool {
	out := map[string]bool{}
	for _, l := range configured {
		if n := normalizeLang(l); NeedsAnnotations(n) {
			out[n] = true
		}
	}
	if n := normalizeLang(lang); NeedsAnnotations(n) {
		out[n] = true
	}
	return out
}
