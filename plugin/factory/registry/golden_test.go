package registry_test

// golden_test.go renders the example protos through every target and compares the
// result byte for byte.
//
// The goldens are the real deliverable of this repository, so they are checked in
// and reviewed: a diff here is a diff in what every downstream consumer compiles
// against. `go test ./... -update` rewrites them.
//
// The inputs are the examples rather than a private fixture tree, deliberately. A
// fixture only the tests use drifts from the thing users copy, and the examples
// are already the most-read demonstration of what the plugin does.

import (
	"bytes"
	"flag"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// targets is every target, rendered in a stable order so a failure names the same
// subtest each run.
var targets = []string{"capnp", "flatbuffers", "ros", "thrift", "wire"}

func TestGolden(t *testing.T) {
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			got := render(t, target)
			dir := filepath.Join("testdata", "golden", target)

			if *update {
				rewrite(t, dir, got)
				return
			}
			compare(t, dir, got)
		})
	}
}

// TestDeterminism renders twice and compares, which catches the bug a committed
// golden file cannot: a map ranged into output.
//
// A golden test passes as long as the first render matches what was recorded, and
// a target that iterated a map would produce a different order on some runs and
// the same order on others. This fails reliably where that fails intermittently.
func TestDeterminism(t *testing.T) {
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			first, second := render(t, target), render(t, target)

			if len(first) != len(second) {
				t.Fatalf("two runs produced %d and %d files", len(first), len(second))
			}
			for path, a := range first {
				b, ok := second[path]
				if !ok {
					t.Errorf("%s: present in the first run, absent in the second", path)
					continue
				}
				if !bytes.Equal(a, b) {
					t.Errorf("%s: two renders of one schema differ — something iterates a map", path)
				}
			}
		})
	}
}
