package langs

// gostage_test.go covers what makes a rerun safe: never overwriting a file that
// is already there, and replacing last run's output rather than accumulating
// beside it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRenamingNeverOverwritesAnExistingFile(t *testing.T) {
	// A directory can hold both PointCloud.go and an already-correct
	// point_cloud.go — distinct files on every filesystem, including the
	// case-insensitive ones where most name collisions cannot arise.
	//
	// Renaming in one pass hands point_cloud.go to whichever the walk reached
	// first and destroys the other: the target name is free at the moment it is
	// claimed, and os.Rename overwrites silently. This is the test that caught it.
	dir := seed(t, map[string]string{
		"PointCloud.go":  "package p // from PointCloud.go\n",
		"point_cloud.go": "package p // pre-existing\n",
	})

	if _, err := RenameGoFiles(dir); err != nil {
		t.Fatal(err)
	}

	got := tree(t, dir)
	if len(got) != 2 {
		t.Fatalf("files = %v, want 2 — a rename overwrote one", got)
	}

	// The already-correct file keeps both its name and its contents; the renamed
	// one takes a suffix rather than the occupied name.
	body, err := os.ReadFile(filepath.Join(dir, "point_cloud.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "pre-existing") {
		t.Errorf("point_cloud.go contains %q, want the pre-existing file untouched", body)
	}
}

func TestRunWithGoRenameIsIdempotent(t *testing.T) {
	// The reason renaming is staged rather than done in place. flatc re-emits
	// PascalCase every run, so a second pass over the output directory would find
	// last run's create_sensor_request.go beside this run's
	// CreateSensorRequest.go, dedup them into create_sensor_request2.go, and
	// produce a package declaring every type twice.
	out := t.TempDir()

	// Two rounds of "the generator wrote PascalCase, now fix it", which is what
	// RunWithGoRename does around the compile step.
	for range 2 {
		staging := t.TempDir()
		write(t, filepath.Join(staging, "CreateSensorRequest.go"), "package p\n\ntype CreateSensorRequest struct{}\n")
		if _, err := RenameGoFiles(staging); err != nil {
			t.Fatal(err)
		}
		if err := moveTree(staging, out); err != nil {
			t.Fatal(err)
		}
	}

	got := tree(t, out)
	want := []string{"create_sensor_request.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after two runs: %v, want %v — the second run accumulated instead of replacing", got, want)
	}
}

func TestRenameIsIdempotent(t *testing.T) {
	// `buffers generate` may run repeatedly over the same directory. A second
	// pass must not rename already-correct names into sensor2.go.
	dir := seed(t, map[string]string{"Sensor.go": "package p\n"})

	if _, err := RenameGoFiles(dir); err != nil {
		t.Fatal(err)
	}
	first := tree(t, dir)

	n, err := RenameGoFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("a second pass renamed %d files, want 0", n)
	}
	if second := tree(t, dir); !reflect.DeepEqual(first, second) {
		t.Errorf("a second pass changed the tree: %v -> %v", first, second)
	}
}

// execCommand is exec.Command, named so the test reads without importing os/exec
// at every call site.
var execCommand = exec.Command

// requireGo finds the Go toolchain or skips.
func requireGo(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH")
	}
	return path
}

// gofiles_test.go covers the PascalCase-to-snake_case rename applied to flatc's
// Go output.
//
// The case that matters is the build-constraint guard. A naive rename is not
// merely untidy — it silently removes types from the build, and the error appears
// at compile time in the consumer's project as "undefined: SensorTest", nowhere
// near the generator that caused it.
func seed(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		write(t, filepath.Join(dir, rel), body)
	}
	return dir
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// tree lists every file under dir, relative and sorted.
func tree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
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
		if rel == "go.mod" {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}
