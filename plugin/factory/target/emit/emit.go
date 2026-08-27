// Package emit holds what all four renderers do the same way: build indented
// text, and hand finished files back to protoc.
//
// It is small on purpose. Four targets writing four flavours of strings.Builder
// plumbing is how their output drifts apart in whitespace, and a golden diff full
// of indentation changes hides the one line that actually moved.
package emit

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

// Sink accepts a finished file, relative to the plugin's output directory.
//
// It is a function rather than the *protogen.Plugin itself so that the CLI can
// render to disk without constructing a CodeGeneratorResponse, and so that a test
// can render into a map. Neither of those should have to fake a protoc run.
type Sink func(path string, content []byte) error

// Through returns a Sink that writes through protoc's response, which is how a
// buf or protoc invocation gets its files.
func Through(p *protogen.Plugin) Sink {
	return func(path string, content []byte) error {
		_, err := p.NewGeneratedFile(path, "").Write(content)
		return err
	}
}

// Buf accumulates indented text.
//
// The zero value is ready to use, and Indent is a count of levels rather than of
// spaces so a caller never has to know a target's indent width.
type Buf struct {
	// b accumulates the rendered text.
	b strings.Builder
	// depth is the current indent level, in units of Unit.
	depth int
	// Unit is one indent level. Empty means two spaces, which is what every
	// target here uses; Cap'n Proto and FlatBuffers both conventionally indent
	// by two.
	Unit string
}

// Line writes one line at the current depth. An empty string writes a bare
// newline rather than a line of trailing whitespace, which git would flag and no
// formatter would.
func (b *Buf) Line(s string) {
	if s == "" {
		b.b.WriteByte('\n')
		return
	}
	b.b.WriteString(b.prefix())
	b.b.WriteString(s)
	b.b.WriteByte('\n')
}

// Linef is Line with formatting.
func (b *Buf) Linef(format string, args ...any) { b.Line(fmt.Sprintf(format, args...)) }

// Raw writes text with no indent and no trailing newline, for a banner that
// already carries its own.
func (b *Buf) Raw(s string) { b.b.WriteString(s) }

// In increases the indent depth, Out decreases it. Out below zero is clamped
// rather than panicking: an unbalanced pair is a rendering bug worth finding in
// the golden diff, not one worth taking the plugin down over.
func (b *Buf) In() { b.depth++ }

// Out decreases the indent depth.
func (b *Buf) Out() {
	if b.depth > 0 {
		b.depth--
	}
}

// Block writes an opening line, runs body one level deeper, then writes the
// closing line. It is the shape of nearly every construct in every target here,
// and it makes an unbalanced indent impossible rather than merely unlikely.
func (b *Buf) Block(open, close string, body func()) {
	b.Line(open)
	b.In()
	body()
	b.Out()
	b.Line(close)
}

// Doc writes a doc comment, one line per source line, using the given marker.
// An empty description writes nothing at all — not an empty comment.
func (b *Buf) Doc(marker, doc string) {
	if strings.TrimSpace(doc) == "" {
		return
	}
	for _, line := range strings.Split(doc, "\n") {
		if line == "" {
			b.Line(marker)
			continue
		}
		b.Linef("%s %s", marker, line)
	}
}

// String returns the accumulated text.
func (b *Buf) String() string { return b.b.String() }

// Bytes returns the accumulated text, ending in exactly one newline.
//
// Normalizing here rather than trusting each renderer to end tidily is what keeps
// "no newline at end of file" out of the golden diffs.
func (b *Buf) Bytes() []byte {
	return []byte(strings.TrimRight(b.b.String(), "\n") + "\n")
}

// prefix renders the indent for the current depth.
func (b *Buf) prefix() string {
	unit := b.Unit
	if unit == "" {
		unit = "  "
	}
	return strings.Repeat(unit, b.depth)
}
