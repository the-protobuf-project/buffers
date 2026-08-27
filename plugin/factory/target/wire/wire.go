// Package wire renders build wiring for Square Wire, the JVM protobuf
// implementation.
//
// # Why this target emits no schema
//
// Every other target here converts: proto in, a different IDL out, with a
// mapping to define and lose information across. Wire is not a different IDL. It
// consumes .proto directly and generates Kotlin, Java and Swift from it, so the
// schema this target would emit is the schema the user already has, and copying
// it into an output directory would only create a second copy to drift.
//
// What Wire actually needs from a build, and what nothing in a .proto supplies,
// is the wiring: which types are roots for tree shaking, what RPC role the
// generated code plays, and where the sources are. Wire's tree shaking is the
// interesting half — a JVM app that links a large proto tree pays for every
// message it never touches, and Wire prunes what is unreachable from the declared
// roots. Choosing those roots by hand is tedious and gets stale.
//
// AIP makes the choice mechanical. The roots of an API are its services and its
// resources: everything a caller can reach is reachable from a method, and
// everything a resource carries is reachable from the resource. That is derivable
// from google.api.resource and the service declarations, which is exactly what
// this target does.
//
// # What it emits
//
//	wire.gradle.kts   the `wire { }` block, roots derived from AIP
//	Topics.kt         topic constants for server-streaming methods, when any exist
package wire

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"
	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/coreir"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// Target renders Square Wire build wiring.
type Target struct {
	// sink receives each finished file, relative to the output directory.
	sink emit.Sink
	// info is the provenance stamped into every banner.
	info provenance.Info
}

// New returns a Wire target writing through the given sink.
func New(sink emit.Sink, info provenance.Info) *Target {
	info.Target = "wire"
	return &Target{sink: sink, info: info}
}

// Name identifies the target in the registry and in `target=` options.
func (t *Target) Name() string { return "wire" }

// Languages lists what the Wire compiler generates from the rooted schema.
func (t *Target) Languages() []string { return []string{schemaOnly, "kotlin", "java", "swift"} }

// schemaOnly is the language every target supports: emit the IDL, compile
// nothing.
const schemaOnly = "schema"

// defaultLanguage is what the Gradle block is written for when the caller did not
// choose. Kotlin is Wire's primary backend and the one its RPC support targets.
const defaultLanguage = "kotlin"

// Generate renders the build wiring.
func (t *Target) Generate(_ factory.Ctx, m *coreir.Model, lang string) error {
	if lang == "" || lang == schemaOnly {
		lang = defaultLanguage
	}
	if !t.supports(lang) {
		return fmt.Errorf("wire cannot emit %q; supported: %s", lang, strings.Join(t.Languages()[1:], ", "))
	}

	r := &run{Target: t, schema: m.Schema, lang: lang}
	if err := r.gradle(); err != nil {
		return err
	}
	if err := r.topics(); err != nil {
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

// run is one Generate call's state.
type run struct {
	*Target
	// schema is the graph being rendered.
	schema *buffers.Schema
	// lang is the language the build block is written for. Unlike the other
	// targets, this one's output *is* build configuration, so a Kotlin run and a
	// Java run genuinely differ.
	lang string
	// diags accumulates problems found while rendering.
	diags []buffers.Diagnostic
}
