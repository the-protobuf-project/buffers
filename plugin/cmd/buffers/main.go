// Command buffers converts AIP-annotated protos into serialization schema and,
// optionally, drives the toolchains that compile that schema into language code.
//
// It is the second of this repository's two entry points. protoc-gen-buffers is
// the first: a protoc plugin that runs inside a buf or protoc invocation and
// emits schema. This one compiles the protos itself, renders every configured
// target in one pass, and then runs flatc, capnp or thrift over the result.
//
// The split is deliberate. A protoc plugin that shelled out to another compiler
// would hand that compiler's availability and latency to every caller, including
// the ones who only wanted a descriptor pass. Everything that needs a subprocess
// lives here instead.
//
// # Install
//
//	go install github.com/the-protobuf-project/buffers/plugin/cmd/buffers@latest
//
// # Use
//
//	buffers init                 # write a starter buffers.yaml
//	buffers generate             # render every configured target
//	buffers generate --lang go   # ... and compile the schema into Go
//	buffers targets              # what can be emitted, and what is installed
//	buffers doctor               # what is missing here, and how to install it
//	buffers verify               # fail if regenerating would move a slot
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
)

// version is the build version, injected at release time via
// -ldflags "-X main.version=...".
var version = "dev"

// main runs the CLI, exiting non-zero on failure.
func main() {
	if err := root().Execute(); err != nil {
		// cobra has already printed the error; exiting non-zero is all that is
		// left, and printing it a second time would double every message.
		os.Exit(1)
	}
}

// root builds the command tree.
func root() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buffers",
		Short: "One AIP-annotated schema, every serialization surface",
		Long: `buffers converts AIP-annotated protobuf into FlatBuffers, Cap'n Proto, Apache
Thrift, ROS 2 and Square Wire schema, and drives the toolchains that compile it.

Every field is assigned a target slot that does not move between runs, recorded
in buffers.lock. That file is the point of the tool: a proto field number and a
Cap'n Proto ordinal are different numbering schemes, and a slot that silently
shifts when someone deletes a field is a wire break that nothing else reports.

Thrift is the exception, and needs no ledger: its field ids are proto field
numbers, so nothing can shift.`,
		SilenceUsage: true,
		// A failure inside a command is not a usage error, and printing the whole
		// help text after a schema diagnostic buries it.
		SilenceErrors: false,
		Version:       version,
	}

	cmd.AddCommand(generateCmd(), verifyCmd(), targetsCmd(), doctorCmd(), initCmd())
	return cmd
}

// fail formats an error for a human at a terminal.
func fail(format string, args ...any) error { return fmt.Errorf(format, args...) }

// provenanceInfo is the banner context for this build. Commands that only read
// the schema still need one, because the source builds the same model either way.
func provenanceInfo() provenance.Info { return provenance.Info{Version: version} }
