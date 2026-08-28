package thrift

// order.go puts a file's structs in an order Thrift's parser will accept.
//
// Thrift resolves a type name where it is used. A struct naming a type declared
// further down the same file is not a forward reference, it is
// "Type X has not been defined" — so declaration order is load-bearing here in a
// way it is not for any other target. FlatBuffers and Cap'n Proto both resolve
// the whole file before checking, and proto itself has no ordering rule at all,
// so a message graph that compiles everywhere else can still fail here.
//
// The order is therefore derived rather than copied from the proto. Within a
// file, a struct is emitted after everything it names. Across files it does not
// arise: an `include` is fully parsed before the including file's body.

import (
	"github.com/the-protobuf-project/protokit/buffers"
)

// orderedMessages returns a file's emittable messages, dependency-first.
//
// The walk is a depth-first post-order over same-file message references, seeded
// in declaration order, which makes the result deterministic and keeps unrelated
// types in the order the author wrote them.
func orderedMessages(schema *buffers.Schema, f *buffers.File) []*buffers.Message {
	all := flattenMessages(f)
	inFile := make(map[buffers.NodeID]*buffers.Message, len(all))
	for _, m := range all {
		inFile[m.Node] = m
	}

	var out []*buffers.Message
	state := make(map[buffers.NodeID]int, len(all)) // 0 unvisited, 1 on stack, 2 done

	var visit func(m *buffers.Message)
	visit = func(m *buffers.Message) {
		switch state[m.Node] {
		case 2:
			return
		case 1:
			// A cycle. Thrift can express a struct that names itself, but two
			// structs that name each other cannot both be declared first, so the
			// edge is dropped here and reported by cycles() rather than looping.
			return
		}
		state[m.Node] = 1
		for _, dep := range dependencies(m) {
			if next, ok := inFile[dep]; ok && next != m {
				visit(next)
			}
		}
		state[m.Node] = 2
		out = append(out, m)
	}

	for _, m := range all {
		visit(m)
	}
	return out
}

// dependencies returns the same-file message types a message names, in a stable
// order: its fields' types, including a map's key and value and every oneof arm,
// which all appear in Fields.
func dependencies(m *buffers.Message) []buffers.NodeID {
	var out []buffers.NodeID
	add := func(f *buffers.Field) {
		if f != nil && f.Kind == buffers.KindMessage && f.WellKnown == buffers.WKNone {
			out = append(out, buffers.NodeID(f.Message))
		}
	}
	for _, f := range m.Fields {
		if f.Skip || !allows(f.Targets) {
			continue
		}
		if f.Kind == buffers.KindMap {
			add(f.MapKey)
			add(f.MapValue)
			continue
		}
		add(f)
	}
	return out
}

// cycles reports the messages caught in a mutual reference, which Thrift cannot
// order.
//
// Self-reference is excluded: Thrift registers a struct's name before parsing its
// body, so a tree node naming its own type compiles. Two structs naming each
// other do not, whichever is emitted first.
func cycles(f *buffers.File) [][]*buffers.Message {
	all := flattenMessages(f)
	inFile := make(map[buffers.NodeID]*buffers.Message, len(all))
	for _, m := range all {
		inFile[m.Node] = m
	}

	var found [][]*buffers.Message
	state := make(map[buffers.NodeID]int, len(all))
	var stack []*buffers.Message

	var visit func(m *buffers.Message)
	visit = func(m *buffers.Message) {
		switch state[m.Node] {
		case 2:
			return
		case 1:
			for i, on := range stack {
				if on == m {
					found = append(found, append([]*buffers.Message(nil), stack[i:]...))
					return
				}
			}
			return
		}
		state[m.Node] = 1
		stack = append(stack, m)
		for _, dep := range dependencies(m) {
			if next, ok := inFile[dep]; ok && next != m {
				visit(next)
			}
		}
		stack = stack[:len(stack)-1]
		state[m.Node] = 2
	}

	for _, m := range all {
		visit(m)
	}
	return found
}
