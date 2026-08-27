package bufir

// diag_build.go holds the checks that run once over the finished graph, and the
// diagnostic plumbing the builder collects them through.
//
// They are separate from the walk because they need the whole graph: a package
// split across two ROS packages cannot be seen from any single file.

import (
	"fmt"
	"sort"
	"strings"
)

// checkROSPackages reports one proto package split across two ROS packages.
func (b *builder) checkROSPackages() {
	byProto := map[string]map[string][]string{}
	for _, file := range b.schema.Files {
		if byProto[file.Package] == nil {
			byProto[file.Package] = map[string][]string{}
		}
		byProto[file.Package][file.ROSPackage] = append(byProto[file.Package][file.ROSPackage], file.Path)
	}

	pkgs := make([]string, 0, len(byProto))
	for pkg := range byProto {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	for _, pkg := range pkgs {
		groups := byProto[pkg]
		if len(groups) < 2 {
			continue
		}
		names := make([]string, 0, len(groups))
		for ros, files := range groups {
			sort.Strings(files)
			names = append(names, fmt.Sprintf("%s (%s)", ros, strings.Join(files, ", ")))
		}
		sort.Strings(names)
		b.report(Diagnostic{
			Rule: RuleLint,
			Node: NodeID(pkg),
			Message: fmt.Sprintf("proto package %s is split across %d ROS packages: %s",
				pkg, len(groups), strings.Join(names, "; ")),
			Hint: "set the same (buffers.v1.file).ros_package on every file of the package — the default " +
				"is derived per file, so omitting it on one file splits that file's types out",
		})
	}
}

// report records a diagnostic for later severity resolution.
func (b *builder) report(d Diagnostic) { b.pending = append(b.pending, d) }

// diags returns the accumulated diagnostics in a stable order.
func (b *builder) diags() []Diagnostic {
	sort.SliceStable(b.pending, func(i, j int) bool {
		if b.pending[i].Node != b.pending[j].Node {
			return b.pending[i].Node < b.pending[j].Node
		}
		return b.pending[i].Message < b.pending[j].Message
	})
	return b.pending
}

// joinDiags renders diagnostics as an indented list for one error.
func joinDiags(ds []Diagnostic) string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.String()
	}
	return strings.Join(out, "\n  - ")
}
