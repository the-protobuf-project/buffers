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
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	set, err = asProtocWouldSend(set)
	if err != nil {
		return nil, err
	}
	return newPlugin(set, targets, in.Parameter)
}

// asProtocWouldSend round-trips the set through its wire form.
//
// protocompile interprets each option into a dynamic message built from the
// compiled .proto that declares it, and ToFileDescriptorProto copies those
// through untouched. protoc and buf hand a plugin bytes instead, which parse
// into the generated Go types wherever the binary links them — and protokit's
// readers expect exactly those. Given a google.api.resource as a dynamic
// message they panicked converting it, so any AIP-annotated schema with
// googleapis among its imports crashed. Re-parsing makes this path deliver
// what the protoc path always has.
func asProtocWouldSend(set *descriptorpb.FileDescriptorSet) (*descriptorpb.FileDescriptorSet, error) {
	data, err := proto.Marshal(set)
	if err != nil {
		return nil, fmt.Errorf("encode compiled descriptors: %w", err)
	}
	out := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("decode compiled descriptors: %w", err)
	}
	return out, nil
}

// collect appends a file and its transitive dependencies, dependencies first —
// the order protoc guarantees and protogen relies on.
//
// It walks protoreflect descriptors rather than linker.File: the well-known
// types arrive through protocompile's standard imports as plain descriptors,
// and a walk that only followed linker files left google/protobuf/*.proto out
// of the request, which protogen then refused as an unresolvable import.
func collect(f protoreflect.FileDescriptor, set *descriptorpb.FileDescriptorSet, seen map[string]bool) {
	if seen[f.Path()] {
		return
	}
	seen[f.Path()] = true

	imports := f.Imports()
	for i := range imports.Len() {
		collect(imports.Get(i).FileDescriptor, set, seen)
	}
	set.File = append(set.File, protodesc.ToFileDescriptorProto(f))
}
