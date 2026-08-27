package protofile

// protofile_test.go covers discovery and file selection: which protos a run
// generates from, and which it merely compiles against.
//
// A mistake here produces *no* output rather than wrong output — generating from
// the wrong file set, or from none — which the golden tests cannot see, because
// they start from a request that is already correct.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIsDependencyOnly(t *testing.T) {
	// A descriptor set carries every transitive dependency, so generating from
	// all of it would emit google/protobuf/descriptor.fbs into the user's tree.
	for _, path := range []string{
		"google/protobuf/descriptor.proto",
		"google/api/annotations.proto",
		"google/rpc/status.proto",
		"google/longrunning/operations.proto",
		"buffers/v1/annotations.proto",
	} {
		if !isDependencyOnly(path) {
			t.Errorf("%s should not be generated from", path)
		}
	}

	// Anything else is the user's own, whatever it is named. Guessing that a
	// path is "not really theirs" is not this function's call — the config's
	// generate: list is how a caller narrows further.
	for _, path := range []string{
		"sensors/v1/sensors.proto",
		"googleapis_local/thing.proto",
		"my/google/thing.proto",
		"buffers/v2/mine.proto",
	} {
		if isDependencyOnly(path) {
			t.Errorf("%s is the user's own and should be generated from", path)
		}
	}
}

func TestDiscoverReturnsRootRelativePaths(t *testing.T) {
	// The path has to be root-relative because that is how an import is spelled:
	// a proto importing "sensors/v1/x.proto" resolves against the root, not the
	// working directory.
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"sensors/v1/sensors.proto": "syntax = \"proto3\";",
		"sensors/v1/enums.proto":   "syntax = \"proto3\";",
		"root.proto":               "syntax = \"proto3\";",
		"notes.md":                 "not a proto",
		"sensors/v1/README":        "also not",
	})

	got, err := discover([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"root.proto", "sensors/v1/enums.proto", "sensors/v1/sensors.proto"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("discover = %v, want %v (sorted, root-relative, .proto only)", got, want)
	}
}

func TestDiscoverAcrossRootsDoesNotDuplicate(t *testing.T) {
	// Two roots can legitimately contain the same relative path — a vendored tree
	// alongside a local one. The same path listed twice in FileToGenerate makes
	// protogen generate it twice, and for this plugin that means two writes to one
	// output file and two ledger entries for one field.
	a, b := t.TempDir(), t.TempDir()
	writeFiles(t, a, map[string]string{"shared/v1/thing.proto": "syntax = \"proto3\";"})
	writeFiles(t, b, map[string]string{"shared/v1/thing.proto": "syntax = \"proto3\";"})

	got, err := discover([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("discover across overlapping roots = %v; the same import path must appear once", got)
	}
}

func TestDiscoverMissingRootIsAnError(t *testing.T) {
	// Not silently empty: a typo'd proto path would otherwise produce a run that
	// generates nothing and reports success.
	if _, err := discover([]string{filepath.Join(t.TempDir(), "absent")}); err == nil {
		t.Error("a missing root discovered without error")
	}
}

func TestLoadRequiresASource(t *testing.T) {
	if _, err := Load(Input{}); err == nil {
		t.Error("Load with neither paths nor a descriptor set returned no error")
	}
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// descriptorSet returns the committed example set, skipping when absent.
func descriptorSet(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "examples", "descriptors.binpb")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("descriptor set missing (%v); run: buf build examples/proto -o examples/descriptors.binpb --as-file-descriptor-set", err)
	}
	return path
}
