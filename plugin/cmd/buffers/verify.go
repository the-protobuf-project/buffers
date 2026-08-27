package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/the-protobuf-project/protokit/buffers"
	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/config"
	"github.com/the-protobuf-project/buffers/plugin/factory/registry"
	"github.com/the-protobuf-project/buffers/plugin/factory/source/protofile"
)

// verify.go is the CI command: rebuild the ledger from the current protos and
// fail if it differs from the committed one.
//
// This is what makes buffers.lock load-bearing rather than decorative. The plugin
// already reports a slot that moved, but a report is only as good as whoever
// reads the output — and a wire break is exactly the thing that gets scrolled
// past. `buffers verify` in CI turns it into a failed build.
func verifyCmd() *cobra.Command {
	var cfgPath string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Fail if regenerating would move a field's target slot",
		Long: `Rebuilds the ordinal ledger from the current protos and compares it to the
committed buffers.lock.

Run it in CI. A field's slot in an emitted schema is a wire format: a consumer
compiled against a schema where a field sat at ordinal 5 reads slot 5 forever,
and if a rebuild puts something else there, nothing — not protoc, not capnp, not
flatc, not the consumer — reports an error. It reads the wrong field.

Exits non-zero when the ledger would change, and prints what moved.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			return verify(cmd, cfg)
		},
	}

	cmd.Flags().StringVarP(&cfgPath, "config", "c", config.FileName, "path to buffers.yaml")
	return cmd
}

// verify compares the committed ledger against what the current protos would
// produce, and reports what moved.
func verify(cmd *cobra.Command, cfg *config.Config) error {
	if cfg.Lock == "" {
		return fail("this config disables the ordinal ledger (lock: is empty), so there is nothing to verify.\n" +
			"    Set `lock: buffers.lock` and commit the file to make slot stability checkable")
	}

	committed, err := os.ReadFile(cfg.Lock)
	if os.IsNotExist(err) {
		return fail("%s does not exist yet — run `buffers generate` and commit it", cfg.Lock)
	}
	if err != nil {
		return err
	}
	before, err := buffers.ParseLock(committed, cfg.Lock)
	if err != nil {
		return err
	}

	plugin, err := protofile.Load(protofile.Input{
		Paths:         cfg.Proto.Paths,
		Imports:       cfg.Proto.Imports,
		DescriptorSet: cfg.Proto.DescriptorSet,
		Generate:      cfg.Proto.Generate,
	})
	if err != nil {
		return err
	}

	// The build reads the committed ledger, so a slot it records wins over
	// derivation and the rebuilt ledger matches. That is the point: what this
	// compares is whether the *set* of recorded slots changed — a field added or
	// removed — not whether derivation alone would have agreed.
	src := registry.New(nil, buffers.Options{Strict: cfg.Strict, LockPath: cfg.Lock},
		provenanceInfo(), registry.Options{}).Sources["proto"]
	model, err := src.Build(factory.Ctx{Plugin: plugin})
	if err != nil {
		return err
	}

	if model.Schema.Lock.Equal(before) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s is up to date\n", cfg.Lock)
		return nil
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "%s is out of date.\n\n", cfg.Lock)
	for _, line := range diffLocks(before, model.Schema.Lock) {
		fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", line)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "\nRun `buffers generate` and commit the result.\n")
	return fail("the ordinal ledger would change")
}

// diffLocks describes what changed, in terms a reviewer can act on.
//
// It reports slots, not YAML: "sensors.v1.Sensor.rate_hz moved from ordinal 5 to
// 6" is the sentence someone needs, and a textual diff of the file buries it
// among the name fields that are only commentary.
//
// The distinction that matters most is between a removal that moved other fields
// and one that did not. Both change the ledger and both require a regenerate, but
// only the first is a wire break — the second means the author did reserve the
// slot, and telling them to reserve it would be wrong.
func diffLocks(before, after *buffers.Lock) []string {
	var out []string

	beforeFields := map[buffers.NodeID]map[int32]int32{}
	for _, m := range before.Messages {
		slots := map[int32]int32{}
		for _, f := range m.Fields {
			slots[f.Number] = f.Ordinal
		}
		beforeFields[m.Node] = slots
	}

	for _, m := range after.Messages {
		old, existed := beforeFields[m.Node]
		if !existed {
			out = append(out, fmt.Sprintf("+ %s is new (%d fields)", m.Node, len(m.Fields)))
			continue
		}

		var added, moved, gone []string
		for _, f := range m.Fields {
			was, had := old[f.Number]
			switch {
			case !had:
				added = append(added, fmt.Sprintf("+ %s.%s (proto field %d) takes ordinal %d",
					m.Node, f.Name, f.Number, f.Ordinal))
			case was != f.Ordinal:
				moved = append(moved, fmt.Sprintf("! %s.%s MOVED from ordinal %d to %d",
					m.Node, f.Name, was, f.Ordinal))
			}
			delete(old, f.Number)
		}

		numbers := make([]int32, 0, len(old))
		for number := range old {
			numbers = append(numbers, number)
		}
		sort.Slice(numbers, func(i, j int) bool { return numbers[i] < numbers[j] })
		for _, number := range numbers {
			gone = append(gone, fmt.Sprintf("- %s: proto field %d (ordinal %d) is gone",
				m.Node, number, old[number]))
		}

		out = append(out, gone...)
		out = append(out, added...)
		out = append(out, moved...)

		// The verdict, phrased from what actually happened. A removal that moved
		// nothing means the slot was held — by a `reserved` declaration, or by the
		// ledger itself — and the only action needed is to commit the result.
		switch {
		case len(moved) > 0:
			out = append(out, fmt.Sprintf("  ^ %s: this is a WIRE BREAK. Every consumer compiled against the "+
				"old schema now reads the wrong field. Add a `reserved` declaration for the removed "+
				"number(s) and regenerate.", m.Node))
		case len(gone) > 0:
			out = append(out, fmt.Sprintf("  ^ %s: no other field moved, so the slot is held and existing "+
				"consumers are unaffected. Regenerate and commit.", m.Node))
		}
		delete(beforeFields, m.Node)
	}

	nodes := make([]buffers.NodeID, 0, len(beforeFields))
	for node := range beforeFields {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	for _, node := range nodes {
		out = append(out, fmt.Sprintf("- %s is gone", node))
	}
	return out
}
