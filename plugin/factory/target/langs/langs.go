// Package langs drives the external compilers that turn an emitted schema into
// language code: flatc for FlatBuffers, capnp for Cap'n Proto, thrift for Apache
// Thrift.
//
// # Why this is the CLI's job and not the plugin's
//
// A protoc plugin runs inside someone else's build. Shelling out to a second
// compiler from there hands every caller that compiler's availability, its exit
// codes and its latency, whether or not they wanted language output — and a
// `buf generate` that fails because flatc is not installed is a confusing failure
// for someone who only asked for a descriptor pass.
//
// So the plugin emits schema and stops, and this package exists on the other side
// of that line, invoked by `buffers generate` when a config entry asks for a
// language.
//
// # What it guarantees
//
// Nothing about the generated code — that is the toolchain's business. What it
// does own is the failure modes: a missing compiler is reported as a missing
// compiler with the install line, not as an exec error; and a compiler that runs
// and fails has its own diagnostics forwarded rather than summarized, because
// flatc's message about a schema is more useful than anything this package could
// say about flatc.
package langs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Request is one compile: a target's emitted schema, in one language.
type Request struct {
	// Target is the buffers target whose schema is being compiled.
	Target string

	// Language is the language to produce.
	Language string

	// SchemaDir is the directory the schema was emitted into. It is also the
	// import root, since the schema's own includes are relative to it.
	SchemaDir string

	// OutDir is where the compiled code goes. Created if absent.
	OutDir string

	// Files are the schema files to compile, relative to SchemaDir.
	Files []string

	// Flags are extra generator flags for this invocation — the package-naming
	// options that vary per group. See layout.go.
	Flags []string

	// Options are generator options for a compiler that takes them attached to
	// the generator name rather than as separate flags. Thrift spells its Go
	// module root `--gen go:package_prefix=…`, where flatc spells the equivalent
	// as a flag of its own, so the two cannot share Flags.
	Options []string

	// Annotated names the languages whose annotation blocks the emitted schema
	// carries. It is not the same as Language: a schema annotated for Go needs
	// go.capnp resolvable even when it is being compiled as C++, because an
	// unresolvable import fails the parse regardless of the output language.
	Annotated []string
}

// Tool describes an external compiler.
type Tool struct {
	// Binary is the executable name looked up on PATH.
	Binary string

	// Install is how to obtain it, resolved to this machine's package manager at
	// the point the message is printed. See install.go.
	Install Install

	// Args builds the command line for one request.
	Args func(Request) ([]string, error)

	// PerFile makes Run invoke the compiler once per schema file rather than once
	// for the whole list. It exists for Apache Thrift, whose compiler accepts a
	// single input file per run.
	PerFile bool
}

// Run compiles one request.
//
// The command runs with its working directory set to the schema directory, so
// that the file arguments — which are relative to it, and which appear verbatim
// in the compiler's diagnostics — stay short and recognizable. Every *other* path
// handed to the compiler is made absolute first: a relative -I or -o would
// otherwise be resolved against that new working directory rather than against
// the one the user typed the command in.
func Run(r Request) error {
	tool, ok := tools[r.Target]
	if !ok {
		return fmt.Errorf("%w for target %q: its code generation is driven by the consuming build "+
			"(colcon for ros, Gradle for wire), not by a standalone compiler", ErrNoToolchain, r.Target)
	}

	path, err := exec.LookPath(tool.Binary)
	if err != nil {
		return fmt.Errorf("%s is not on PATH, and the %s target needs it to produce %s.\n%s",
			tool.Binary, r.Target, r.Language, tool.Install.Detail("    "))
	}

	if err := os.MkdirAll(r.OutDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", r.OutDir, err)
	}
	if r.SchemaDir, err = filepath.Abs(r.SchemaDir); err != nil {
		return err
	}
	if r.OutDir, err = filepath.Abs(r.OutDir); err != nil {
		return err
	}
	if err := checkGenerator(r); err != nil {
		return err
	}

	for _, batch := range batches(tool, r.Files) {
		one := r
		one.Files = batch
		if err := invoke(path, tool, one); err != nil {
			return err
		}
	}
	return nil
}

// batches splits the file list into the groups one invocation each takes, which
// is the whole list for every compiler here except Thrift's.
func batches(tool Tool, files []string) [][]string {
	if !tool.PerFile {
		return [][]string{files}
	}
	out := make([][]string, 0, len(files))
	for _, f := range files {
		out = append(out, []string{f})
	}
	return out
}

// invoke runs the compiler once.
func invoke(path string, tool Tool, r Request) error {
	args, err := tool.Args(r)
	if err != nil {
		return err
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = r.SchemaDir
	// The compiler's own diagnostics are forwarded rather than captured: flatc
	// naming a line in a schema is more useful than anything this package could
	// say about flatc.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", tool.Binary, strings.Join(args, " "), err)
	}
	return nil
}
