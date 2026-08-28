package thrift

// service.go renders a proto service as a Thrift service.
//
// Methods are the one place Thrift is *less* structured than the other targets
// here, and that turns out to help. A Thrift function has no ordinal — it is
// dispatched by name — so there is no slot to keep stable, no placeholder to
// emit for a skipped method, and nothing that shifts when one is removed.
//
// What Thrift does not have is streaming. It is a request/response protocol:
// one call in, one result out. A proto streaming method has no form here, so it
// is rendered as the unary call it most nearly is and reported, rather than
// dropped — a service missing half its methods is harder to debug than one whose
// generated client returns a single response where the caller expected a stream.

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// service renders one service and its methods.
func (r *run) service(b *emit.Buf, f *buffers.File, s *buffers.Service) {
	r.doc(b, s.Doc)
	b.Block(fmt.Sprintf("service %s {", typeName(string(s.Node), s.Package)), "}", func() {
		first := true
		for _, m := range s.Methods {
			if m.Skip || !allows(m.Targets) {
				b.Linef("# %s is excluded from this target. A Thrift function is dispatched by name,", m.Name)
				b.Line("# so nothing holds its place and nothing moves.")
				continue
			}
			if !first {
				b.Line("")
			}
			first = false
			r.method(b, f, m)
		}
	})
}

// method renders one RPC.
func (r *run) method(b *emit.Buf, f *buffers.File, m *buffers.Method) {
	notes := make([]string, 0, 2)
	if m.Pattern != "Custom" {
		notes = append(notes, fmt.Sprintf("AIP-%s standard method.", aipNumber(m.Pattern)))
	}
	if note := r.streamNote(m); note != "" {
		notes = append(notes, note)
	}
	r.doc(b, m.Doc, notes...)

	b.Linef("%s %s(%s)", r.returnType(m, f), ident(m.Name), r.params(m, f))
}

// returnType renders a method's result, which is `void` when the proto returns
// google.protobuf.Empty — Thrift has a real void and boxing nothing in a struct
// would only give the caller an empty object to unwrap.
func (r *run) returnType(m *buffers.Method, f *buffers.File) string {
	if m.Output == nil || m.Output.WellKnown == buffers.WKEmpty {
		return "void"
	}
	return r.qualify(string(m.Output.Node), f)
}

// params renders a method's argument list, which is empty when the proto takes
// google.protobuf.Empty.
func (r *run) params(m *buffers.Method, f *buffers.File) string {
	if m.Input == nil || m.Input.WellKnown == buffers.WKEmpty {
		return ""
	}
	return fmt.Sprintf("1: %s request", r.qualify(string(m.Input.Node), f))
}

// streamNote returns the caveat a streaming method carries, and records the
// diagnostic that goes with it.
func (r *run) streamNote(m *buffers.Method) string {
	side := ""
	switch {
	case m.ClientStream && m.ServerStream:
		side = "Bidirectional streaming"
	case m.ServerStream:
		side = "Server streaming"
	case m.ClientStream:
		side = "Client streaming"
	default:
		return ""
	}

	r.collect(&buffers.Diagnostic{
		Rule: buffers.RuleTarget,
		Node: m.Node,
		Message: fmt.Sprintf("%s has no Thrift form — Thrift is one request to one response — so it "+
			"is emitted as a unary call and the generated client returns a single message", m.Name),
		Hint: "carry the stream on a transport that has one, and keep this method for the " +
			"request/response half",
	})
	return fmt.Sprintf("%s in proto. Thrift is request/response and has no stream, so this is a "+
		"unary call: the client receives one message, not a sequence.", side)
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
	}
	return "136"
}
