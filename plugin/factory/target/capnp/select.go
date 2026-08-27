package capnp

// select.go decides what this target emits and where it goes: which declarations
// survive the skip and target filters, and the small helpers every renderer here
// shares.

import (
	"path"
	"strings"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
)

// topLevel returns the file's emittable top-level messages.
func topLevel(f *bufir.File) []*bufir.Message {
	var out []*bufir.Message
	for _, m := range f.Messages {
		if m.Skip || m.IsMapEntry || !allows(m.Targets) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// topLevelEnums returns the file's emittable top-level enums.
func topLevelEnums(f *bufir.File) []*bufir.Enum {
	var out []*bufir.Enum
	for _, e := range f.Enums {
		if !e.Skip {
			out = append(out, e)
		}
	}
	return out
}

// emittableServices returns the services this target renders an interface for.
func emittableServices(f *bufir.File) []*bufir.Service {
	var out []*bufir.Service
	for _, s := range f.Services {
		if s.Skip || !s.CapnpInterface || !allows(s.Targets) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// ownerOf returns the file declaring a named type, for qualification.
func (r *run) ownerOf(fullName string) *bufir.File {
	if m := r.schema.Messages[bufir.NodeID(fullName)]; m != nil {
		return m.File
	}
	if e := r.schema.Enums[bufir.NodeID(fullName)]; e != nil {
		return e.File
	}
	return nil
}

// collect records a diagnostic, ignoring nil so callers can pass a projection's
// result directly.
func (r *run) collect(d *bufir.Diagnostic) {
	if d != nil {
		r.diags = append(r.diags, *d)
	}
}

// banner renders the file header, naming the .proto the schema came from.
func (r *run) banner(protoPath string) string {
	info := r.info
	info.Source = protoPath
	return provenance.Render(provenance.Hash, info)
}

// allows reports whether a target allow-list admits this target. An empty list
// admits everything, which is the common case.
func allows(list []string) bool {
	if len(list) == 0 {
		return true
	}
	for _, t := range list {
		if t == "capnp" {
			return true
		}
	}
	return false
}

// capnpPath maps a proto path onto its .capnp sibling.
func capnpPath(protoPath string) string {
	return strings.TrimSuffix(protoPath, path.Ext(protoPath)) + ".capnp"
}
