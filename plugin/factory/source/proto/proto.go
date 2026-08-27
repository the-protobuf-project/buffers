// Package proto is the factory Source that reads proto descriptors. It wraps
// bufir's descriptor walk so the rest of the factory never depends on protoc
// directly.
package proto

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
	"github.com/the-protobuf-project/buffers/plugin/factory/coreir"
)

// Source builds the message graph. Options are fixed at construction; the protoc
// plugin context arrives per-run via factory.Ctx.
type Source struct {
	// opts configures the graph build. It is fixed at construction; the protoc
	// plugin context arrives per-run via factory.Ctx.
	opts bufir.Options
}

// New returns a proto Source driven by the given build options.
func New(opts bufir.Options) *Source { return &Source{opts: opts} }

// Name identifies this source in the registry and config.
func (s *Source) Name() string { return "proto" }

// Build walks the plugin's CodeGeneratorRequest into the graph.
func (s *Source) Build(ctx factory.Ctx) (*coreir.Model, error) {
	if ctx.Plugin == nil {
		return nil, fmt.Errorf("the proto source requires a protoc plugin context (only available in a buf or protoc run)")
	}
	schema, err := bufir.Build(ctx.Plugin, s.opts)
	if err != nil {
		return nil, err
	}
	return &coreir.Model{Schema: schema}, nil
}
