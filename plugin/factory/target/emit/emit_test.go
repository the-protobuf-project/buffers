package emit

import (
	"strings"
	"testing"
)

// emit_test.go covers the text builder every renderer writes through.
//
// Indentation is worth pinning because it is invisible in review: a golden diff
// where a block shifted by two spaces looks the same as one where a field moved,
// and the .capnp and .fbs grammars do not care about whitespace, so a target can
// emit badly indented output that still compiles and still fails review.
func TestBlockIndentsAndCloses(t *testing.T) {
	var b Buf
	b.Block("struct Sensor {", "}", func() {
		b.Line("name @0 :Text;")
		b.Block("group mount {", "}", func() {
			b.Line("x @1 :Float64;")
		})
	})

	want := strings.Join([]string{
		"struct Sensor {",
		"  name @0 :Text;",
		"  group mount {",
		"    x @1 :Float64;",
		"  }",
		"}",
		"",
	}, "\n")
	if got := b.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnbalancedOutIsClampedRatherThanPanicking(t *testing.T) {
	// An unbalanced In/Out pair is a rendering bug worth finding in a golden diff,
	// not one worth taking a protoc plugin down over — a panic mid-run leaves buf
	// reporting a crashed plugin rather than a schema problem.
	var b Buf
	b.Out()
	b.Out()
	b.Line("top")

	if got := b.String(); got != "top\n" {
		t.Errorf("got %q, want %q — depth should clamp at zero", got, "top\n")
	}
}

func TestEmptyLineCarriesNoTrailingWhitespace(t *testing.T) {
	// A blank line rendered at depth would be two spaces of trailing whitespace,
	// which git flags and no schema formatter would.
	var b Buf
	b.In()
	b.Line("a")
	b.Line("")
	b.Line("b")

	for i, line := range strings.Split(b.String(), "\n") {
		if line != strings.TrimRight(line, " \t") {
			t.Errorf("line %d %q has trailing whitespace", i, line)
		}
	}
}

func TestDocSkipsEmptyDescriptions(t *testing.T) {
	// An empty doc must write nothing at all — not an empty comment line, which
	// would appear above every undocumented field in every emitted schema.
	var b Buf
	b.Doc("#", "")
	b.Doc("#", "   \n  ")
	if got := b.String(); got != "" {
		t.Errorf("got %q, want nothing", got)
	}

	b = Buf{}
	b.Doc("#", "first\n\nthird")
	want := "# first\n#\n# third\n"
	if got := b.String(); got != want {
		t.Errorf("got %q, want %q — a blank doc line keeps the marker and drops the space", got, want)
	}
}

func TestBytesEndsInExactlyOneNewline(t *testing.T) {
	// Normalizing here rather than trusting each renderer is what keeps "no
	// newline at end of file" out of every golden diff.
	for _, tc := range []struct{ name, in string }{
		{"none", "a"},
		{"one", "a\n"},
		{"several", "a\n\n\n"},
	} {
		var b Buf
		b.Raw(tc.in)
		if got := string(b.Bytes()); got != "a\n" {
			t.Errorf("%s: Bytes() = %q, want %q", tc.name, got, "a\n")
		}
	}
}

func TestCustomIndentUnit(t *testing.T) {
	b := Buf{Unit: "\t"}
	b.Block("{", "}", func() { b.Line("x") })

	if got := b.String(); got != "{\n\tx\n}\n" {
		t.Errorf("got %q, want tab-indented", got)
	}
}
