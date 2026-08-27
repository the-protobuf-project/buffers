// Package registry wires the sources and targets one run may select from.
//
// It is named registry rather than wire, which is what the equivalent package is
// called in store, because `wire` is a target name here — the Square Wire
// backend — and a package and a target sharing a name is the sort of collision
// that reads fine until someone greps for it.
//
// The wiring lives in its own package for the reason store's does: main should
// not import every target directly. It imports this, this imports them, and the
// set of backends a build contains is one file to read rather than an import
// block to infer it from.
package registry

import (
	"github.com/the-protobuf-project/protokit/buffers"
	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/coreir"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/source/proto"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/capnp"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/flatbuffers"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/ros"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/wire"
)

// Options carries the per-run settings that are not uniform across targets.
//
// It exists for one reason today, and the reason is worth stating rather than
// generalizing: some Cap'n Proto generators refuse a schema that lacks their
// language's annotation block, and the annotation cannot simply always be
// emitted, because its `using` import is unresolvable for anyone not compiling
// that language. So the set has to be chosen per run, from the languages actually
// being produced — which the caller knows and the target does not.
type Options struct {
	// Languages are the languages this run will ultimately produce, across every
	// target. Targets read it to decide what a generator will demand of the
	// schema; none of them compile anything themselves.
	Languages []string

	// GoModule is the Go module generated Cap'n Proto code will live in. Only the
	// capnp target reads it, and only for Go output.
	GoModule string
}

// New returns the registry for one run.
//
// Everything a target needs that varies per run — where files go, what the banner
// says, which languages are downstream — is passed in here, so the targets
// themselves hold no configuration and can be constructed in a test with a sink
// that writes to a map.
func New(sink emit.Sink, opts buffers.Options, info provenance.Info, run Options) *factory.Registry[*coreir.Model] {
	reg := factory.NewRegistry[*coreir.Model]()
	reg.AddSource(proto.New(opts))
	reg.AddTarget(flatbuffers.New(sink, info))
	reg.AddTarget(capnp.New(sink, info, run.GoModule, run.Languages...))
	reg.AddTarget(ros.New(sink, info))
	reg.AddTarget(wire.New(sink, info))
	return reg
}

// TargetNames lists the registered targets, for an error message that tells the
// caller what they could have typed.
func TargetNames() string {
	return New(nil, buffers.Options{}, provenance.Info{}, Options{}).TargetNames()
}
