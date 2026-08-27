package bufir

// build_service.go walks services and methods, and classifies how each method's
// messages move — a call, a publication, or a long-running action.

import (
	"strings"

	"github.com/the-protobuf-project/protokit/naming"
	"google.golang.org/protobuf/compiler/protogen"

	bufferspbv1 "github.com/the-protobuf-project/buffers/plugin/pb/bufferspbv1"
)

// service walks one service and resolves the defaults its methods decide.
func (b *builder) service(s *protogen.Service, file *File) *Service {
	sd := s.Desc
	opts := serviceOpts(sd)

	svc := &Service{
		Node:    NodeID(sd.FullName()),
		Name:    string(sd.Name()),
		Package: string(sd.ParentFile().Package()),
		File:    file,
		Doc:     docOf(s.Comments.Leading),
		CapnpID: resolveCapnpID(opts.GetCapnpId(), func() uint64 { return typeCapnpID(string(sd.FullName())) }),
		Targets: append([]string(nil), opts.GetTargets()...),
		Skip:    opts.GetSkip(),
	}

	for _, m := range s.Methods {
		svc.Methods = append(svc.Methods, b.method(m, svc))
	}

	// Defaults that depend on the methods, so they are resolved after the loop.
	//
	// A service with no methods yields no interface: Cap'n Proto accepts an empty
	// one, but emitting it claims a type ID and an ordinal space for something
	// that can never be called.
	svc.CapnpInterface = len(svc.Methods) > 0
	if opts != nil && opts.CapnpInterface != nil {
		svc.CapnpInterface = opts.GetCapnpInterface()
	}

	// ROS services need a request and a response, so a service whose methods are
	// all publications has nothing to emit.
	svc.ROSService = false
	for _, m := range svc.Methods {
		if m.Transport == TransportCall {
			svc.ROSService = true
			break
		}
	}
	if opts != nil && opts.RosService != nil {
		svc.ROSService = opts.GetRosService()
	}

	b.assignMethodSlots(svc)
	return svc
}

// method walks one RPC and classifies how its messages move.
func (b *builder) method(m *protogen.Method, parent *Service) *Method {
	md := m.Desc
	opts := methodOpts(md)

	method := &Method{
		Node:         NodeID(md.FullName()),
		Name:         string(md.Name()),
		Doc:          docOf(m.Comments.Leading),
		Parent:       parent,
		Input:        b.schema.Messages[NodeID(md.Input().FullName())],
		Output:       b.schema.Messages[NodeID(md.Output().FullName())],
		ClientStream: md.IsStreamingClient(),
		ServerStream: md.IsStreamingServer(),
		ROSName:      orDefault(opts.GetRosType(), string(md.Name())),
		Targets:      append([]string(nil), opts.GetTargets()...),
		Skip:         opts.GetSkip(),
		Pattern:      classifyMethod(string(md.Name())),
	}
	if opts != nil && opts.Ordinal != 0 {
		method.Ordinal = opts.GetOrdinal()
	}

	method.Transport = b.resolveTransport(method, opts)
	method.Topic = orDefault(opts.GetTopic(), naming.SnakeCase(method.Name))
	return method
}

// resolveTransport decides how a method's messages move.
//
// The declared transport wins; otherwise a server-streaming method is a
// publication and everything else is a call. A client-streaming method has no
// honest reading in any target here — ROS has no such thing, and a Cap'n Proto
// method takes one parameter struct — so it is reported rather than guessed at.
func (b *builder) resolveTransport(m *Method, opts *bufferspbv1.MethodOptions) Transport {
	declared := transportOf(opts.GetTransport())

	if m.ClientStream {
		b.report(Diagnostic{
			Rule:    RuleTarget,
			Node:    m.Node,
			Message: "client streaming has no equivalent in any buffers target; the method is emitted as a call over its input message",
			Hint:    "model the stream as a topic the client publishes to, or exclude the method with (buffers.v1.method).skip",
		})
	}

	if declared != TransportUnspecified {
		if declared == TransportTopic && !m.ServerStream {
			b.report(Diagnostic{
				Rule:    RuleLint,
				Node:    m.Node,
				Message: "declared TRANSPORT_TOPIC on a unary method; its response will be published once per call rather than streamed",
				Hint:    "add the `stream` keyword to the response, or use TRANSPORT_CALL",
			})
		}
		return declared
	}
	if m.ServerStream {
		return TransportTopic
	}
	return TransportCall
}

// classifyMethod names the AIP standard method a name implies.
//
// It is prefix matching on the name, and deliberately nothing more. protokit's
// service IR classifies more thoroughly — it checks the shape the name implies
// and rejects a GetBook carrying a body — but reaching for it here would drag in
// the whole route table, including the conflict analysis that fails a build over
// two overlapping google.api.http templates. That is the right check for a
// gateway and an unrelated reason for a .capnp file to fail to generate. A proto
// with no HTTP annotations at all is the common robotics input, and it must still
// produce a schema.
//
// What the classification is used for is correspondingly modest: shaping a ROS
// .srv like the standard method it implements, and ordering an interface's
// methods. Nothing here decides whether a payload is well formed.
func classifyMethod(name string) string {
	for _, prefix := range []string{
		"BatchGet", "BatchCreate", "BatchUpdate", "BatchDelete",
		"Get", "List", "Create", "Update", "Delete", "Undelete", "Search",
	} {
		if strings.HasPrefix(name, prefix) {
			return prefix
		}
	}
	return "Custom"
}

// assignSlots runs the ordinal pass over every indexed message and enum.
//
// It walks the index rather than the file tree so that nested messages and
// imported types are covered by exactly the same code path as top-level generated
// ones. Sorting the node IDs first is what makes the resulting ledger and the
// diagnostics deterministic; ranging a map here would reorder buffers.lock on
