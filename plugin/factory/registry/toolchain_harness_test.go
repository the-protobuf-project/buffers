package registry_test

// toolchain_harness_test.go materializes a rendered tree on disk and locates the
// compilers, which the toolchain tests need and the golden tests do not.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireTool finds a compiler or skips the test.
func requireTool(t *testing.T, name string) string {
	t.Helper()

	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s is not on PATH; skipping the toolchain check", name)
	}
	return path
}

// writeTree materializes rendered output in a temp directory, since a compiler
// needs real files to resolve includes against.
func writeTree(t *testing.T, files map[string][]byte) string {
	t.Helper()

	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return dir
}

// schemaFiles lists the emitted schema files, relative to dir.
func schemaFiles(t *testing.T, dir, ext string) []string {
	t.Helper()

	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ext {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", dir, err)
	}
	if len(out) == 0 {
		t.Fatalf("no %s files were generated", ext)
	}
	return out
}

// firstLines truncates compiler output, which can run to hundreds of lines when a
// single type fails to resolve.
func firstLines(b []byte, n int) string {
	lines := strings.SplitN(string(b), "\n", n+1)
	if len(lines) > n {
		lines = append(lines[:n], "... (truncated)")
	}
	return strings.Join(lines, "\n")
}

// requireCapnpSchemas skips when Cap'n Proto's own schema files are absent.
//
// Every emitted .capnp opens with `using Cxx = import "/capnp/c++.capnp";`,
// which capnp resolves from its standard import path. Debian and Ubuntu split
// that away from the compiler: `capnproto` installs /usr/bin/capnp and nothing
// under /usr/include/capnp, so the import fails and every schema looks broken.
// The fix is `libcapnp-dev`.
//
// Skipping rather than failing keeps the signal honest — the schema is fine, the
// environment is incomplete — and the message names the package, because the
// compiler's own error does not.
func requireCapnpSchemas(t *testing.T) {
	t.Helper()

	for _, dir := range capnpIncludeDirs() {
		if _, err := os.Stat(filepath.Join(dir, "capnp", "c++.capnp")); err == nil {
			return
		}
	}
	t.Skip("capnp is installed but its schema files are not (no capnp/c++.capnp on the " +
		"standard import path); install libcapnp-dev on Debian/Ubuntu")
}

// capnpIncludeDirs lists the prefixes capnp searches for a rooted import.
func capnpIncludeDirs() []string {
	return []string{"/usr/include", "/usr/local/include", "/opt/homebrew/include"}
}
