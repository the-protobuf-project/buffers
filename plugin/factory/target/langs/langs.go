// Package langs drives the external compilers that turn an emitted schema into
// language code: flatc for FlatBuffers, capnp for Cap'n Proto.
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
	"errors"
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

	// Install is the line to show someone who does not have it.
	Install string

	// Args builds the command line for one request.
	Args func(Request) ([]string, error)
}

// tools maps a buffers target to the compiler that consumes its schema.
//
// ros and wire are absent, and that is not an omission. rosidl generates from a
// .msg as part of a colcon build, driven by a package's CMakeLists rather than by
// a one-shot command, and Wire generates from .proto during a Gradle build. In
// both cases the build system owns the invocation, and a `buffers` subprocess
// that tried to take it over would be guessing at a workspace it cannot see.
var tools = map[string]Tool{
	"flatbuffers": {
		Binary:  "flatc",
		Install: "brew install flatbuffers   (or see https://flatbuffers.dev)",
		Args: func(r Request) ([]string, error) {
			flag, ok := flatcFlags[r.Language]
			if !ok {
				return nil, fmt.Errorf("flatc has no generator for %q; it supports: %s",
					r.Language, keys(flatcFlags))
			}
			args := []string{flag, "-o", r.OutDir, "-I", r.SchemaDir}
			args = append(args, r.Flags...)
			return append(args, r.Files...), nil
		},
	},
	"capnp": {
		Binary: "capnp",
		// Two packages on Debian and Ubuntu, and the second is the one people
		// miss: `capnproto` is the compiler, `libcapnp-dev` is the schema files
		// every generated .capnp imports. Without it capnp reports
		// "Import failed: /capnp/c++.capnp", which reads as a defect in the
		// schema rather than a missing package.
		Install: "brew install capnp   |   apt install capnproto libcapnp-dev   " +
			"(see https://capnproto.org/install.html)",
		Args: func(r Request) ([]string, error) {
			gen, ok := capnpGenerators[r.Language]
			if !ok {
				return nil, fmt.Errorf("capnp has no generator for %q; it supports: %s",
					r.Language, keys(capnpGenerators))
			}

			args := []string{"compile", "-I", r.SchemaDir}

			// Annotation schemas for every language whose block the emitted
			// schema may carry — not only the one being compiled now. A file
			// annotated for Go fails to compile *as C++* if go.capnp cannot be
			// resolved, so the import path has to cover what is in the file
			// rather than what is being asked for.
			for _, lang := range r.Annotated {
				if dir, ok := annotationImportPath(lang); ok {
					args = append(args, "-I", dir)
				}
			}

			// capnp writes output next to the source unless the generator spec
			// carries a directory after a colon.
			args = append(args, "-o", gen+":"+r.OutDir)
			args = append(args, r.Flags...)
			return append(args, r.Files...), nil
		},
	},
}

// ErrNoToolchain reports that a target has no one-shot compiler to drive.
var ErrNoToolchain = errors.New("no toolchain to invoke")

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
		return fmt.Errorf("%s is not on PATH, and the %s target needs it to produce %s.\n    install: %s",
			tool.Binary, r.Target, r.Language, tool.Install)
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

	args, err := tool.Args(r)
	if err != nil {
		return err
	}
	if err := checkGenerator(r); err != nil {
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
