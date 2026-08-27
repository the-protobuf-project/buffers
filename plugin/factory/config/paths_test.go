package config

// paths_test.go covers path resolution, which decides where a run's output lands.
//
// Getting it wrong writes a tree somewhere the user did not ask for, and nothing
// else in the suite would see that.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathsResolveAgainstTheConfigFile(t *testing.T) {
	// So that `buffers generate -c sub/buffers.yaml` from a repository root
	// behaves the same as running it from sub/.
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, FileName)
	body := `version: v1
proto:
  paths: [proto]
  imports: [vendor]
  descriptor_set: ""
out: generated
lock: generated/buffers.lock
generate:
  - target: capnp
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, tc := range []struct{ got, want string }{
		{cfg.Proto.Paths[0], filepath.Join(sub, "proto")},
		{cfg.Proto.Imports[0], filepath.Join(sub, "vendor")},
		{cfg.Out, filepath.Join(sub, "generated")},
		{cfg.Lock, filepath.Join(sub, "generated", "buffers.lock")},
	} {
		if tc.got != tc.want {
			t.Errorf("resolved %q, want %q", tc.got, tc.want)
		}
	}
}

func TestEmptyLockStaysEmpty(t *testing.T) {
	// An empty lock disables the ledger. Joining it against the config directory
	// would turn "disabled" into a path, and the run would write a ledger the
	// author asked not to have.
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, FileName)
	if err := os.WriteFile(path, []byte(minimal), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lock != "" {
		t.Errorf("Lock = %q, want empty — an unset ledger must stay unset", cfg.Lock)
	}
}

func TestOmittedOutMeansTheConfigDirectory(t *testing.T) {
	// Legal, and the useful default: a bare buffers.yaml beside the protos writes
	// <target>/ next to itself. Pinned because it is a behaviour rather than an
	// oversight — validation deliberately does not require `out`.
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, FileName)
	body := strings.Replace(minimal, "out: generated\n", "", 1)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("a config without `out` should load: %v", err)
	}
	if cfg.Out != sub {
		t.Errorf("Out = %q, want the config's own directory %q", cfg.Out, sub)
	}
	if got, want := cfg.OutDir(cfg.Generate[0]), filepath.Join(sub, "capnp"); got != want {
		t.Errorf("OutDir = %q, want %q", got, want)
	}
}

func TestOutputDirectories(t *testing.T) {
	cfg := &Config{Out: "gen"}

	// Defaults to the target name, which is what keeps two entries apart.
	if got := cfg.OutDir(Entry{Target: "capnp"}); got != filepath.Join("gen", "capnp") {
		t.Errorf("OutDir = %q, want gen/capnp", got)
	}
	if got := cfg.OutDir(Entry{Target: "capnp", Out: "schema"}); got != filepath.Join("gen", "schema") {
		t.Errorf("OutDir with an override = %q, want gen/schema", got)
	}

	// Compiled language output is separate from the schema, so it cannot land in
	// the same directory and be mistaken for it.
	if got := cfg.LangDir(Entry{Target: "capnp"}, "go"); got != filepath.Join("gen", "capnp-go") {
		t.Errorf("LangDir = %q, want gen/capnp-go", got)
	}
	if got := cfg.LangDir(Entry{Target: "capnp", LangOut: "_lang/capnp"}, "go"); got != filepath.Join("gen", "_lang", "capnp", "go") {
		t.Errorf("LangDir with an override = %q, want gen/_lang/capnp/go", got)
	}
}

func TestModuleForPrefersTheEntry(t *testing.T) {
	// Each target's Go output lives in its own directory, so one module root
	// cannot describe both and the per-entry value has to win.
	cfg := &Config{GoModule: "example.com/base"}

	if got := cfg.ModuleFor(Entry{Target: "capnp"}); got != "example.com/base" {
		t.Errorf("ModuleFor without an override = %q, want the top-level value", got)
	}
	if got := cfg.ModuleFor(Entry{Target: "capnp", GoModule: "example.com/capnp"}); got != "example.com/capnp" {
		t.Errorf("ModuleFor with an override = %q, want the entry value", got)
	}
}
