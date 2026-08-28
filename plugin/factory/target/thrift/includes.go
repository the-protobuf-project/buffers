package thrift

// includes.go binds each imported proto file to the prefix Thrift will reference
// its types through.
//
// Thrift scopes an included file's types under the include's *base name*, not
// under any namespace the file declares. `include "sensors/v1/geometry.thrift"`
// makes its types reachable as `geometry.Vector3` and by nothing else, whatever
// `namespace * sensors.v1` says. That single fact drives everything here,
// including the one failure it makes possible: two included files with the same
// base name claim the same prefix, and only one of them can win.

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"
)

// assignIncludes returns the imports this file will include, keyed by proto path.
//
// The google.protobuf files are filtered out: their types are substituted by the
// prelude rather than passed through, exactly as the FlatBuffers and Cap'n Proto
// targets do, so including them would name a .thrift nothing emits.
func (r *run) assignIncludes(f *buffers.File) map[string]string {
	out := map[string]string{}
	for _, imp := range f.Imports {
		if strings.HasPrefix(imp, "google/protobuf/") {
			continue
		}
		out[imp] = includePrefix(imp)
	}
	r.checkPrefixes(f, out)
	r.checkEmitted(f, out)
	return out
}

// includePrefix derives the prefix Thrift binds an include to: the file's base
// name with the extension removed.
func includePrefix(protoPath string) string {
	base := path.Base(thriftPath(protoPath))
	return strings.TrimSuffix(base, path.Ext(base))
}

// prefixFor returns the prefix bound to another file's types.
func (r *run) prefixFor(f *buffers.File) string {
	if prefix, ok := r.includes[f.Path]; ok {
		return prefix
	}
	return includePrefix(f.Path)
}

// includeLines renders the include statements, plus the prelude's when the body
// reached for a substituted type.
func (r *run) includeLines() []string {
	paths := make([]string, 0, len(r.includes))
	for p := range r.includes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := make([]string, 0, len(paths)+1)
	for _, p := range paths {
		out = append(out, fmt.Sprintf("include %q", thriftPath(p)))
	}
	if len(r.fileNeeds) > 0 {
		out = append(out, fmt.Sprintf("include %q", preludePath))
	}
	sort.Strings(out)
	return out
}

// checkPrefixes reports two includes competing for one prefix.
//
// This is a real collision and not a stylistic one: a `common.proto` in two
// packages produces two `common.thrift`, both of which want to be `common.`, and
// every reference through the shadowed one silently resolves to a type in the
// other file — or fails with an error naming a type that does exist, just not
// there.
func (r *run) checkPrefixes(f *buffers.File, includes map[string]string) {
	byPrefix := map[string][]string{}
	for p, prefix := range includes {
		byPrefix[prefix] = append(byPrefix[prefix], p)
	}

	prefixes := make([]string, 0, len(byPrefix))
	for prefix, paths := range byPrefix {
		if len(paths) > 1 {
			prefixes = append(prefixes, prefix)
		}
	}
	sort.Strings(prefixes)

	for _, prefix := range prefixes {
		paths := byPrefix[prefix]
		sort.Strings(paths)
		r.collect(&buffers.Diagnostic{
			Rule: buffers.RuleTarget,
			Node: buffers.NodeID(f.Path),
			Message: fmt.Sprintf("%s both include as %q; Thrift scopes an include by its file name, "+
				"so one shadows the other", strings.Join(paths, " and "), prefix),
			Hint: "rename one of the .proto files — the base name is the Thrift namespace here, and " +
				"the directory above it does not disambiguate",
		})
	}
}

// checkEmitted reports an include naming a file this run does not write.
//
// An import outside the generate set has no .thrift beside it, and Thrift fails
// on a missing include with a path rather than with the type that needed it. The
// message is more useful from here, where the referring file is still in hand.
func (r *run) checkEmitted(f *buffers.File, includes map[string]string) {
	emitted := make(map[string]bool, len(r.schema.Files))
	for _, got := range r.schema.Files {
		emitted[got.Path] = true
	}

	paths := make([]string, 0, len(includes))
	for p := range includes {
		if !emitted[p] {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)

	for _, p := range paths {
		r.collect(&buffers.Diagnostic{
			Rule:    buffers.RuleTarget,
			Node:    buffers.NodeID(f.Path),
			Message: fmt.Sprintf("includes %q, which this run does not generate", thriftPath(p)),
			Hint: fmt.Sprintf("add %s to the files being generated from, or the emitted schema will "+
				"not compile", p),
		})
	}
}
