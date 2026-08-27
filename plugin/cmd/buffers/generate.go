package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/the-protobuf-project/protokit/buffers"
	"github.com/the-protobuf-project/protokit/factory"
	"github.com/the-protobuf-project/protokit/header"

	"github.com/the-protobuf-project/buffers/plugin/factory/config"
	"github.com/the-protobuf-project/buffers/plugin/factory/registry"
	"github.com/the-protobuf-project/buffers/plugin/factory/source/protofile"
)

// generateCmd builds the `buffers generate` command.
func generateCmd() *cobra.Command {
	var (
		cfgPath string
		targets []string
		lang    string
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Render every configured target from the protos",
		Long: `Compiles the protos, renders each target in buffers.yaml, and writes the ordinal
ledger.

With --lang, also runs the target's compiler (flatc, capnp) over the emitted
schema. Without it, schema is emitted and nothing is invoked.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			return generate(cmd, cfg, targets, lang, dryRun)
		},
	}

	cmd.Flags().StringVarP(&cfgPath, "config", "c", config.FileName, "path to buffers.yaml")
	cmd.Flags().StringSliceVarP(&targets, "target", "t", nil,
		"render only these targets (default: every entry in the config)")
	cmd.Flags().StringVar(&lang, "lang", "",
		"also compile the emitted schema into this language, overriding the config's languages")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"report what would be written without writing it")
	return cmd
}

// generate runs the whole pipeline.
func generate(cmd *cobra.Command, cfg *config.Config, only []string, lang string, dryRun bool) error {
	header.SetTool("protoc-gen-buffers")
	header.SetProject("https://github.com/the-protobuf-project/buffers")

	entries, err := selectEntries(cfg, only)
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

	opts := buffers.Options{Strict: cfg.Strict, LockPath: cfg.Lock}
	info := provenanceInfo()

	// The model is built once and rendered by every target. Rebuilding per target
	// would re-read the ledger each time and let two targets disagree about a slot
	// — which is the one thing the ledger exists to prevent.
	src := registry.New(nil, opts, info, registry.Options{}).Sources["proto"]
	model, err := src.Build(factory.Ctx{Plugin: plugin})
	if err != nil {
		return err
	}

	var written int
	for _, e := range entries {
		dir := cfg.OutDir(e)
		sink, count := diskSink(dir, dryRun, cmd)

		// The whole language list for this entry, not just the one being
		// rendered for: a target has to know everything its schema will later be
		// compiled into, because a generator that needs an annotation needs it
		// present in the one schema that gets written.
		reg := registry.New(sink, opts, info, registry.Options{
			Languages: entryLanguages(e, lang),
			GoModule:  cfg.ModuleFor(e),
		})
		tgt, ok := reg.Targets[e.Target]
		if !ok {
			return fail("unknown target %q — valid targets: %s", e.Target, reg.TargetNames())
		}
		if err := tgt.Generate(factory.Ctx{Plugin: plugin}, model, renderLanguage(e, lang)); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-12s -> %s (%d files)\n", e.Target, dir, *count)
		written += *count
	}

	if err := writeLock(cmd, model.Schema, cfg.Lock, dryRun); err != nil {
		return err
	}
	if err := report(cmd, model.Schema, cfg.Strict); err != nil {
		return err
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "\ndry run: %d files would be written\n", written)
		return nil
	}
	return compile(cmd, cfg, entries, lang, model)
}

// entryLanguages returns every language an entry will ultimately produce.
func entryLanguages(e config.Entry, override string) []string {
	if override != "" {
		return []string{override}
	}
	return e.Languages
}

// renderLanguage decides what language string a target renders for.
//
// Most targets emit one schema regardless — the language only decides what a
// compiler does with it afterwards. The Wire target is the exception: its output
// *is* build configuration, so a Kotlin run and a Java run differ.
func renderLanguage(e config.Entry, override string) string {
	if override != "" {
		return override
	}
	if len(e.Languages) > 0 {
		return e.Languages[0]
	}
	return ""
}

// selectEntries narrows the config's entries to those named by --target.
func selectEntries(cfg *config.Config, only []string) ([]config.Entry, error) {
	if len(only) == 0 {
		return cfg.Generate, nil
	}
	want := map[string]bool{}
	for _, t := range only {
		want[t] = true
	}

	var out []config.Entry
	for _, e := range cfg.Generate {
		if want[e.Target] {
			out = append(out, e)
			delete(want, e.Target)
		}
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for t := range want {
			missing = append(missing, t)
		}
		sort.Strings(missing)
		return nil, fail("no generate entry for target(s): %s", strings.Join(missing, ", "))
	}
	return out, nil
}
