package registry_test

// nullvalue_test.go covers google.protobuf.NullValue, the one well-known type
// that is an enum rather than a message.
//
// Every target substitutes the well-known messages, but an enum field took the
// ordinary path and was rendered as a reference into google/protobuf/struct,
// a file no target emits or imports. capnp then refused the schema outright
// ("Not defined: Struct"), so any proto with a null arm in a oneof — the usual
// way to say "this value may be null" — could not be mirrored.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/registry"
	"github.com/the-protobuf-project/buffers/plugin/factory/source/protofile"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

const nullableProto = `syntax = "proto3";
package nullable.v1;
import "google/protobuf/struct.proto";

// Reading is a value that may be null.
message Reading {
  // The value.
  oneof kind {
    // Explicitly no value.
    google.protobuf.NullValue null_value = 1;
    // A number.
    double number_value = 2;
  }
}
`

// renderNullable renders the nullable fixture through one target.
func renderNullable(t *testing.T, target string) map[string][]byte {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "nullable", "v1", "reading.proto")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(nullableProto), 0o644); err != nil {
		t.Fatal(err)
	}
	plugin, err := protofile.Load(protofile.Input{Paths: []string{root}})
	if err != nil {
		t.Fatalf("load: %v", err)
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

// emitted returns the rendered schema for the fixture's one file, wherever the
// target laid it out: protoc-style nullable/v1/ for most, nullable_v1/msg/ for
// ROS.
func emitted(t *testing.T, files map[string][]byte, ext string) string {
	t.Helper()
	for path, content := range files {
		if strings.HasPrefix(path, "nullable") && strings.HasSuffix(path, ext) {
			return string(content)
		}
	}
	t.Fatalf("no nullable/*%s was rendered", ext)
	return ""
}

func TestANullValueNeverReferencesStructProto(t *testing.T) {
	for target, ext := range map[string]string{
		"capnp":       ".capnp",
		"flatbuffers": ".fbs",
		"thrift":      ".thrift",
		"ros":         ".msg",
	} {
		t.Run(target, func(t *testing.T) {
			got := emitted(t, renderNullable(t, target), ext)
			// The type itself, in each target's reference syntax. A name derived
			// from the field — FlatBuffers' ReadingKindNullValue wrapper — is
			// the fixture's own and fine.
			for _, bad := range []string{"Struct.", "google.protobuf", "google/protobuf", "google_protobuf", ":NullValue", " NullValue;", "/NullValue", "struct.NullValue"} {
				if strings.Contains(got, bad) {
					t.Errorf("%s output references %q:\n%s", target, bad, got)
				}
			}
		})
	}
}

func TestCapnpRendersANullArmAsVoidAndCompiles(t *testing.T) {
	files := renderNullable(t, "capnp")
	if got := emitted(t, files, ".capnp"); !strings.Contains(got, ":Void;") {
		t.Errorf("the null arm is not Void:\n%s", got)
	}
	capnp := requireTool(t, "capnp")
	dir := writeTree(t, files)
	for _, file := range schemaFiles(t, dir, ".capnp") {
		cmd := exec.Command(capnp, "compile", "-I", dir, "-o-", file)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("capnp rejected %s: %v\n%s", file, err, out)
		}
	}
}

func TestFlatcAcceptsANullArm(t *testing.T) {
	flatc := requireTool(t, "flatc")
	dir := writeTree(t, renderNullable(t, "flatbuffers"))
	for _, file := range schemaFiles(t, dir, ".fbs") {
		cmd := exec.Command(flatc, "--cpp", "-o", t.TempDir(), "-I", dir, file)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("flatc rejected %s: %v\n%s", file, err, out)
		}
	}
}
