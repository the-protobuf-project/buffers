// Command protoc-gen-buffers is a protoc plugin that reads proto descriptors
// annotated with google.api.* and buffers.v1.* options and emits serialization
// schema for the requested target: FlatBuffers, Cap'n Proto, ROS IDL, or Square
// Wire.
//
// # Install
//
//	go install github.com/the-protobuf-project/buffers/plugin/cmd/protoc-gen-buffers@latest
//
// # Usage via buf.gen.yaml
//
//	plugins:
//	  - local: protoc-gen-buffers
//	    out: schema/
//	    opt:
//	      - target=capnp        # flatbuffers | capnp | ros | wire
//	      - lock=buffers.lock   # the ordinal ledger; commit it
//	      - strict=ordinal:error
//
// # What it does not do
//
// It emits schema and stops. Compiling that schema into a language — running
// flatc or capnp — is the `buffers` CLI's job, not this one's. A protoc plugin
// that shells out to another compiler mid-run inherits that compiler's
// availability, its exit codes and its latency, and hands all three to every
// person who only wanted a descriptor pass.
//
// # Inference priority
//
//  1. buffers.v1 options       — explicit ordinals, layout, bounds, widths
//  2. buffers.lock             — the recorded slot for anything seen before
//  3. google.api.*             — field behavior, resources, method shape
//  4. the proto's own structure — field numbers, declaration order, packages
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/registry"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
	"github.com/the-protobuf-project/protokit/factory"
	"github.com/the-protobuf-project/protokit/header"
)

// version is the build version, injected at release time via
// -ldflags "-X main.version=...".
var version = "dev"

// resolveVersion returns the build version to stamp into generated files.
//
// A release sets `version` via ldflags and wins outright. Otherwise we recover it
// from the build info the Go toolchain embeds: `go install …@v0.1.2` records the
// tag as the main module version. Only a genuine local build — which reports
// "(devel)" — falls back to "dev".
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return version
}

// main runs the plugin: a CodeGeneratorRequest on stdin, a response on stdout.
func main() {
	v := resolveVersion()

	// When invoked directly with -version (not by protoc), print and exit before
	// protogen tries to read a CodeGeneratorRequest from stdin.
	if len(os.Args) == 2 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Printf("protoc-gen-buffers %s\n", v)
		return
	}

	// Every generated file's banner names the tool that produced it. SetProject
	// is needed because the repository is not named after the binary: the plugin
	// is protoc-gen-buffers and it lives in buffers, so deriving the link from the
	// tool name would put a dead URL in every emitted schema.
	header.SetTool("protoc-gen-buffers")
	header.SetProject("https://github.com/the-protobuf-project/buffers")

	// flags are populated by protogen (ParamFunc maps each buf.gen.yaml opt:
	// "key=value" to flags.Set) before the Run closure reads them.
	var flags flag.FlagSet
	target := flags.String("target", "", "output backend: "+registry.TargetNames())
	lock := flags.String("lock", bufir.LockFileName,
		"path to the ordinal ledger, relative to the directory buf or protoc is run from "+
			"(not to the plugin's out: directory — the ledger is read back on the next run, "+
			"so both ends must name the same file). It records the target slot every field "+
			"was assigned, and a build that would move one reports it instead. Commit this "+
			"file. Empty disables it, which gives up slot stability across runs")
	strict := flags.String("strict", "",
		"per-rule severity: \"\"=all warn, \"true\"=all error, or "+
			"\"ordinal:error,layout:warn,target:error,lint:warn\"")
	var languages stringList
	flags.Var(&languages, "lang",
		"a language the emitted schema will be compiled into (go, rust, python, java, "+
			"kotlin, swift, dart, cpp, ...). Repeat it for several. This plugin compiles "+
			"nothing — the list only tells a target what its generator will demand of the "+
			"schema. It matters for capnp, whose Go and Java generators reject a schema "+
			"missing their annotation blocks; see plugin/factory/target/capnp/annotations.go")

	goModule := flags.String("go_module", "",
		"Go module the generated Cap'n Proto code will live in. capnpc-go's $Go.import "+
			"must name where a generated file lands, which is the schema path under this "+
			"module root; `option go_package` describes a different path and cannot supply it")

	// protogen.Options.Run would be shorter, and cannot be used: it unmarshals the
	// request and calls New in one step, leaving nowhere to supply a Go import
	// path for a proto that declares none. protogen rejects such a request
	// outright — "unable to determine Go import path" — which would make this
	// plugin unusable on exactly the schemas it exists for, since a .proto written
	// for ROS or FlatBuffers has no reason to carry `option go_package`. So the
	// loop Run performs is written out here, with SynthesizeGoPackages between the
	// two halves.
	run(&flags, func(p *protogen.Plugin) error {
		// Proto3 `optional` is fully supported — explicit presence becomes a
		// FlatBuffers `= null` and a Cap'n Proto default — so declare it rather
		// than letting buf warn that the plugin might mishandle it.
		p.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

		opts := bufir.Options{Strict: *strict, LockPath: *lock}
		info := provenance.Info{
			Version:       v,
			ProtocVersion: protocVersion(p),
		}

		reg := registry.New(emit.Through(p), opts, info, registry.Options{
			Languages: languages,
			GoModule:  *goModule,
		})
		tgt, ok := reg.Targets[*target]
		if !ok {
			if *target == "" {
				return fmt.Errorf("required option \"target\" is missing — add opt: [target=%s] to your buf.gen.yaml plugin entry",
					reg.TargetNames())
			}
			return fmt.Errorf("unknown target %q — valid targets: %s", *target, reg.TargetNames())
		}

		ctx := factory.Ctx{Plugin: p}
		model, err := reg.Sources["proto"].Build(ctx)
		if err != nil {
			return err
		}
		if err := tgt.Generate(ctx, model, ""); err != nil {
			return err
		}

		// The ledger is written last, after every target has had a chance to add
		// a diagnostic, and only when the run is otherwise succeeding: recording
		// slots for a schema that failed to emit would leave the ledger claiming
		// assignments no consumer ever saw.
		if err := writeLock(model.Schema, *lock); err != nil {
			return err
		}
		return reportDiagnostics(p, model.Schema, *strict)
	})
}
