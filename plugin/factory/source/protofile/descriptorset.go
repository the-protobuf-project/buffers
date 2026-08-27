package protofile

// descriptorset.go builds a plugin request from a prebuilt FileDescriptorSet, and
// supplies the Go import paths protogen demands.
//
// This is the robust input path: buf has already resolved every dependency,
// including BSR modules that exist nowhere in the working tree, so nothing here
// has to.

import (
	"fmt"
	"os"
	"path"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// fromDescriptorSet reads a prebuilt FileDescriptorSet.
func fromDescriptorSet(in Input) (*protogen.Plugin, error) {
	data, err := os.ReadFile(in.DescriptorSet)
	if err != nil {
		return nil, fmt.Errorf("read descriptor set: %w", err)
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(data, set); err != nil {
		return nil, fmt.Errorf("parse descriptor set %s: %w", in.DescriptorSet, err)
	}

	targets := in.Generate
	if len(targets) == 0 {
		// Everything the set carries that is not somebody else's schema. A
		// descriptor set includes every transitive dependency, so generating from
		// all of it would emit google/protobuf/descriptor.fbs into the user's
		// output tree.
		for _, f := range set.File {
			if isDependencyOnly(f.GetName()) {
				continue
			}
			targets = append(targets, f.GetName())
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("descriptor set %s contains no files to generate from", in.DescriptorSet)
	}
	return newPlugin(set, targets, in.Parameter)
}

// isDependencyOnly reports whether a path is a well-known or vendored schema that
// a run should compile against but never generate for.
//
// The list is prefix-based and deliberately short. Anything else in a descriptor
// set was put there by the user's own buf.yaml, and guessing that it is "not
// really theirs" is not this function's call to make — `generate:` in the config
// is how a caller narrows it further.
func isDependencyOnly(path string) bool {
	for _, prefix := range []string{
		"google/protobuf/",
		"google/api/",
		"google/rpc/",
		"google/type/",
		"google/longrunning/",
		"buffers/v1/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// newPlugin assembles the CodeGeneratorRequest and hands it to protogen.
func newPlugin(set *descriptorpb.FileDescriptorSet, targets []string, parameter string) (*protogen.Plugin, error) {
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: targets,
		ProtoFile:      set.File,
	}
	if parameter != "" {
		req.Parameter = proto.String(parameter)
	}
	SynthesizeGoPackages(req)

	p, err := protogen.Options{}.New(req)
	if err != nil {
		return nil, fmt.Errorf("build plugin request: %w", err)
	}
	return p, nil
}

// SynthesizeGoPackages supplies a Go import path for every generated file that
// declares none, so that protogen will accept the request.
//
// # Why this is necessary
//
// protogen refuses to build a request when a file in FileToGenerate has no
// `option go_package`:
//
//	unable to determine Go import path for "robot/v1/robot.proto"
//
// That is reasonable for protoc-gen-go and wrong here. This plugin emits
// FlatBuffers, Cap'n Proto, ROS and Gradle wiring; none of them is Go, and a
// robotics team writing .proto for ROS has no reason to declare a Go package.
// Without this, the most on-target user of the tool cannot run it at all.
//
// # Why it does not corrupt what the plugin reads
//
// The synthetic value is passed as an M parameter, which is protogen's own
// mechanism for supplying an import path out of band. It sets the file's
// GoImportPath and leaves FileDescriptorProto.Options.GoPackage untouched — so
// bufir, which reads the descriptor option directly, still sees the absence.
// That matters: the Cap'n Proto target derives $Go.package from a real
// go_package and warns when there is none, and a synthesized one would turn that
// warning into a confidently wrong annotation.
//
// The value itself is the file's directory, which is never used — no Go is
// emitted from it — but must be a syntactically valid import path.
func SynthesizeGoPackages(req *pluginpb.CodeGeneratorRequest) {
	declared := map[string]bool{}
	for _, f := range req.ProtoFile {
		if f.GetOptions().GetGoPackage() != "" {
			declared[f.GetName()] = true
		}
	}

	var params []string
	for _, name := range req.GetFileToGenerate() {
		if declared[name] {
			continue
		}
		dir := path.Dir(name)
		if dir == "." || dir == "/" {
			dir = strings.TrimSuffix(path.Base(name), ".proto")
		}
		params = append(params, "M"+name+"="+dir)
	}
	if len(params) == 0 {
		return
	}

	// Prepended, so an M the caller supplied for the same file still wins:
	// protogen takes the last value for a repeated key.
	joined := strings.Join(params, ",")
	if existing := req.GetParameter(); existing != "" {
		joined += "," + existing
	}
	req.Parameter = proto.String(joined)
}
