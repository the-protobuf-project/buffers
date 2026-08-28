package thrift

// run.go holds one Generate call's mutable state, and renders one file.
//
// The state is per-run rather than on the Target so a Target can be shared:
// nothing it holds survives a Generate, and two concurrent renders would
// otherwise interleave their include tables and their prelude bookkeeping.

import (
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// run is one Generate call's mutable state.
type run struct {
	*Target
	// schema is the graph being rendered.
	schema *buffers.Schema

	// needed tracks the substituted well-known records the whole run reached for,
	// which decides what the prelude contains.
	needed map[preludeType]bool
	// fileNeeds tracks them for the file currently rendering, which decides
	// whether that file includes the prelude at all. The body is rendered before
	// the header for exactly this reason.
	fileNeeds map[preludeType]bool

	// includes maps a proto path to the prefix Thrift will reference its types
	// through, which is the included file's base name. See includes.go.
	includes map[string]string

	// diags accumulates problems found while projecting types.
	diags []buffers.Diagnostic
}

// file renders one .thrift, or nil when the file has nothing this target emits.
//
// Declarations come out in the order the proto declares them. That is worth
// stating because the obvious worry — that Thrift resolves a type name where it
// is used, so a struct naming something declared further down would fail — is
// not true of the compiler: forward references and mutually recursive structs
// both compile, verified against thrift's cpp, go, java, py, rb and rs backends
// by TestThriftAcceptsTheSchema. So there is no ordering to derive, and imposing
// one would only reorder the author's file for no reason.
func (r *run) file(f *buffers.File) ([]byte, error) {
	msgs := flattenMessages(f)
	enums := flattenEnums(f)
	svcs := emittableServices(f)
	if len(msgs) == 0 && len(enums) == 0 && len(svcs) == 0 {
		return nil, nil
	}

	r.fileNeeds = map[preludeType]bool{}
	r.includes = r.assignIncludes(f)
	r.reportCollisions(f)

	var b emit.Buf
	for _, e := range enums {
		b.Line("")
		r.enum(&b, e)
	}
	for _, m := range msgs {
		// A oneof becomes a named union type, which Thrift declares at file scope
		// rather than inside the struct. It is emitted immediately before its
		// owner because that is where a reader looks for it, not because Thrift
		// requires it.
		for _, one := range liveOneofs(m) {
			b.Line("")
			r.union(&b, f, m, one)
		}
		b.Line("")
		r.structOf(&b, f, m)
	}
	for _, s := range svcs {
		b.Line("")
		r.service(&b, f, s)
	}

	// The header is assembled last: which files this one includes depends on
	// which substitutions the body reached for.
	var head emit.Buf
	head.Raw(r.banner(f.Path))
	head.Line("")
	r.namespaces(&head, f)
	if lines := r.includeLines(); len(lines) > 0 {
		head.Line("")
		for _, line := range lines {
			head.Line(line)
		}
	}
	head.Raw(b.String())
	return head.Bytes(), nil
}

// namespaces renders the per-language package declarations.
//
// `namespace *` sets the default for every generator at once, which is what the
// proto package already means, and the two overrides are the cases where a proto
// says something more specific. Emitting one line per Thrift language instead
// would be twenty lines of the same string, and would silently omit whichever
// generator was not on the list when someone reached for it.
func (r *run) namespaces(b *emit.Buf, f *buffers.File) {
	def := f.Namespace
	if def == "" {
		def = f.Package
	}
	if def != "" {
		b.Linef("namespace * %s", def)
	}
	if f.JVMPackage != "" {
		b.Linef("namespace java %s", f.JVMPackage)
	}
	if f.GoPackage != "" {
		b.Linef("namespace go %s", f.GoPackage)
	}
}

// doc renders a declaration's documentation as a Thrift doc comment.
//
// The `/** */` form is used rather than the `#` the banner takes, because it is
// the one the Thrift compiler attaches to a declaration and copies into the
// generated code. A proto author's comment therefore reaches the generated Java
// or Python docstring, which a `#` comment would not.
//
// Notes are folded into the same block rather than emitted above it, so a caveat
// about a field travels with the field into whatever language consumes it. They
// are also the only part that gets wrapped: a note arrives as one long sentence
// composed here, while the prose above it was wrapped by the proto author the way
// they wanted it, and reflowing that would rewrite their paragraphs to say the
// same words differently.
func (r *run) doc(b *emit.Buf, prose string, notes ...string) {
	lines := append([]string(nil), splitLines(prose)...)
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, note := range notes {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, wrap(note, docWidth)...)
	}
	if len(lines) == 0 {
		return
	}

	b.Line("/**")
	for _, line := range lines {
		if line == "" {
			b.Line(" *")
			continue
		}
		b.Linef(" * %s", closeSafe(line))
	}
	b.Line(" */")
}

// closeSafe defuses a `*/` inside prose, which would otherwise end the doc
// comment early and leave the rest of the sentence as stray tokens.
//
// It is a real input, not a theoretical one: a proto comment quoting a C or Java
// block comment carries the sequence verbatim, and the resulting .thrift fails to
// parse somewhere after the comment rather than in it. A space is inserted rather
// than the text escaped because Thrift's lexer has no escape inside a comment —
// there is nothing to escape it with.
func closeSafe(s string) string { return strings.ReplaceAll(s, "*/", "* /") }

// docWidth is the column a note is wrapped at, chosen so that ` * `, the text and
// a struct's two-space indent land under eighty.
const docWidth = 74

// wrap breaks a line greedily at spaces, and returns it untouched when it already
// fits.
func wrap(line string, width int) []string {
	if len(line) <= width {
		return []string{line}
	}
	var out []string
	var cur string
	for _, word := range strings.Fields(line) {
		switch {
		case cur == "":
			cur = word
		case len(cur)+1+len(word) <= width:
			cur += " " + word
		default:
			out = append(out, cur)
			cur = word
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
