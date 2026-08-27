package main

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/config"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/registry"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/langs"
)

// targets.go and init.go are the two commands that exist so somebody can find out
// what this tool does without reading its source.
func targetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "targets",
		Short: "List the targets, their languages, and which toolchains are installed",
		RunE: func(cmd *cobra.Command, _ []string) error {
			reg := registry.New(nil, buffers.Options{}, provenance.Info{}, registry.Options{})

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TARGET\tSCHEMA\tTOOLCHAIN\tLANGUAGES")

			for _, name := range []string{"flatbuffers", "capnp", "ros", "wire"} {
				tgt, ok := reg.Targets[name]
				if !ok {
					continue
				}

				ext := langs.Extension(name)
				if ext == "" {
					ext = schemaKind(name)
				}

				tool, driveable, installed := langs.Available(name)
				status := "n/a — driven by the consuming build"
				switch {
				case driveable && installed:
					status = tool + " (installed)"
				case driveable:
					status = tool + " (NOT on PATH)"
				}

				// The first language is always "schema", which is not a language
				// so much as the absence of one; listing it here would be noise.
				languages := tgt.Languages()
				if len(languages) > 0 && languages[0] == "schema" {
					languages = languages[1:]
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, ext, status, strings.Join(languages, ", "))
			}
			if err := w.Flush(); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), `
ros and wire have no standalone compiler by design: rosidl generates from a .msg
during a colcon build, and Wire generates from .proto during a Gradle build. In
both cases the build system owns the invocation, so buffers emits the definitions
and the wiring and stops there.

Cap'n Proto ships only its C++ generator. Every other language is a separate
capnpc-<lang> binary you install; the list below shows which are on PATH.`)

			w = tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "\nCAPNP GENERATOR\tSTATUS\tINSTALL")
			for _, lang := range langs.CapnpLanguages() {
				binary, install, installed := langs.CapnpGenerator(lang)
				status := "not installed"
				if installed {
					status = "installed"
					install = ""
				}
				fmt.Fprintf(w, "%s (%s)\t%s\t%s\n", lang, binary, status, install)
			}
			return w.Flush()
		},
	}
}

// schemaKind describes what a target emits, for the targets listing.
func schemaKind(target string) string {
	switch target {
	case "ros":
		return ".msg/.srv"
	case "wire":
		return ".gradle.kts"
	}
	return ""
}

// starter is the config `buffers init` writes.
//
// It lives in a file rather than a Go string literal because it documents itself
// with backticked prose, and a raw string literal cannot contain a backtick — the
// alternative is concatenating fragments around each one, which makes the
// template unreadable in exactly the file where it most needs to be read.
//
//go:embed starter.yaml
var starter string

// initCmd builds the `buffers init` command.
func initCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter buffers.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := os.Stat(config.FileName); err == nil && !force {
				return fail("%s already exists; pass --force to overwrite it", config.FileName)
			}
			if err := os.WriteFile(config.FileName, []byte(starter), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", config.FileName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config")
	return cmd
}
