package protofile

// compile.go builds a plugin request from .proto sources, using protocompile.
//
// Source info is requested explicitly: it is what carries comments, and without
// it every doc comment in every emitted schema would be empty — which is most of
// what makes the output readable.

import (
	"context"
	"fmt"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
)

// fromSource compiles .proto files with protocompile.
func fromSource(in Input) (*protogen.Plugin, error) {
	if len(in.Paths) == 0 {
		return nil, fmt.Errorf("no proto paths given")
	}

	roots := append(append([]string{}, in.Paths...), in.Imports...)
	targets, err := discover(in.Paths)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no .proto files under %s", strings.Join(in.Paths, ", "))
	}
	if len(in.Generate) > 0 {
		targets = in.Generate
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: roots,
		}),
		// Standard source info is what carries comments. Without it every doc
		// comment in every emitted schema would be empty, which is most of what
		// makes the output readable.
		SourceInfoMode: protocompile.SourceInfoStandard,
	}

	linked, err := compiler.Compile(context.Background(), targets...)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}

	set := &descriptorpb.FileDescriptorSet{}
	seen := map[string]bool{}
	for _, f := range linked {
		collect(f, set, seen)
	}
	return newPlugin(set, targets, in.Parameter)
}

// collect appends a linked file and its transitive dependencies, dependencies
// first — the order protoc guarantees and protogen relies on.
func collect(f linker.File, set *descriptorpb.FileDescriptorSet, seen map[string]bool) {
	if seen[f.Path()] {
		return
	}
	seen[f.Path()] = true

	imports := f.Imports()
	for i := range imports.Len() {
		if dep, ok := imports.Get(i).FileDescriptor.(linker.File); ok {
			collect(dep, set, seen)
		}
	}
	set.File = append(set.File, protodesc.ToFileDescriptorProto(f))
}
