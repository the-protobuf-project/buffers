package langs

// tools.go holds one entry per external compiler: how to find it, how to tell
// somebody to install it, and how to build its command line.
//
// It is separate from langs.go so that the set of toolchains a build can drive is
// one file to read rather than something to infer from a long map literal wedged
// between the runner and its types.

import (
	"errors"
	"fmt"
	"strings"
)

// tools maps a buffers target to the compiler that consumes its schema.
//
// ros and wire are absent, and that is not an omission. rosidl generates from a
// .msg as part of a colcon build, driven by a package's CMakeLists rather than by
// a one-shot command, and Wire generates from .proto during a Gradle build. In
// both cases the build system owns the invocation, and a `buffers` subprocess
// that tried to take it over would be guessing at a workspace it cannot see.
var tools = map[string]Tool{
	"flatbuffers": {
		Binary: "flatc",
		Install: Install{
			Docs: "https://flatbuffers.dev/flatbuffers_guide_building.html",
			By: map[string]string{
				"brew":   "brew install flatbuffers",
				"apt":    "sudo apt-get install -y flatbuffers-compiler",
				"dnf":    "sudo dnf install -y flatbuffers-compiler",
				"pacman": "sudo pacman -S flatbuffers",
				"apk":    "sudo apk add flatbuffers",
				"port":   "sudo port install flatbuffers",
			},
			Notes: []string{"Go output additionally wants flatbuffers 23.1.4 or newer; see capability.go."},
		},
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
		Install: Install{
			Docs: "https://capnproto.org/install.html",
			By: map[string]string{
				"brew": "brew install capnp",
				// Two packages on Debian and Ubuntu, and the second is the one
				// people miss: `capnproto` is the compiler, `libcapnp-dev` is the
				// schema files every generated .capnp imports. Without it capnp
				// reports "Import failed: /capnp/c++.capnp", which reads as a
				// defect in the schema rather than as a missing package.
				"apt":    "sudo apt-get install -y capnproto libcapnp-dev",
				"dnf":    "sudo dnf install -y capnproto capnproto-devel",
				"pacman": "sudo pacman -S capnproto",
				"apk":    "sudo apk add capnproto capnproto-dev",
				"port":   "sudo port install capnproto",
			},
			Notes: []string{
				"Only the C++ generator ships with capnp; every other language is a " +
					"separate capnpc-<lang> binary. Run `buffers doctor` for those.",
			},
		},
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
	"thrift": {
		Binary: "thrift",
		Install: Install{
			Docs: "https://thrift.apache.org/docs/install/",
			By: map[string]string{
				"brew":   "brew install thrift",
				"apt":    "sudo apt-get install -y thrift-compiler",
				"dnf":    "sudo dnf install -y thrift",
				"pacman": "sudo pacman -S thrift",
				"apk":    "sudo apk add thrift",
				"port":   "sudo port install thrift",
			},
		},

		// The Apache Thrift compiler takes exactly one input file per run, where
		// flatc and capnp both take a list. Everything else about the invocation
		// is the same, so the difference is expressed as a flag on the tool
		// rather than as a second code path in the runner.
		PerFile: true,

		Args: func(r Request) ([]string, error) {
			if len(r.Files) != 1 {
				return nil, fmt.Errorf("thrift compiles one file per invocation; got %d", len(r.Files))
			}
			// -out rather than -o: -o creates a gen-<lang>/ directory beneath the
			// path it is given, which would put the output somewhere the caller
			// did not ask for and nothing downstream looks.
			gen := r.Language
			if len(r.Options) > 0 {
				// thrift attaches generator options to the generator name:
				// `--gen go:package_prefix=…`, not `--gen go --package_prefix=…`.
				gen += ":" + strings.Join(r.Options, ",")
			}
			args := []string{"--gen", gen, "-out", r.OutDir, "-I", r.SchemaDir}
			args = append(args, r.Flags...)
			return append(args, r.Files...), nil
		},
	},
}

// ErrNoToolchain reports that a target has no one-shot compiler to drive.
var ErrNoToolchain = errors.New("no toolchain to invoke")
