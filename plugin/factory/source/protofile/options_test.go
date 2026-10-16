package protofile

// options_test.go covers the options a schema declares with someone else's
// extensions — google.api.resource above all — on the from-source path.
//
// protocompile interprets such an option into a dynamic message built from the
// compiled .proto. protoc and buf hand a plugin bytes instead, which parse into
// the generated Go types, and protokit's readers expect those. Given the dynamic
// form they panicked converting one into the other, so any schema with AIP
// annotations and googleapis among its imports crashed `buffers generate`.

import (
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
)

// googleAPIResource is google/api/resource.proto cut down to the fields read
// here, with the real extension and field numbers. Tests cannot fetch
// googleapis, and the numbers are all the wire format sees.
const googleAPIResource = `syntax = "proto3";
package google.api;
import "google/protobuf/descriptor.proto";
option go_package = "google.golang.org/genproto/googleapis/api/annotations;annotations";
extend google.protobuf.MessageOptions {
  ResourceDescriptor resource = 1053;
}
message ResourceDescriptor {
  string type = 1;
  repeated string pattern = 2;
  string plural = 5;
  string singular = 6;
}
`

func TestAnAnnotationFromAnImportedProtoIsReadNotPanickedOn(t *testing.T) {
	deps, root := t.TempDir(), t.TempDir()
	writeFiles(t, deps, map[string]string{"google/api/resource.proto": googleAPIResource})
	writeFiles(t, root, map[string]string{
		"a/v1/thing.proto": `syntax = "proto3";
package a.v1;
import "google/api/resource.proto";

// Thing is a resource.
message Thing {
  option (google.api.resource) = {
    type: "a.example.com/Thing"
    pattern: "things/{thing}"
    singular: "thing"
    plural: "things"
  };
  string name = 1;
}
`,
	})

	plugin, err := Load(Input{Paths: []string{root}, Imports: []string{deps}})
	if err != nil {
		t.Fatalf("compile from source: %v", err)
	}
	schema, err := buffers.Build(plugin, buffers.Options{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var got *buffers.Resource
	for _, m := range schema.Messages {
		if m.Name == "Thing" {
			got = m.Resource
		}
	}
	if got == nil {
		t.Fatal("Thing's google.api.resource was not read")
	}
	if got.Type != "a.example.com/Thing" || got.Singular != "thing" {
		t.Errorf("read %+v, want type a.example.com/Thing, singular thing", got)
	}
}

func TestAWellKnownTypeNeedsNoImportRoot(t *testing.T) {
	// The well-known types arrive through protocompile's standard imports, not
	// from disk, and once went missing from the request: protogen then refused
	// the whole run for an import it could not resolve.
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"a/v1/reading.proto": `syntax = "proto3";
package a.v1;
import "google/protobuf/timestamp.proto";

// Reading has a time.
message Reading {
  google.protobuf.Timestamp read_time = 1;
}
`,
	})
	plugin, err := Load(Input{Paths: []string{root}})
	if err != nil {
		t.Fatalf("compile from source: %v", err)
	}
	if len(plugin.Files) < 2 {
		t.Fatalf("got %d files; the timestamp.proto dependency is missing", len(plugin.Files))
	}
}
