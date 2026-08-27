package protofile

// descriptorset_test.go covers the two input paths and the Go import paths
// protogen demands of both.
//
// The go_package case is the one that matters: protogen refuses a request whose
// generated files declare none, which would make this plugin unusable on exactly
// the schemas it exists for — a .proto written for ROS has no reason to name a Go
// package.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFromDescriptorSetSelectsOnlyTheUsersFiles(t *testing.T) {
	set := descriptorSet(t)

	plugin, err := Load(Input{DescriptorSet: set})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	var generated []string
	for _, f := range plugin.Files {
		if f.Generate {
			generated = append(generated, f.Desc.Path())
		}
	}
	if len(generated) == 0 {
		t.Fatal("nothing was flagged for generation")
	}
	for _, path := range generated {
		if isDependencyOnly(path) {
			t.Errorf("%s is a dependency and should not be generate-flagged", path)
		}
	}

	// The dependencies must still be *present*, or a field whose type is declared
	// in one cannot be resolved.
	if len(plugin.Files) <= len(generated) {
		t.Error("no dependency files were carried; imported types would not resolve")
	}
}

func TestExplicitGenerateNarrowsTheSet(t *testing.T) {
	set := descriptorSet(t)
	const only = "sensors/v1/enums.proto"

	plugin, err := Load(Input{DescriptorSet: set, Generate: []string{only}})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	var generated []string
	for _, f := range plugin.Files {
		if f.Generate {
			generated = append(generated, f.Desc.Path())
		}
	}
	if !reflect.DeepEqual(generated, []string{only}) {
		t.Errorf("generate-flagged = %v, want exactly %v", generated, []string{only})
	}
}

func TestMissingDescriptorSetIsAnError(t *testing.T) {
	if _, err := Load(Input{DescriptorSet: filepath.Join(t.TempDir(), "absent.binpb")}); err == nil {
		t.Error("a missing descriptor set loaded without error")
	}
}

func TestMalformedDescriptorSetIsAnError(t *testing.T) {
	// A truncated or wrong-format file must not read as an empty set, which would
	// silently generate nothing.
	path := filepath.Join(t.TempDir(), "bad.binpb")
	if err := os.WriteFile(path, []byte("this is not a descriptor set"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(Input{DescriptorSet: path}); err == nil {
		t.Error("a malformed descriptor set loaded without error")
	}
}

func TestFromSourceCompilesAndCarriesComments(t *testing.T) {
	// Comments are most of what makes the emitted schema readable, and they only
	// survive if the compiler is asked for source info.
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"a/v1/thing.proto": `syntax = "proto3";
package a.v1;

// Thing is documented.
message Thing {
  // field is documented too.
  string field = 1;
}
`,
	})

	plugin, err := Load(Input{Paths: []string{root}})
	if err != nil {
		t.Fatalf("compile from source: %v", err)
	}
	if len(plugin.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(plugin.Files))
	}

	msg := plugin.Files[0].Messages[0]
	if got := string(msg.Comments.Leading); got == "" {
		t.Error("the message's leading comment was dropped; source info is not being requested")
	}
	if got := string(msg.Fields[0].Comments.Leading); got == "" {
		t.Error("the field's leading comment was dropped")
	}
}

func TestProtosWithoutGoPackageAreUsable(t *testing.T) {
	// The failure this guards against is total: protogen refuses the whole
	// request when a generated file declares no go_package, so a .proto written
	// for ROS or FlatBuffers — which has no reason to name a Go package — could
	// not be processed at all.
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"robot/v1/robot.proto": `syntax = "proto3";
package robot.v1;

// Pose is a position.
message Pose {
  double x = 1;
}
`,
	})

	plugin, err := Load(Input{Paths: []string{root}})
	if err != nil {
		t.Fatalf("a proto without go_package must still be processable: %v", err)
	}
	if len(plugin.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(plugin.Files))
	}

	// And the descriptor must still report the option as absent. The synthetic
	// value goes in as an M parameter precisely so it does not land here — the
	// Cap'n Proto target reads this field to decide whether it can emit a
	// $Go.package, and a synthesized one would turn its warning into a
	// confidently wrong annotation.
	if got := plugin.Files[0].Desc.Options().(interface{ GetGoPackage() string }).GetGoPackage(); got != "" {
		t.Errorf("go_package on the descriptor = %q, want empty — the synthetic path leaked", got)
	}
}

func TestSynthesizeGoPackagesLeavesDeclaredOnesAlone(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"a/v1/thing.proto": `syntax = "proto3";
package a.v1;
option go_package = "example.com/gen/av1;av1";
message Thing { string field = 1; }
`,
	})

	plugin, err := Load(Input{Paths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	got := plugin.Files[0].Desc.Options().(interface{ GetGoPackage() string }).GetGoPackage()
	if got != "example.com/gen/av1;av1" {
		t.Errorf("go_package = %q, want the declared value untouched", got)
	}
}

func TestFromSourceReportsACompileError(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"bad.proto": "syntax = \"proto3\"; message {"})

	if _, err := Load(Input{Paths: []string{root}}); err == nil {
		t.Error("a malformed proto compiled without error")
	}
}

func TestFromSourceWithNoProtosIsAnError(t *testing.T) {
	if _, err := Load(Input{Paths: []string{t.TempDir()}}); err == nil {
		t.Error("an empty proto tree loaded without error")
	}
}
