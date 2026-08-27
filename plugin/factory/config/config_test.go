package config

// config_test.go covers buffers.yaml validation: the rules that catch a
// hand-edited file before a run writes anything.
//
// A rule exists to fail a bad config, so one that silently stops firing is one
// that has been removed — which no other test would notice.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write drops a config into a temp dir and returns its path.
func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const minimal = `version: v1
proto:
  paths: [proto]
out: generated
generate:
  - target: capnp
`

func TestLoadMinimal(t *testing.T) {
	cfg, err := Load(write(t, minimal))
	if err != nil {
		t.Fatalf("a minimal config should load: %v", err)
	}
	if len(cfg.Generate) != 1 || cfg.Generate[0].Target != "capnp" {
		t.Errorf("generate = %+v, want one capnp entry", cfg.Generate)
	}
}

func TestVersionIsRequiredAndChecked(t *testing.T) {
	for name, body := range map[string]string{
		"missing": strings.Replace(minimal, "version: v1\n", "", 1),
		"wrong":   strings.Replace(minimal, "version: v1", "version: v99", 1),
	} {
		if _, err := Load(write(t, body)); err == nil {
			t.Errorf("%s version: loaded without error", name)
		}
	}
}

func TestProtoSourceMustBeUnambiguous(t *testing.T) {
	// Neither: nothing to read.
	neither := `version: v1
out: generated
generate:
  - target: capnp
`
	if _, err := Load(write(t, neither)); err == nil {
		t.Error("a config naming no proto source loaded without error")
	}

	// Both: a descriptor set is already resolved, so paths would be read and
	// silently ignored — the kind of setting someone edits and cannot see take
	// effect.
	both := `version: v1
proto:
  paths: [proto]
  descriptor_set: set.binpb
out: generated
generate:
  - target: capnp
`
	err := loadErr(t, both)
	if err == "" {
		t.Fatal("a config setting both paths and descriptor_set loaded without error")
	}
	if !strings.Contains(err, "descriptor_set") {
		t.Errorf("error %q does not name the conflicting field", err)
	}
}

func TestTwoEntriesCannotClaimOneDirectory(t *testing.T) {
	// The second would overwrite the first, and the symptom — a missing file —
	// appears nowhere near the cause.
	body := `version: v1
proto:
  paths: [proto]
out: generated
generate:
  - target: capnp
    out: shared
  - target: ros
    out: shared
`
	if err := loadErr(t, body); err == "" {
		t.Error("two entries writing one directory loaded without error")
	}
}

func TestUnknownFieldIsRejected(t *testing.T) {
	// Strict decoding. A typo'd key that parsed cleanly would declare nothing and
	// look exactly like a setting that had no effect.
	body := strings.Replace(minimal, "out: generated", "out: generated\nstrictt: true", 1)
	err := loadErr(t, body)
	if err == "" {
		t.Fatal("an unknown field loaded without error")
	}
	if !strings.Contains(err, "strictt") {
		t.Errorf("error %q does not name the unknown field", err)
	}
}

func TestValidationReportsEveryProblemAtOnce(t *testing.T) {
	// A config is hand-edited and checked in a batch; one problem per run turns a
	// single fix into several round trips.
	body := `version: v99
out: generated
generate: []
`
	err := loadErr(t, body)
	for _, want := range []string{"version", "proto", "generate"} {
		if !strings.Contains(err, want) {
			t.Errorf("error does not mention %q; got:\n%s", want, err)
		}
	}
}

func TestMissingFileIsAnError(t *testing.T) {
	// Not a silently empty config: a run against nothing would emit nothing and
	// report success.
	if _, err := Load(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Error("a missing config loaded without error")
	}
}

// loadErr returns the error text from loading a config body, or "" on success.
func loadErr(t *testing.T, body string) string {
	t.Helper()
	_, err := Load(write(t, body))
	if err == nil {
		return ""
	}
	return err.Error()
}
