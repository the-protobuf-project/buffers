package thrift

// select.go decides what this target emits and where it goes: which declarations
// survive the skip and target filters, flattened for a format that has no
// nesting, and the small helpers every renderer here shares.

import (
	"path"
	"sort"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
)

// flattenMessages returns every message a file declares, nested ones included.
//
// Thrift has one flat scope per file, so a nested proto message becomes a
// sibling declaration with its path folded into the name; see names.go. Map entry
// messages are dropped outright, because Thrift has a real map and nothing here
// needs the synthetic pair type protoc invents.
func flattenMessages(f *buffers.File) []*buffers.Message {
	var out []*buffers.Message
	var walk func(msgs []*buffers.Message)
	walk = func(msgs []*buffers.Message) {
		for _, m := range msgs {
			if !m.Skip && !m.IsMapEntry && allows(m.Targets) {
				out = append(out, m)
			}
			walk(m.Nested)
		}
	}
	walk(f.Messages)
	return out
}

// flattenEnums returns every enum a file declares, nested ones included, since
// Thrift has no nesting and each becomes a top-level declaration.
func flattenEnums(f *buffers.File) []*buffers.Enum {
	var out []*buffers.Enum
	for _, e := range f.Enums {
		if !e.Skip {
			out = append(out, e)
		}
	}
	var walk func(msgs []*buffers.Message)
	walk = func(msgs []*buffers.Message) {
		for _, m := range msgs {
			for _, e := range m.Enums {
				if !e.Skip {
					out = append(out, e)
				}
			}
			walk(m.Nested)
		}
	}
	walk(f.Messages)
	return out
}

// emittableServices returns the services this target renders.
func emittableServices(f *buffers.File) []*buffers.Service {
	var out []*buffers.Service
	for _, s := range f.Services {
		if s.Skip || !allows(s.Targets) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// liveOneofs returns the oneofs of a message that become a union type, in the
// order their first arm occupies.
//
// A one-armed oneof is excluded: it carries no more information than a plain
// optional field, and a Thrift union with a single member is a discriminant that
// can only ever hold one value. struct.go renders that arm inline instead.
func liveOneofs(m *buffers.Message) []*buffers.Oneof {
	var out []*buffers.Oneof
	for _, one := range m.Oneofs {
		if one.Skip || len(liveArms(one)) < 2 {
			continue
		}
		out = append(out, one)
	}
	return out
}

// liveArms returns a oneof's surviving arms in proto field number order, which is
// the order their Thrift ids run in.
func liveArms(one *buffers.Oneof) []*buffers.Field {
	arms := make([]*buffers.Field, 0, len(one.Fields))
	for _, f := range one.Fields {
		if f.Skip || !allows(f.Targets) {
			continue
		}
		arms = append(arms, f)
	}
	sort.SliceStable(arms, func(i, j int) bool { return arms[i].Number < arms[j].Number })
	return arms
}

// ownerOf returns the file declaring a named type, for qualification.
func (r *run) ownerOf(fullName string) *buffers.File {
	if m := r.schema.Messages[buffers.NodeID(fullName)]; m != nil {
		return m.File
	}
	if e := r.schema.Enums[buffers.NodeID(fullName)]; e != nil {
		return e.File
	}
	return nil
}

// collect records a diagnostic, ignoring nil so callers can pass a projection's
// result directly.
func (r *run) collect(d *buffers.Diagnostic) {
	if d != nil {
		r.diags = append(r.diags, *d)
	}
}

// banner renders the file header, naming the .proto the schema came from.
//
// The ledger note every other target carries is replaced rather than added to. A
// Thrift field id is the proto field number, so nothing in buffers.lock decides
// anything in this file, and pointing a reader at the ledger to explain an id
// would send them somewhere that does not record it.
func (r *run) banner(protoPath string) string {
	info := r.info
	info.Source = protoPath
	info.NoLedger = true
	info.Notes = append(info.Notes,
		"field ids are the proto field numbers; Thrift numbers fields the way proto does")
	return provenance.Render(provenance.Hash, info)
}

// allows reports whether a target allow-list admits this target. An empty list
// admits everything, which is the common case.
func allows(list []string) bool {
	if len(list) == 0 {
		return true
	}
	for _, t := range list {
		if t == "thrift" {
			return true
		}
	}
	return false
}

// thriftPath maps a proto path onto its .thrift sibling.
func thriftPath(protoPath string) string {
	return strings.TrimSuffix(protoPath, path.Ext(protoPath)) + ".thrift"
}
