// Package proto is the factory Source that reads proto descriptors. It wraps
// protokit's buffers-IR walk so the rest of the factory never depends on protoc
// directly.
//
// It is also where this repository's vocabulary meets protokit's engine: the
// buffers.v1 reader is attached here rather than at the four places that build an
// Options, so no call site has to remember it.
package proto

import (
	"fmt"

	"github.com/the-protobuf-project/protokit/buffers"
	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/coreir"
	"github.com/the-protobuf-project/buffers/plugin/factory/vocab"
)

// Source builds the message graph. Options are fixed at construction; the protoc
// plugin context arrives per-run via factory.Ctx.
type Source struct {
	// opts configures the graph build. It is fixed at construction; the protoc
	// plugin context arrives per-run via factory.Ctx.
	opts buffers.Options
}

// New returns a proto Source driven by the given build options.
func New(opts buffers.Options) *Source { return &Source{opts: opts} }

// Name identifies this source in the registry and config.
func (s *Source) Name() string { return "proto" }

// Build walks the plugin's CodeGeneratorRequest into the graph.
func (s *Source) Build(ctx factory.Ctx) (*coreir.Model, error) {
	if ctx.Plugin == nil {
		return nil, fmt.Errorf("the proto source requires a protoc plugin context (only available in a buf or protoc run)")
	}
	// Copied rather than mutated: a Source is built once and may be reused, and
	// the vocabulary is a property of this repository rather than of the caller's
	// options.
	opts := s.opts
	opts.Annotations = vocab.Reader{}
	opts.Vocabulary = vocab.Spellings()

	schema, err := buffers.Build(ctx.Plugin, opts)
	if err != nil {
		return nil, err
	}
	return &coreir.Model{Schema: schema}, nil
}
