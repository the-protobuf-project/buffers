package ros

// srv.go renders the call surface: a .srv per request/response method, and a
// manifest of the topics the streaming ones publish.
//
// The manifest exists because ROS declares the topic-to-type binding in launch
// files and in code, never in the IDL — so a schema-derived record of it exists
// nowhere else, and it is what eCAL publisher and subscriber generation will read.

import (
	"sort"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/names"
)

// topic is one publication, recorded for the manifest.
type topic struct {
	// Name is the topic a subscriber listens on.
	Name string
	// Type is the ROS message carried on it.
	Type string
	// Service is the proto service the topic was derived from.
	Service string
	// Method is the streaming method that publishes it.
	Method string
	// Doc is that method's first doc line.
	Doc string
}

// service renders one .srv per call method, and records every publication for the
// topic manifest.
func (r *run) service(f *buffers.File, s *buffers.Service) error {
	for _, m := range s.Methods {
		if m.Skip || !allows(m.Targets) {
			continue
		}

		switch m.Transport {
		case buffers.TransportTopic:
			payload := "std_msgs/Empty"
			if m.Output != nil {
				payload = r.qualify(r.messageRosName(string(m.Output.Node)), f)
			}
			r.topics = append(r.topics, topic{
				Name:    m.Topic,
				Type:    f.ROSPackage + "/" + strings.TrimPrefix(payload, f.ROSPackage+"/"),
				Service: s.Name,
				Method:  m.Name,
				Doc:     firstLine(m.Doc),
			})
			continue

		case buffers.TransportAction:
			r.collect(&buffers.Diagnostic{
				Rule: buffers.RuleTarget,
				Node: m.Node,
				Message: "TRANSPORT_ACTION needs a goal, result and feedback payload, which is derivable " +
					"only from an AIP-151 long-running method returning google.longrunning.Operation",
				Hint: "return an Operation, or use TRANSPORT_CALL",
			})
			continue
		}

		if !s.ROSService {
			continue
		}
		if err := r.srv(f, s, m); err != nil {
			return err
		}
	}
	return nil
}

// srv renders one .srv file.
//
// The request and response sections carry the input and output messages' fields
// inline rather than a single field naming each message. Inlining is what a ROS
// caller expects — `GetSensor.Request.name`, not `GetSensor.Request.request.name`
// — and the wrapper messages are still emitted as .msg files for anyone who wants
// them.
func (r *run) srv(f *buffers.File, s *buffers.Service, m *buffers.Method) error {
	var b emit.Buf
	b.Raw(r.banner(f.Path))
	b.Line("")
	b.Doc("#", m.Doc)
	if m.Pattern != "Custom" {
		b.Linef("# AIP standard method: %s.", m.Pattern)
	}
	b.Linef("# Generated from %s.%s.", s.Name, m.Name)
	b.Line("")

	if m.Input != nil {
		r.fields(&b, f, m.Input)
	}
	// The request/response separator. This is ROS .srv grammar, not formatting:
	// rosidl splits the file on a line of exactly three dashes, and a .srv
	// without one does not parse. It is the only place this repository emits
	// that sequence.
	b.Line("---")
	if m.Output != nil && m.Output.WellKnown != buffers.WKEmpty {
		r.fields(&b, f, m.Output)
	}

	return r.sink(srvPath(f.ROSPackage, names.Pascal(m.ROSName)), b.Bytes())
}

// manifest records every publication as YAML.
//
// ROS has no file that declares which topic carries which message — that lives in
// launch files and in code — so a schema-derived list is the only place the
// binding is written down. It is also what the eventual eCAL publisher and
// subscriber generation reads, since an eCAL channel is the same idea.
func (r *run) manifest(schema *buffers.Schema) error {
	if len(r.topics) == 0 {
		return nil
	}
	sort.Slice(r.topics, func(i, j int) bool { return r.topics[i].Name < r.topics[j].Name })

	var b emit.Buf
	b.Raw(r.banner("(every service in this run)"))
	b.Line("")
	b.Line("# Topics derived from server-streaming methods.")
	b.Line("#")
	b.Line("# ROS declares the topic-to-type binding in launch files and in code, never in")
	b.Line("# the IDL, so this manifest is the only schema-derived record of it. It is also")
	b.Line("# what eCAL publisher and subscriber generation will read: an eCAL channel and a")
	b.Line("# ROS topic are the same idea under two names.")
	b.Line("")
	b.Line("topics:")
	b.In()
	for _, t := range r.topics {
		b.Linef("- name: %q", t.Name)
		b.In()
		b.Linef("type: %q", t.Type)
		b.Linef("source: %q", t.Service+"."+t.Method)
		if t.Doc != "" {
			b.Linef("doc: %q", t.Doc)
		}
		b.Out()
	}
	b.Out()

	return r.sink("topics.yaml", b.Bytes())
}
