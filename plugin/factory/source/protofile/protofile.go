// Package protofile turns .proto files on disk into the protogen.Plugin the rest
// of the pipeline expects.
//
// The protoc plugin never uses it: protoc hands it a CodeGeneratorRequest on
// stdin, already parsed and linked. The CLI does, because a CLI that required
// protoc or buf to be installed before it could read a .proto would be a wrapper
// rather than a tool.
//
// It is a separate package from the proto Source so that protoc-gen-buffers does
// not link a proto compiler it will never call.
//
// # Two ways in
//
// From source, via protocompile: convenient, and only as good as the import paths
// it is given. A tree that depends on googleapis needs those .proto files on
// disk somewhere.
//
// From a descriptor set, via `buf build -o set.binpb --as-file-descriptor-set`:
// the robust path. buf has already resolved every dependency, including BSR
// modules that exist nowhere in the working tree, so nothing here has to. When a
// project already uses buf — which anything in this ecosystem does — this is the
// option to reach for.
package protofile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"google.golang.org/protobuf/compiler/protogen"
)

// Input describes where to find the protos.
type Input struct {
	// Paths are directories whose .proto files are generated from. They are also
	// import roots.
	Paths []string

	// Imports are additional import roots that are compiled but not generated
	// from.
	Imports []string

	// DescriptorSet is a prebuilt FileDescriptorSet. When set, Paths and Imports
	// are not read and no compilation happens; Generate below selects which of
	// the set's files are generated from.
	DescriptorSet string

	// Generate restricts which files are generate-flagged. Empty means every file
	// found under Paths, or — for a descriptor set — every file that is not a
	// well-known type or a dependency-only import.
	Generate []string

	// Parameter is the option string handed to the plugin, exactly as protoc
	// would pass it.
	Parameter string
}

// Load produces a protogen.Plugin.
func Load(in Input) (*protogen.Plugin, error) {
	if in.DescriptorSet != "" {
		return fromDescriptorSet(in)
	}
	return fromSource(in)
}

// discover finds every .proto under the given roots, returning paths relative to
// the root that contains them — which is how an import path is spelled.
//
// A path found under two roots is returned once. Two roots can legitimately hold
// the same relative path — a vendored tree beside a local one — and listing it
// twice in FileToGenerate makes protogen generate the file twice: two writes to
// one output path, and two ledger entries for one field.
func discover(roots []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".proto" {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if slash := filepath.ToSlash(rel); !seen[slash] {
				seen[slash] = true
				out = append(out, slash)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", root, err)
		}
	}
	sort.Strings(out)
	return out, nil
}
