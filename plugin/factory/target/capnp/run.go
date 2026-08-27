package capnp

// run.go holds one Generate call's mutable state, and renders one file.
//
// The state is per-run rather than on the Target so that a Target can be shared:
// nothing it holds survives a Generate, and two concurrent renders would
// otherwise interleave their alias tables and their prelude bookkeeping.

import (
	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// run is one Generate call's mutable state.
type run struct {
	*Target
	// schema is the graph being rendered.
	schema *buffers.Schema

	// needed and fileNeeds track substituted well-known records for the run and
	// for the file currently rendering. See the FlatBuffers target for why the
	// body is rendered before the header.
	needed map[preludeType]bool
	// fileNeeds tracks substitutions for the file currently rendering, which decides
	// whether that file imports the prelude at all.
	fileNeeds map[preludeType]bool

	// aliases maps a proto path to the `using` name the current file binds it to.
	// Cap'n Proto has no implicit cross-file scope: a type in another file is
	// only reachable through an alias the importing file declares.
	aliases map[string]string

	// sinks collects the callback interfaces streaming methods need, which are
	// discovered while rendering a service and emitted after it.
	sinks []sinkIface

	// annotations is the normalized set of languages needing annotation blocks.
	annotations map[string]bool

	// goModule is copied from the target so annotations.go can reach it.
	goModule string

	// diags accumulates problems found while projecting types.
	diags []buffers.Diagnostic
}

// sinkIface is a generated callback interface for a server-streaming method.
type sinkIface struct {
	// Name is the generated interface's name.
	Name string
	// ID is its derived Cap'n Proto type ID.
	ID uint64
	// Element is the type each pushed value carries.
	Element string
	// Method is the streaming method it was generated for.
	Method string
	// Doc is that method's leading comment, as prose.
	Doc string
}

// file renders one .capnp, or nil when the file has nothing this target emits.
func (r *run) file(f *buffers.File) ([]byte, error) {
	msgs := topLevel(f)
	enums := topLevelEnums(f)
	svcs := emittableServices(f)
	if len(msgs) == 0 && len(enums) == 0 && len(svcs) == 0 {
		return nil, nil
	}

	r.fileNeeds = map[preludeType]bool{}
	r.aliases = r.assignAliases(f)
	r.sinks = nil
	r.warnEnumOnlyGo(f, msgs, enums, svcs)

	var b emit.Buf
	for _, e := range enums {
		b.Line("")
		r.enum(&b, e)
	}
	for _, m := range msgs {
		b.Line("")
		r.structOf(&b, f, m)
	}
	for _, s := range svcs {
		b.Line("")
		r.iface(&b, f, s)
	}
	for _, s := range r.sinks {
		b.Line("")
		r.sinkInterface(&b, s)
	}

	// The header is assembled last: which files this one imports depends on which
	// substitutions the body reached for.
	var head emit.Buf
	head.Linef("@0x%016x;", f.CapnpID)
	head.Line("")
	head.Raw(r.banner(f.Path))
	head.Line("")
	head.Line(`using Cxx = import "/capnp/c++.capnp";`)
	head.Linef("$Cxx.namespace(%q);", cxxNamespace(f.Namespace))
	r.annotationHeader(&head, f)

	if using := r.usings(f); len(using) > 0 {
		head.Line("")
		for _, u := range using {
			head.Line(u)
		}
	}
	head.Raw(b.String())
	return head.Bytes(), nil
}
