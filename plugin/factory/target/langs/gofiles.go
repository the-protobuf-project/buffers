package langs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/the-protobuf-project/protokit/naming"
)

// goFileGuard is appended to a name whose trailing word would otherwise be read
// as a build constraint: SensorTest becomes sensor_test_fbs.go.
//
// "fbs" rather than something generic, so that a reader who finds the odd name
// can tell which generator produced it and why.
const goFileGuard = "fbs"

// RenameGoFiles rewrites PascalCase .go file names under dir to snake_case,
// reporting how many it renamed.
//
// It is deliberately narrow: it renames files and reads none of them. The
// contents are flatc's business, and a rename that also rewrote source would be a
// much larger claim than this needs to make.
func RenameGoFiles(dir string) (int, error) {
	// Files are collected and sorted before anything moves, for two reasons. A
	// rename during a walk can have the walk visit the new name; and the dedup
	// suffix GoFileName assigns depends on the order names are seen, so an
	// unsorted walk would produce sensor2.go on some runs and not others.
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".go" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("scan %s: %w", dir, err)
	}
	sort.Strings(paths)

	// Names are deduped only against each other, never against what is already on
	// disk.
	//
	// Both halves matter. Two files in one batch can snake to the same name — on a
	// case-sensitive filesystem SensorID.go and SensorId.go both want
	// sensor_id.go — and the second must take a suffix rather than destroy the
	// first. But a file already sitting in the output directory is last run's
	// output, and *must* be overwritten: reserving it instead makes a rerun
	// accumulate create_sensor_request2.go, create_sensor_request3.go, and a
	// package that no longer compiles because every type is declared twice.
	//
	// That is only safe because callers rename inside a directory this run
	// populated, never in place. See RunWithGoRename.
	used := map[string]map[string]bool{}
	reserve := func(dir, name string) {
		if used[dir] == nil {
			used[dir] = map[string]bool{}
		}
		used[dir][name] = true
	}

	type pending struct{ path, dir, base string }
	var todo []pending

	for _, path := range paths {
		dir, base := filepath.Split(path)
		if base == correctName(base, nil) {
			// Already correct. Reserved — without the extension, since that is the
			// key protokit's Unique compares — so a later file in this same batch
			// does not rename onto it, and skipped so the pass is a no-op on an
			// already-converted tree.
			reserve(dir, strings.TrimSuffix(base, ".go"))
			continue
		}
		todo = append(todo, pending{path: path, dir: dir, base: base})
	}

	renamed := 0
	for _, p := range todo {
		if used[p.dir] == nil {
			used[p.dir] = map[string]bool{}
		}
		want := correctName(p.base, used[p.dir])

		target := filepath.Join(p.dir, want)
		// A case-only rename is a no-op on a case-insensitive filesystem and a
		// real move elsewhere, so it goes through a temporary name. Without this,
		// macOS reports success and leaves Sensor.go in place.
		tmp := filepath.Join(p.dir, "."+want+".tmp")
		if err := os.Rename(p.path, tmp); err != nil {
			return renamed, fmt.Errorf("rename %s: %w", p.path, err)
		}
		if err := os.Rename(tmp, target); err != nil {
			return renamed, fmt.Errorf("rename %s to %s: %w", tmp, target, err)
		}
		renamed++
	}
	return renamed, nil
}

// correctName returns the Go file name a generated file should have.
//
// Passing a nil used map asks the question "is this name already correct?"
// without reserving anything, which is how the first pass distinguishes a file
// that needs renaming from one that does not.
func correctName(base string, used map[string]bool) string {
	if used == nil {
		used = map[string]bool{}
	}
	return naming.GoFileName(strings.TrimSuffix(base, ".go"), goFileGuard, used)
}

// GoFilesNeedRenaming reports whether a target and language produce PascalCase Go
// file names.
//
// Only flatc does. capnpc-go already names its output after the schema file —
// sensors.capnp.go — which is lowercase and needs nothing.
func GoFilesNeedRenaming(target, lang string) bool {
	return target == "flatbuffers" && lang == "go"
}
