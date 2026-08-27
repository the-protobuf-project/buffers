package main

// output.go writes the ordinal ledger and reports diagnostics.

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
)

// writeLock persists the ordinal ledger.
//
// # Why this does not go through the protoc response
//
// It did, and that was a silent correctness bug worth describing, because the
// asymmetry that caused it is easy to reintroduce.
//
// A file returned through the CodeGeneratorResponse is written relative to the
// plugin entry's `out:` directory, which the plugin is never told. The ledger is
// *read* with os.ReadFile, which is relative to the process's working directory —
// the directory buf was invoked from. So `lock=buffers.lock` read ./buffers.lock
// and wrote <out>/buffers.lock: two different files. The ledger never round
// tripped, every run re-derived its ordinals from scratch, and the guarantee the
// whole plugin exists to provide quietly did not hold under buf. Nothing failed;
// the file was simply always in the wrong place.
//
// Writing it directly fixes the asymmetry by making both ends the same path. It
// also makes a multi-target buf setup behave: four plugin entries rendering four
// targets now converge on one ledger instead of leaving four disjoint copies, one
// per output directory, none of which is ever read.
//
// The cost is that buf does not manage this file — it is not in the response, so
// `buf generate --clean` will not remove it. That is the right trade for a lock
// file. It is an input to the next run as much as an output of this one, which is
// exactly the property that makes go.sum something you commit rather than
// something your build tool cleans.
func writeLock(schema *bufir.Schema, path string) error {
	if path == "" || schema.Lock == nil {
		return nil
	}
	body, err := schema.Lock.Marshal()
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// reportDiagnostics prints the warnings a run chose not to fail on, and fails on
// the rest.
//
// Targets add diagnostics during rendering, after bufir's own build has already
// partitioned its own, so the severity check has to run again here — a target
// that reports a `target:error` problem must still be able to fail the build.
func reportDiagnostics(p *protogen.Plugin, schema *bufir.Schema, spec string) error {
	strict, err := bufir.ParseStrict(spec)
	if err != nil {
		return err
	}
	errs, warns := strict.Partition(schema.Diags)
	for _, d := range warns {
		fmt.Fprintf(os.Stderr, "protoc-gen-buffers: warning: %s\n", d)
	}
	if len(errs) == 0 {
		return nil
	}
	for _, d := range errs {
		fmt.Fprintf(os.Stderr, "protoc-gen-buffers: %s\n", d)
	}
	return fmt.Errorf("%d schema problem(s) — see above", len(errs))
}
