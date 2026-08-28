package main

// doctor.go answers one question: what is missing on this machine, and what do I
// type to fix it.
//
// It exists because the alternative is finding out one failure at a time, in the
// middle of a generate, with a message about the first tool that happened to be
// reached for. A schema render needs nothing installed at all; a `--lang` run
// needs a different compiler per target and, for Cap'n Proto, a different plugin
// per language on top of that. Checking all of it up front costs a few exec
// lookups and turns a sequence of interruptions into one list.
//
// What it deliberately does not do is install anything. Reaching for a package
// manager on somebody's behalf is not a code generator's business, and a wrong
// guess about which one — or about sudo — is worse than a line they can read
// before running.

import (
	"fmt"
	"os/exec"
	"runtime"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/langs"
)

// doctorCmd builds the `buffers doctor` command.
func doctorCmd() *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check which schema compilers are installed, and print how to get the rest",
		Long: `Reports every external compiler buffers can drive, whether it is on this
machine, and the exact command to install the ones that are not — chosen for the
package manager you actually have.

Nothing here is required to emit schema. buffers renders .fbs, .capnp, .thrift
and .msg with no toolchain at all; these are only needed when a generate asks
for a language.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			missing := reportTools(cmd)
			missing += reportCapnpPlugins(cmd)

			if missing == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nEverything buffers can drive is installed.")
				return nil
			}
			if strict {
				return fail("%d compiler(s) missing", missing)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false,
		"exit non-zero when anything is missing, for a CI preflight")
	return cmd
}

// reportTools prints one row per target compiler and returns how many are absent.
func reportTools(cmd *cobra.Command) int {
	fmt.Fprintf(cmd.OutOrStdout(), "platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	if m := langs.Manager(); m != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "   package manager: %s", m)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nTARGET\tCOMPILER\tSTATUS")

	var missing []string
	for _, target := range langs.Targets() {
		tool, _, installed := langs.Available(target)
		status := "NOT INSTALLED"
		if installed {
			status = "ok"
			if v := langs.Version(target); v != "" {
				status = "ok — " + v
			}
		} else {
			missing = append(missing, target)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", target, tool, status)
	}
	fmt.Fprintf(w, "%s\t%s\t%s\n", "ros", "—", "driven by colcon in the consuming build")
	fmt.Fprintf(w, "%s\t%s\t%s\n", "wire", "—", "driven by Gradle in the consuming build")
	if err := w.Flush(); err != nil {
		return len(missing)
	}

	for _, target := range missing {
		in, ok := langs.InstallFor(target)
		if !ok {
			continue
		}
		binary, _, _ := langs.Available(target)
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s (%s):\n%s\n", target, binary, in.Detail("  "))
	}
	return len(missing)
}

// reportCapnpPlugins prints the per-language Cap'n Proto generators and returns
// how many are absent.
//
// They are a separate list because they are a separate kind of thing: capnp ships
// only its C++ backend, so every other language is an executable installed from
// that language's own ecosystem — go install, cargo, pip, npm — and not from the
// system package manager the table above is about.
func reportCapnpPlugins(cmd *cobra.Command) int {
	if _, err := exec.LookPath("capnp"); err != nil {
		// Without capnp itself the plugin list is noise; the row above already
		// said what to do.
		return 0
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nCAPNP GENERATOR\tBINARY\tSTATUS\tINSTALL")

	missing := 0
	for _, lang := range langs.CapnpLanguages() {
		binary, install, installed := langs.CapnpGenerator(lang)
		status := "NOT INSTALLED"
		if installed {
			status, install = "ok", ""
		} else {
			missing++
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", lang, binary, status, install)
	}
	if err := w.Flush(); err != nil {
		return missing
	}

	fmt.Fprintln(cmd.OutOrStdout(),
		"\nA missing capnp generator only matters for the language it produces; the\nschema is emitted either way.")
	return missing
}
