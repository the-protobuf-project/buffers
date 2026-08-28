package thrift

import (
	"strings"
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
)

// TestFlattenedNamesCollide covers the cost of Thrift having one flat scope per
// file.
//
// `p.FooBar` and `p.Foo.Bar` are two distinct types in proto, which scopes a
// nested message under its parent. Thrift does not, so both fold onto `FooBar`
// and one would shadow the other. It is reported rather than renamed: picking a
// loser would make a type's name depend on whether an unrelated nested message
// exists beside it.
func TestFlattenedNamesCollide(t *testing.T) {
	foo := &buffers.Message{Node: "p.Foo", Name: "Foo", Package: "p"}
	foo.Nested = []*buffers.Message{{Node: "p.Foo.Bar", Name: "Bar", Package: "p"}}
	f := &buffers.File{
		Path:     "p/p.proto",
		Package:  "p",
		Messages: []*buffers.Message{{Node: "p.FooBar", Name: "FooBar", Package: "p"}, foo},
	}

	r := &run{}
	r.reportCollisions(f)

	if len(r.diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(r.diags), r.diags)
	}
	msg := r.diags[0].Message
	for _, want := range []string{"p.FooBar", "p.Foo.Bar", "FooBar"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the message does not name %s: %s", want, msg)
		}
	}
}

// TestDistinctNamesDoNotCollide keeps the check from firing on the ordinary case,
// which is every schema that has no collision.
func TestDistinctNamesDoNotCollide(t *testing.T) {
	foo := &buffers.Message{Node: "p.Foo", Name: "Foo", Package: "p"}
	foo.Nested = []*buffers.Message{{Node: "p.Foo.Bar", Name: "Bar", Package: "p"}}
	f := &buffers.File{
		Path:     "p/p.proto",
		Package:  "p",
		Messages: []*buffers.Message{foo},
		Enums:    []*buffers.Enum{{Node: "p.Kind", Name: "Kind", Package: "p"}},
	}

	r := &run{}
	r.reportCollisions(f)
	if len(r.diags) != 0 {
		t.Errorf("a schema with no collision produced %d diagnostics: %v", len(r.diags), r.diags)
	}
}

// TestDocCommentCannotBeClosedByItsContent covers a proto comment that quotes a C
// or Java block comment.
//
// The sequence would otherwise end the generated `/** */` block early and leave
// the rest of the sentence as stray tokens, which fails to parse somewhere after
// the comment rather than in it. Thrift's lexer has no escape inside a comment,
// so a space is inserted instead.
func TestDocCommentCannotBeClosedByItsContent(t *testing.T) {
	if got := closeSafe("ends a block comment with */ inside"); strings.Contains(got, "*/") {
		t.Errorf("closeSafe left a terminator in place: %q", got)
	}
	if got := closeSafe("nothing to defuse here"); got != "nothing to defuse here" {
		t.Errorf("closeSafe altered ordinary prose: %q", got)
	}
}
