package main

// run.go is protogen.Options.Run, opened up so the request can be adjusted
// between unmarshalling and validation.
//
// The closed version cannot be used: it unmarshals and validates in one step,
// leaving nowhere to supply a Go import path for a proto that declares none —
// and protogen rejects such a request outright, which would make this plugin
// unusable on exactly the schemas it exists for.

import (
	"flag"
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/the-protobuf-project/buffers/plugin/factory/source/protofile"
)

// run is protogen.Options.Run, opened up so the request can be adjusted between
// unmarshalling and validation. See the call site for why that is needed.
func run(flags *flag.FlagSet, gen func(*protogen.Plugin) error) {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal(err)
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(in, req); err != nil {
		fatal(fmt.Errorf("parse CodeGeneratorRequest: %w", err))
	}

	protofile.SynthesizeGoPackages(req)

	p, err := protogen.Options{ParamFunc: flags.Set}.New(req)
	if err != nil {
		fatal(err)
	}
	if err := gen(p); err != nil {
		// Reported through the response rather than as a crash, so buf shows the
		// message as a plugin error instead of a failed subprocess.
		p.Error(err)
	}

	out, err := proto.Marshal(p.Response())
	if err != nil {
		fatal(err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fatal(err)
	}
}

// fatal reports a failure that happened before a response could be built, which
// is the only case where exiting is better than answering.
func fatal(err error) {
	fmt.Fprintf(os.Stderr, "protoc-gen-buffers: %v\n", err)
	os.Exit(1)
}
