// Package coreir is the factory's shared model: a Source builds one, a Target
// renders from it.
//
// It is a thin wrapper around a single facet today, where store's equivalent
// carries two. That is not an oversight — it is what the seam is for. store has a
// proto/DB world and a GraphQL world that genuinely do not share a
// representation, so its Model carries whichever the Source populated. buffers
// has one input shape, and pretending otherwise by flattening some future second
// source into bufir.Schema now would be inventing a god-IR before there is a
// second source to disagree with it.
//
// What the wrapper buys in the meantime is the factory's orchestration: one
// registry, one language axis, one config, and a place to put the second facet
// when an eCAL or a live-schema source arrives.
package coreir

import "github.com/the-protobuf-project/buffers/plugin/factory/bufir"

// Model is the unit of work handed from a Source to a Target.
type Model struct {
	// Schema is the proto-derived message graph: files, messages, fields, enums
	// and services, with every field pinned to a stable target slot.
	//
	// The whole graph travels rather than its files alone, because a target needs
	// the indexes to resolve anything: a field's type is a full proto name, and
	// turning that into a Cap'n Proto type reference means looking it up.
	Schema *bufir.Schema
}
