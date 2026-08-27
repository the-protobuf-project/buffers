package registry_test

// harness_test.go builds and renders the fixture the golden tests compare.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/registry"
	"github.com/the-protobuf-project/buffers/plugin/factory/source/protofile"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// buildSchema compiles the example protos into the message graph.
func buildSchema(t *testing.T) *buffers.Schema {
	t.Helper()

	plugin, err := protofile.Load(protofile.Input{DescriptorSet: descriptorSet(t)})
	if err != nil {
		t.Fatalf("load descriptors: %v", err)
	}

	// No lock path: each build starts from an empty ledger, so what the tests
	// assert is what derivation produces rather than what a committed file says.
	src := registry.New(nil, buffers.Options{}, info(), registry.Options{}).Sources["proto"]
	model, err := src.Build(factory.Ctx{Plugin: plugin})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return model.Schema
}

// render runs one target and returns its files.
func render(t *testing.T, target string) map[string][]byte {
	t.Helper()

	plugin, err := protofile.Load(protofile.Input{DescriptorSet: descriptorSet(t)})
	if err != nil {
		t.Fatalf("load descriptors: %v", err)
	}

	out := map[string][]byte{}
	sink := emit.Sink(func(path string, content []byte) error {
		out[path] = content
		return nil
	})

	reg := registry.New(sink, buffers.Options{}, info(), registry.Options{})
	model, err := reg.Sources["proto"].Build(factory.Ctx{Plugin: plugin})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := reg.Targets[target].Generate(factory.Ctx{Plugin: plugin}, model, ""); err != nil {
		t.Fatalf("generate %s: %v", target, err)
	}
	return out
}

// info fixes the banner's variable parts so a golden file records the schema
// rather than the version of the binary that produced it.
func info() provenance.Info {
	return provenance.Info{Version: "test", ProtocVersion: "test"}
}

// descriptorSet returns the path to the example descriptor set, skipping the test
// when it is absent.
//
// It is a checked-in artifact because the examples import googleapis, which buf
// resolves from the BSR — compiling from source here would need those .proto files
// on disk and a network fetch to get them. Regenerate with:
//
//	buf build examples/proto -o examples/descriptors.binpb --as-file-descriptor-set
func descriptorSet(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "examples", "descriptors.binpb")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("descriptor set missing (%v); run: buf build examples/proto -o examples/descriptors.binpb --as-file-descriptor-set", err)
	}
	return path
}
