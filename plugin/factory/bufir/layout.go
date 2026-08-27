package bufir

import (
	"fmt"
	"sort"
)

// layout.go decides which messages are packed records and which are evolvable
// ones, and rejects a packed layout that cannot be honoured.
//
// The eligibility rules are FlatBuffers' rules for a `struct`, because that is
// the strictest of the targets: a struct is a fixed-size inline blob, so every
// field must have a size known at schema time and no field may be absent. Cap'n
// Proto's data section is more forgiving, but a message that qualifies here
// qualifies everywhere, and one set of rules beats three.
//
// A message that *asks* for LAYOUT_STRUCT and does not qualify is an error naming
// the field that disqualified it. Quietly downgrading it to a table would produce
// a schema that compiles and misses the entire point of the request — the caller
// wanted a memory layout, not a set of fields.

// resolveLayouts assigns every message a concrete layout.
//
// Every message is resolved, imported ones included, because a generated schema
// has to know whether an imported message is a packed record before it can
// reference it. Only generated messages are *reported* on: a dependency that asks
// for a layout it cannot have is its own module's problem to fix.
//
// Messages are visited in sorted node order so that the diagnostics come out the
// same way on every run; the memo makes the traversal order irrelevant to the
// result, but not to the reporting.
func (b *builder) resolveLayouts() {
	nodes := make([]NodeID, 0, len(b.schema.Messages))
	for node := range b.schema.Messages {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })

	eligible := map[NodeID]bool{}
	visiting := map[NodeID]bool{}

	for _, node := range nodes {
		msg := b.schema.Messages[node]
		ok, reason := b.packable(msg, eligible, visiting)

		switch msg.Layout {
		case LayoutStruct:
			if !ok && msg.File.Generate {
				b.report(Diagnostic{
					Rule:    RuleLayout,
					Node:    msg.Node,
					Message: fmt.Sprintf("LAYOUT_STRUCT was requested but this message cannot be packed: %s", reason),
					Hint: "remove the layout option to emit an evolvable table, or move the disqualifying field " +
						"into a separate message the struct references",
				})
			}
			if !ok {
				msg.Layout = LayoutTable
			}
		case LayoutUnspecified:
			// Table is the default even when the message would qualify. See
			// buffers.v1.Layout's LAYOUT_UNSPECIFIED for why inference here would
			// turn an ordinary proto edit into a silent wire break.
			msg.Layout = LayoutTable
		}
	}
}

// packable reports whether a message may be a fixed-size inline record, and when
// it may not, the first reason why.
//
// The reason is the return value that matters: "Pose cannot be packed" sends
// someone reading their schema; "field `label` is a string, which has no
// fixed size" sends them to the line.
func (b *builder) packable(msg *Message, memo map[NodeID]bool, visiting map[NodeID]bool) (bool, string) {
	if got, ok := memo[msg.Node]; ok {
		return got, memoReason(got, msg)
	}
	if visiting[msg.Node] {
		// A message that transitively contains itself has no finite size. It is
		// legal proto — the field is a pointer there — and simply cannot be a
		// packed record.
		return false, fmt.Sprintf("%s contains itself, so it has no fixed size", msg.Name)
	}
	visiting[msg.Node] = true
	defer delete(visiting, msg.Node)

	ok, reason := b.packableFields(msg, memo, visiting)
	memo[msg.Node] = ok
	return ok, reason
}

// packableFields reports the first reason a message's fields disqualify it
// from a packed layout.
func (b *builder) packableFields(msg *Message, memo, visiting map[NodeID]bool) (bool, string) {
	switch {
	case msg.IsMapEntry:
		return false, "a map entry is synthesized per target and never emitted as a record"
	case msg.Resource != nil:
		// A resource is an evolvable API entity by construction: AIP-122 gives it
		// a string `name`, which already disqualifies it, and AIP-134 expects
		// fields to be added over its lifetime.
		return false, fmt.Sprintf("%s declares an AIP-123 resource, which is an evolvable entity by construction", msg.Name)
	case len(msg.Oneofs) > 0:
		return false, fmt.Sprintf("oneof %q needs a discriminant, which a fixed-size record has nowhere to put", msg.Oneofs[0].Name)
	case len(msg.Reserved) > 0:
		return false, "a removed field's slot must be held open, which a fixed-size record cannot do"
	}

	for _, f := range msg.Fields {
		if f.Skip {
			continue
		}
		switch {
		case f.Kind == KindMap:
			return false, fmt.Sprintf("field %q is a map, which is variable length", f.Name)
		case f.Repeated && f.FixedLen == 0:
			return false, fmt.Sprintf("field %q is repeated with no (buffers.v1.field).fixed_len, so its size is not known at schema time", f.Name)
		case f.Kind == KindString:
			return false, fmt.Sprintf("field %q is a string, which has no fixed size", f.Name)
		case f.Kind == KindBytes:
			return false, fmt.Sprintf("field %q is bytes, which has no fixed size", f.Name)
		case f.Optional:
			return false, fmt.Sprintf("field %q has explicit presence, which a fixed-size record cannot express — every field is always there", f.Name)
		case f.Kind == KindEnum, f.Kind.Scalar():
			continue
		case f.Kind == KindMessage:
			if f.WellKnown != WKNone {
				return false, fmt.Sprintf("field %q is %s, which every target renders as an evolvable record", f.Name, f.WellKnown)
			}
			child := b.schema.Messages[NodeID(f.Message)]
			if child == nil {
				return false, fmt.Sprintf("field %q references %s, which is not in this compilation unit", f.Name, f.Message)
			}
			ok, reason := b.packable(child, memo, visiting)
			if !ok {
				return false, fmt.Sprintf("field %q holds %s, which cannot be packed: %s", f.Name, child.Name, reason)
			}
		default:
			return false, fmt.Sprintf("field %q is a %s, which has no fixed size", f.Name, f.Kind)
		}
	}
	return true, ""
}

// memoReason reconstructs a short reason for a memoized negative result. The
// specific first-failing-field reason is not cached — it would mean carrying a
// string per message to serve a diagnostic that fires at most once per schema —
// so a repeat visit gets the general form and the caller's own context supplies
// the rest.
func memoReason(ok bool, msg *Message) string {
	if ok {
		return ""
	}
	return fmt.Sprintf("%s is not a fixed-size record", msg.Name)
}
