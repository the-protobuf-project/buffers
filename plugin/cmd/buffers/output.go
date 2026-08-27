package main

// output.go writes what a run produces: the rendered files, the ordinal ledger,
// and the diagnostics.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// diskSink returns a sink that writes beneath dir, and a counter of what it
// wrote.
func diskSink(dir string, dryRun bool, cmd *cobra.Command) (emit.Sink, *int) {
	count := 0
	return func(rel string, content []byte) error {
		count++
		path := filepath.Join(dir, rel)
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "  would write %s (%d bytes)\n", path, len(content))
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		return os.WriteFile(path, content, 0o644)
	}, &count
}

// writeLock persists the ordinal ledger.
//
// It is written once for the whole run rather than once per target, because there
// is one ledger and every target read the same model to produce it. Writing it
// per target would have the last one win and make the file's contents depend on
// config ordering.
func writeLock(cmd *cobra.Command, schema *buffers.Schema, path string, dryRun bool) error {
	if path == "" || schema.Lock == nil {
		return nil
	}
	body, err := schema.Lock.Marshal()
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "  would write %s (%d bytes)\n", path, len(body))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// report prints diagnostics and fails on the ones the strictness spec makes
// errors.
func report(cmd *cobra.Command, schema *buffers.Schema, spec string) error {
	strict, err := buffers.ParseStrict(spec)
	if err != nil {
		return err
	}
	errs, warns := strict.Partition(schema.Diags)
	for _, d := range warns {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", d)
	}
	for _, d := range errs {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", d)
	}
	if len(errs) > 0 {
		return fail("%d schema problem(s) — see above", len(errs))
	}
	return nil
}
