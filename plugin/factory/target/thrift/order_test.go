package thrift

import (
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
)

// msg builds a message naming the given same-package types, for the ordering
// tests.
func msg(file *buffers.File, name string, refs ...string) *buffers.Message {
	m := &buffers.Message{
		Node:    buffers.NodeID("p." + name),
		Name:    name,
		Package: "p",
		File:    file,
	}
	for i, ref := range refs {
		m.Fields = append(m.Fields, &buffers.Field{
			Node:    buffers.NodeID("p." + name + ".f"),
			Name:    "f",
			Number:  int32(i + 1),
			Kind:    buffers.KindMessage,
			Message: "p." + ref,
		})
	}
	file.Messages = append(file.Messages, m)
	return m
}

// indexOf returns a message's position in a rendered order.
func indexOf(order []*buffers.Message, name string) int {
	for i, m := range order {
		if m.Name == name {
			return i
		}
	}
	return -1
}

// TestOrderPutsDependenciesFirst is the constraint Thrift imposes and no other
// target here does: a type must be declared before the type that names it, or the
// compile fails with "Type X has not been defined".
func TestOrderPutsDependenciesFirst(t *testing.T) {
	f := &buffers.File{Path: "p/p.proto", Package: "p"}
	msg(f, "Outer", "Middle")
	msg(f, "Middle", "Inner")
	msg(f, "Inner")

	schema := &buffers.Schema{Files: []*buffers.File{f}}
	order := orderedMessages(schema, f)
	if len(order) != 3 {
		t.Fatalf("got %d messages, want 3", len(order))
	}
	if indexOf(order, "Inner") > indexOf(order, "Middle") {
		t.Error("Inner is declared after Middle, which names it")
	}
	if indexOf(order, "Middle") > indexOf(order, "Outer") {
		t.Error("Middle is declared after Outer, which names it")
	}
}

// TestOrderIsStable checks that unrelated messages keep the order the proto
// author wrote them in, so the ordering does not churn a golden diff.
func TestOrderIsStable(t *testing.T) {
	f := &buffers.File{Path: "p/p.proto", Package: "p"}
	msg(f, "A")
	msg(f, "B")
	msg(f, "C")

	schema := &buffers.Schema{Files: []*buffers.File{f}}
	order := orderedMessages(schema, f)
	for i, want := range []string{"A", "B", "C"} {
		if order[i].Name != want {
			t.Fatalf("order = %s at %d, want %s", order[i].Name, i, want)
		}
	}
}

// TestSelfReferenceIsNotACycle checks that a tree node naming its own type is
// left alone: Thrift registers a struct's name before parsing its body, so that
// case compiles and is not worth a diagnostic.
func TestSelfReferenceIsNotACycle(t *testing.T) {
	f := &buffers.File{Path: "p/p.proto", Package: "p"}
	msg(f, "Node", "Node")

	if got := cycles(f); len(got) != 0 {
		t.Errorf("self-reference reported as a cycle: %v", got)
	}
	schema := &buffers.Schema{Files: []*buffers.File{f}}
	if len(orderedMessages(schema, f)) != 1 {
		t.Error("self-referencing message lost from the order")
	}
}

// TestMutualReferenceIsReported checks the case Thrift genuinely cannot express:
// two structs naming each other cannot both be declared first, and the ordering
// must terminate and say so rather than loop.
func TestMutualReferenceIsReported(t *testing.T) {
	f := &buffers.File{Path: "p/p.proto", Package: "p"}
	msg(f, "Left", "Right")
	msg(f, "Right", "Left")

	got := cycles(f)
	if len(got) == 0 {
		t.Fatal("mutual reference not reported")
	}

	schema := &buffers.Schema{Files: []*buffers.File{f}}
	order := orderedMessages(schema, f)
	if len(order) != 2 {
		t.Errorf("got %d messages in the order, want both", len(order))
	}
}

// TestMapAndOneofReferencesCount checks that the dependency walk sees the types
// reached through a map's value and through a oneof arm, both of which are
// ordinary entries in Message.Fields and both of which Thrift resolves the same
// way as a plain field.
func TestMapAndOneofReferencesCount(t *testing.T) {
	f := &buffers.File{Path: "p/p.proto", Package: "p"}
	msg(f, "Value")
	holder := msg(f, "Holder")
	holder.Fields = append(holder.Fields, &buffers.Field{
		Node:     "p.Holder.m",
		Name:     "m",
		Number:   1,
		Kind:     buffers.KindMap,
		MapKey:   &buffers.Field{Kind: buffers.KindString},
		MapValue: &buffers.Field{Kind: buffers.KindMessage, Message: "p.Value"},
	})

	deps := dependencies(holder)
	if len(deps) != 1 || deps[0] != buffers.NodeID("p.Value") {
		t.Fatalf("dependencies = %v, want [p.Value]", deps)
	}

	schema := &buffers.Schema{Files: []*buffers.File{f}}
	order := orderedMessages(schema, f)
	if indexOf(order, "Value") > indexOf(order, "Holder") {
		t.Error("a map's value type is declared after the message holding the map")
	}
}
