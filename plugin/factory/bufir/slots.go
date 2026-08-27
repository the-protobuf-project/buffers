package bufir

// slots.go drives the ordinal pass over the built graph.
//
// The derivation itself lives in ordinal.go, which knows nothing about
// descriptors; this is the half that walks the graph, feeds it, and records the
// result into the ledger.

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// assignSlots runs the ordinal pass over every indexed message and enum.
//
// It walks the index rather than the file tree so that nested messages and
// imported types take exactly the same path as top-level generated ones.
// Sorting the node IDs first is what makes the ledger and the diagnostics
// deterministic; ranging a map here would reorder buffers.lock on every run.
func (b *builder) assignSlots() {
	nodes := make([]NodeID, 0, len(b.schema.Messages))
	for node := range b.schema.Messages {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })

	for _, node := range nodes {
		msg := b.schema.Messages[node]
		if msg.IsMapEntry {
			// A map entry is never emitted as itself; its two slots are fixed by
			// whichever target synthesizes a replacement for it.
			continue
		}
		if !msg.File.Generate {
			// An imported type's slots belong to the module that emits it. Taking
			// a position on them here would put google.protobuf's field numbering
			// into this repository's ledger and report on holes in someone else's
			// schema that this run has no standing to comment on.
			continue
		}
		b.assignFieldSlots(msg)
	}

	nodes = nodes[:0]
	for node := range b.schema.Enums {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	for _, node := range nodes {
		enum := b.schema.Enums[node]
		if !enum.File.Generate {
			continue
		}
		b.assignValueSlots(enum)
	}
}

// assignFieldSlots resolves one message's fields and records the result.
func (b *builder) assignFieldSlots(msg *Message) {
	inputs := make([]slotInput, len(msg.Fields))
	for i, f := range msg.Fields {
		inputs[i] = slotInput{
			Node:   f.Node,
			Name:   f.Name,
			Number: f.Number,
			Pin:    f.Ordinal,
			HasPin: f.Ordinal != 0,
			Skip:   f.Skip,
		}
	}

	assigned, slots := assignFieldOrdinals(
		msg.Node, inputs, msg.reserved, b.lock.FieldSlots(msg.Node), b.report)

	entries := make([]SlotEntry, 0, len(assigned))
	for i, a := range assigned {
		msg.Fields[i].Ordinal = a.Ordinal
		msg.Fields[i].OrdinalSource = a.Source
		entries = append(entries, SlotEntry{
			Number:  msg.Fields[i].Number,
			Ordinal: a.Ordinal,
			Name:    msg.Fields[i].Name,
		})
	}
	msg.Reserved = slots
	b.recorder.Message(msg.Node, entries)
}

// assignValueSlots resolves one enum's values and records the result.
func (b *builder) assignValueSlots(enum *Enum) {
	inputs := make([]slotInput, len(enum.Values))
	for i, v := range enum.Values {
		inputs[i] = slotInput{
			Node:   v.Node,
			Name:   v.Name,
			Number: v.Number,
			Pin:    v.Ordinal,
			HasPin: v.Ordinal != 0,
			Skip:   v.Skip,
		}
	}

	assigned := assignEnumOrdinals(inputs, b.lock.ValueSlots(enum.Node), b.report)
	entries := make([]SlotEntry, 0, len(assigned))
	for i, a := range assigned {
		enum.Values[i].Ordinal = a.Ordinal
		enum.Values[i].OrdinalSource = a.Source
		entries = append(entries, SlotEntry{
			Number:  enum.Values[i].Number,
			Ordinal: a.Ordinal,
			Name:    enum.Values[i].Name,
		})
	}
	b.recorder.Enum(enum.Node, entries)
}

// assignMethodSlots numbers an interface's methods.
//
// Declaration order, not name order: a Cap'n Proto interface's ordinals are
// positional, and ordering by name would move every method the first time
// somebody added an `AbortJob`. The ledger is what catches a method inserted in
// the middle, which does move everything after it.
func (b *builder) assignMethodSlots(svc *Service) {
	locked := b.lock.MethodSlots(svc.Node)
	entries := make([]MethodEntry, 0, len(svc.Methods))

	for i, m := range svc.Methods {
		ordinal, source := int32(i), OrdinalDerived
		if got, ok := locked[m.Name]; ok {
			if got != ordinal {
				b.report(Diagnostic{
					Rule: RuleOrdinal,
					Node: m.Node,
					Message: fmt.Sprintf("buffers.lock records method ordinal %d, but this build derives %d; the ledger wins",
						got, ordinal),
					Hint: "a method was inserted or removed above this one; Cap'n Proto method ordinals are positional",
				})
			}
			ordinal, source = got, OrdinalLocked
		}
		if m.Ordinal != 0 {
			ordinal, source = m.Ordinal, OrdinalPinned
		}
		m.Ordinal, m.OrdinalSource = ordinal, source
		entries = append(entries, MethodEntry{Name: m.Name, Ordinal: ordinal})
	}
	b.recorder.Service(svc.Node, entries)
}

// reservedRangesOf reads a message's `reserved N to M;` declarations.
//
// protoreflect reports the end exclusive; this package works inclusive
// throughout, because that is how the proto source reads and how a diagnostic
// suggesting a `reserved` line has to phrase it.
func reservedRangesOf(md protoreflect.MessageDescriptor) []reservedRange {
	ranges := md.ReservedRanges()
	out := make([]reservedRange, 0, ranges.Len())
	for i := range ranges.Len() {
		r := ranges.Get(i)
		out = append(out, reservedRange{Start: int32(r[0]), End: int32(r[1]) - 1})
	}
	return out
}

// fileDoc recovers a file's own leading comment.
//
// protogen exposes comments on messages, fields and services but not on the file
// itself, so this reads the source location for the `syntax` declaration — field
// 12 of FileDescriptorProto — which is where protoc files the comment block at
// the top of a .proto.
func fileDoc(fd protoreflect.FileDescriptor) string {
	loc := fd.SourceLocations().ByPath(protoreflect.SourcePath{12})
	return docOf(protogen.Comments(loc.LeadingComments))
}

// checkROSPackages reports one proto package whose files disagree about which ROS
// package they belong to.
//
// It is legal — the ROS target will emit the types into two package directories
// and reference them across the boundary — and it is almost always an oversight,
// because ros_package is set per file and forgetting it on one file of a package
// silently defaults that file to the proto package name. The result compiles and
// splits an enum away from the messages that use it, which is exactly the shape
