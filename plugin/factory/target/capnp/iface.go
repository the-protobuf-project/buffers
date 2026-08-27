package capnp

// iface.go renders enums and the RPC surface: an interface per service, and the
// callback interface a server-streaming method needs.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// enum renders a proto enum.
func (r *run) enum(b *emit.Buf, e *bufir.Enum) {
	b.Doc("#", e.Doc)
	if e.Underlying.Bits() != 16 {
		b.Linef("# (buffers.v1.enumeration).underlying asked for %d bits. A Cap'n Proto enum is", e.Underlying.Bits())
		b.Line("# always 16, with no way to declare otherwise; the narrowing is honoured by")
		b.Line("# the FlatBuffers target only.")
	}
	if e.BitFlags {
		b.Line("# Declared (bit_flags). Cap'n Proto has no bitmask enum; the values are emitted")
		b.Line("# as ordinary enumerants and a reader must not treat them as bit positions.")
	}

	// Cap'n Proto enumerants are positional, so they are emitted in ordinal order
	// rather than proto declaration order.
	values := make([]*bufir.EnumValue, 0, len(e.Values))
	for _, v := range e.Values {
		if !v.Skip {
			values = append(values, v)
		}
	}
	sort.SliceStable(values, func(i, j int) bool { return values[i].Ordinal < values[j].Ordinal })

	b.Block(fmt.Sprintf("enum %s @0x%016x {", typeName(e.Name), bufir.DeriveTypeID(string(e.Node))), "}", func() {
		for _, v := range values {
			b.Doc("#", v.Doc)
			b.Linef("%s @%d;", enumerant(e.Name, v.Name), v.Ordinal)
		}
	})
}

// iface renders a proto service as a Cap'n Proto interface.
func (r *run) iface(b *emit.Buf, f *bufir.File, s *bufir.Service) {
	b.Doc("#", s.Doc)
	b.Block(fmt.Sprintf("interface %s @0x%016x {", typeName(s.Name), s.CapnpID), "}", func() {
		for i, m := range s.Methods {
			if m.Skip || !allows(m.Targets) {
				b.Linef("# %s is excluded from this target; this holds its ordinal.", m.Name)
				b.Linef("excluded%d @%d () -> ();", i, m.Ordinal)
				continue
			}
			if i > 0 {
				b.Line("")
			}
			r.method(b, f, m)
		}
	})
}

// method renders one RPC.
func (r *run) method(b *emit.Buf, f *bufir.File, m *bufir.Method) {
	b.Doc("#", m.Doc)
	if m.Pattern != "Custom" {
		b.Linef("# AIP-%s standard method.", aipNumber(m.Pattern))
	}

	params := "()"
	if m.Input != nil && m.Input.WellKnown != bufir.WKEmpty {
		params = fmt.Sprintf("(request :%s)", r.qualify(string(m.Input.Node), f))
	}

	// A server-streaming method has no direct Cap'n Proto form. The idiom is to
	// pass a capability the callee pushes to, so the stream becomes a parameter
	// rather than a return type.
	if m.ServerStream {
		sink := typeName(m.Name) + "Sink"
		element := "Void"
		if m.Output != nil {
			element = r.qualify(string(m.Output.Node), f)
		}
		r.sinks = append(r.sinks, sinkIface{
			Name:    sink,
			ID:      bufir.DeriveTypeID(string(m.Node) + ".sink"),
			Element: element,
			Method:  m.Name,
			Doc:     m.Doc,
		})

		b.Line("# Server streaming. Cap'n Proto RPC has no streaming return, so the stream is")
		b.Linef("# a capability the caller supplies and the callee pushes to. Topic: %q.", m.Topic)
		inner := strings.TrimSuffix(strings.TrimPrefix(params, "("), ")")
		if inner != "" {
			inner += ", "
		}
		b.Linef("%s @%d (%ssink :%s) -> ();", member(m.Name), m.Ordinal, inner, sink)
		return
	}

	results := "()"
	if m.Output != nil && m.Output.WellKnown != bufir.WKEmpty {
		results = fmt.Sprintf("(response :%s)", r.qualify(string(m.Output.Node), f))
	}
	b.Linef("%s @%d %s -> %s;", member(m.Name), m.Ordinal, params, results)
}

// sinkInterface renders the callback interface a streaming method needs.
func (r *run) sinkInterface(b *emit.Buf, s sinkIface) {
	b.Linef("# Stream sink for %s. The caller implements it; the callee calls push once", s.Method)
	b.Line("# per element and done when the stream ends. Returning from push is the")
	b.Line("# backpressure signal: the callee should await it before pushing again.")
	b.Block(fmt.Sprintf("interface %s @0x%016x {", s.Name, s.ID), "}", func() {
		b.Linef("push @0 (value :%s) -> ();", s.Element)
		b.Line("done @1 () -> ();")
	})
}

// aipNumber maps a classified method pattern to the AIP that defines it, for the
// one-line note on each standard method.
func aipNumber(pattern string) string {
	switch pattern {
	case "Get":
		return "131"
	case "List":
		return "132"
	case "Create":
		return "133"
	case "Update":
		return "134"
	case "Delete":
		return "135"
	case "BatchGet":
		return "231"
	case "BatchCreate":
		return "233"
	case "BatchUpdate":
		return "234"
	case "BatchDelete":
		return "235"
	case "Undelete":
		return "164"
	case "Search":
		return "136"
	}
	return "136"
}
