package registry_test

// compare_test.go compares a rendered tree against the recorded goldens, and
// rewrites them under -update.
//
// A failure names the first differing line rather than dumping two files, because
// a golden diff is read by someone deciding whether a wire format just changed.

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// compare checks rendered output against the recorded goldens.
func compare(t *testing.T, dir string, got map[string][]byte) {
	t.Helper()

	want, err := readTree(dir)
	if err != nil {
		t.Fatalf("read goldens from %s: %v\n    run: go test ./... -update", dir, err)
	}
	if len(want) == 0 {
		t.Fatalf("no goldens in %s\n    run: go test ./... -update", dir)
	}

	for _, path := range sortedKeys(got) {
		expected, ok := want[path]
		if !ok {
			t.Errorf("%s is newly generated and has no golden\n    run: go test ./... -update", path)
			continue
		}
		if !bytes.Equal(got[path], expected) {
			t.Errorf("%s differs from its golden\n%s", path, firstDifference(expected, got[path]))
		}
	}
	for _, path := range sortedKeys(want) {
		if _, ok := got[path]; !ok {
			t.Errorf("%s has a golden but is no longer generated\n    run: go test ./... -update", path)
		}
	}
}

// rewrite replaces a golden tree with what was just rendered.
func rewrite(t *testing.T, dir string, got map[string][]byte) {
	t.Helper()

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("clear %s: %v", dir, err)
	}
	for path, content := range got {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	t.Logf("rewrote %d files in %s", len(got), dir)
}

// readTree reads a golden directory into the same shape render returns.
func readTree(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = content
		return nil
	})
	return out, err
}

// firstDifference locates the first differing line, so a failure points at a line
// rather than dumping two files.
func firstDifference(want, got []byte) string {
	wantLines := bytes.Split(want, []byte("\n"))
	gotLines := bytes.Split(got, []byte("\n"))

	for i := range max(len(wantLines), len(gotLines)) {
		var w, g []byte
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if !bytes.Equal(w, g) {
			return "    line " + itoa(i+1) + ":\n      want: " + string(w) + "\n      got:  " + string(g)
		}
	}
	return "    (files differ only in trailing bytes)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
