package langs

// gostage.go compiles Go output through a scratch directory so the rename that
// follows is idempotent.
//
// flatc re-emits PascalCase every run. Renaming in the output directory would
// therefore leave last run's snake_case files beside this run's PascalCase ones,
// and deduping the pair produces create_sensor_request.go alongside
// create_sensor_request2.go — a package declaring every type twice.

import (
	"fmt"
	"os"
	"path/filepath"
)

// RunWithGoRename compiles a request whose output needs Go file names fixed.
//
// The generator writes into a scratch directory, the names are corrected there,
// and the result is moved into place. The indirection is what makes a rerun
// idempotent: flatc re-emits PascalCase every time, so renaming in the output
// directory would leave last run's snake_case files beside this run's
// PascalCase ones, and the dedup would turn the pair into
// create_sensor_request.go plus create_sensor_request2.go — a package declaring
// every type twice.
func RunWithGoRename(r Request) (int, error) {
	staging, err := os.MkdirTemp("", "buffers-go-")
	if err != nil {
		return 0, fmt.Errorf("create staging directory: %w", err)
	}
	// The staging directory is under the OS temp root, so a failure to remove it
	// leaks a directory the OS will reap and has no bearing on whether the
	// generated code is correct. Reporting it would replace a successful
	// generation with an error about cleanup.
	defer func() { _ = os.RemoveAll(staging) }()

	final := r.OutDir
	r.OutDir = staging
	if err := Run(r); err != nil {
		return 0, err
	}

	renamed, err := RenameGoFiles(staging)
	if err != nil {
		return renamed, err
	}
	if err := moveTree(staging, final); err != nil {
		return renamed, err
	}
	return renamed, nil
}

// moveTree moves everything under src into dst, overwriting.
//
// Files are copied rather than renamed because the staging directory is likely on
// a different filesystem from the output tree, where os.Rename fails with
// EXDEV rather than falling back.
func moveTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o644)
	})
}
