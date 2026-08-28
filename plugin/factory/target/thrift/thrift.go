// Package thrift renders the message graph as Apache Thrift IDL (.thrift),
// including a service per proto service.
//
// # Why this is the closest fit of any target here
//
// Every other backend has to invent a mapping for a field's slot, because none of
// them number fields the way proto does. Thrift does: a Thrift field id is a
// 1-based, sparse, permanently-assigned integer, which is exactly what a proto
// field number is. So this target emits the proto field number verbatim and never
// reads the derived ordinal.
//
// That inverts this repository's main warning, and the inversion is the point.
// buffers.lock does not govern a .thrift. Deleting a field without reserving it
// cannot shift anything here, because nothing slides down to fill the gap. The
// ledger still records the ordinals the other targets are rendered from — it just
// has no authority over this one, which is why the banner does not send a reader
// to it.
//
// Identifiers are the same story. Cap'n Proto rewrites every name because its
// grammar requires a case; Thrift's grammar requires none, so `display_name`
// stays `display_name`, `SENSOR_KIND_LIDAR` stays itself, and `GetSensor` stays
// `GetSensor`. The only renaming here is for words the Thrift compiler claims and
// for nesting, which Thrift does not have.
//
// # What it cannot hold
//
// No unsigned integers, so uint32 and uint64 become i32 and i64 and a value above
// the signed maximum reads back negative. No 32-bit float, so `float` widens to
// `double`. No streaming, so a streaming method becomes a unary call. No bounds,
// no nesting, and no dynamically typed JSON. Each of those is reported rather
// than done quietly.
//
// What it has that no other target here does is a real map, so a proto map stays
// a map instead of decaying into a list of pairs.
package thrift

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/coreir"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// Target renders Apache Thrift IDL.
type Target struct {
	// sink receives each finished file, relative to the output directory.
	sink emit.Sink
	// info is the provenance stamped into every banner.
	info provenance.Info
}

// New returns a Thrift target writing through the given sink.
func New(sink emit.Sink, info provenance.Info) *Target {
	info.Target = "thrift"
	return &Target{sink: sink, info: info}
}

// Name identifies the target in the registry and in `target=` options.
func (t *Target) Name() string { return "thrift" }

// Languages lists what `thrift --gen` can produce from the emitted IDL.
//
// Unlike Cap'n Proto's, none of these is a separate install: Apache Thrift ships
// its generators inside the one binary, so having thrift is having all of them.
//
// It is a **superset across versions**, not a description of any one thrift, and
// that is deliberate. The generator set moves between releases: 0.24 generates
// `mmd` and has no `swift`, while the thrift Ubuntu currently packages generates
// `swift` and has no `mmd`. Both are supported versions, so any single list is
// wrong on one of them. This one is the union, and the installed compiler decides
// what a particular run can actually do — see langs/thriftgen.go, which turns a
// generator this thrift lacks into a message naming what it does offer.
//
// The plugin cannot make that check itself, and should not: it never runs a
// subprocess, so it validates against this list and leaves the real answer to the
// CLI. Being permissive is the right failure direction there — refusing a
// language the local thrift supports would be worse than accepting one it does
// not and having thrift say so.
//
// The spellings are the compiler's own — `py`, `rb`, `rs`, `netstd` — because
// they are passed to `--gen` unchanged and a translation table would be one more
// place for a typo to hide. names.go maps the few obvious aliases.
//
// Seven of these emit something other than a language: `gv` is GraphViz, `html`,
// `markdown` and `mmd` are documentation, and `json`, `xml` and `xsd` describe
// the schema. They are listed because thrift will produce them.
func (t *Target) Languages() []string {
	return []string{
		schemaOnly,
		"c_glib", "cl", "cpp", "d", "dart", "delphi", "erl", "go", "gv", "haxe",
		"html", "java", "javame", "js", "json", "kotlin", "lua", "markdown",
		"mmd", "netstd", "ocaml", "perl", "php", "py", "rb", "rs", "st", "swift",
		"xml", "xsd",
	}
}

// schemaOnly is the language every target supports: emit the IDL, compile
// nothing.
const schemaOnly = "schema"

// Generate renders one .thrift per generate-flagged proto file, plus the prelude
// when a substituted well-known type was reached for.
func (t *Target) Generate(_ factory.Ctx, m *coreir.Model, lang string) error {
	if lang != "" && lang != schemaOnly && !t.supports(lang) {
		return fmt.Errorf("thrift cannot emit %q; supported: %s", lang, strings.Join(t.Languages()[1:], ", "))
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
		if err := t.sink(thriftPath(file.Path), body); err != nil {
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
	want := normalizeLang(lang)
	for _, got := range t.Languages() {
		if got == want {
			return true
		}
	}
	return false
}
