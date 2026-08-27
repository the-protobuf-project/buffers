package wire

// gradle.go renders the `wire { }` build block, whose interesting half is the
// tree-shaking roots.
//
// Wire prunes everything unreachable from a declared root, and choosing those by
// hand is tedious and goes stale. AIP makes the choice mechanical: an API's
// reachable surface is exactly its services plus its resources, so the list
// writes itself — with the justification on every line, because a roots list
// without one is a wall of names nobody can review.

import (
	"fmt"
	"sort"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// gradle renders the `wire { }` block.
func (r *run) gradle() error {
	roots, why := r.roots()
	pkgs := r.jvmPackages()

	var b emit.Buf
	b.Raw(r.banner("(every file in this run)"))
	b.Line("")
	b.Line("// This is a fragment, not a build file: paste the block below into your")
	b.Line("// build.gradle.kts, or keep it here and read it as the record of which types")
	b.Line("// are API roots.")
	b.Line("//")
	b.Line("// It is not written as an `apply(from = ...)` script because the Kotlin DSL")
	b.Line("// cannot configure a typed plugin extension from an applied script without")
	b.Line("// resolving the plugin's types by hand, which is worse to read than a paste.")
	b.Line("//")
	b.Line("// Wire consumes the .proto files directly. There is no generated schema to")
	b.Line("// point at, and `sourcePath` below should name wherever your protos already")
	b.Line("// live.")
	b.Line("")

	b.Block("wire {", "}", func() {
		b.Block("sourcePath {", "}", func() {
			b.Line(`srcDir("src/main/proto")`)
		})
		b.Line("")

		if len(roots) > 0 {
			b.Line("// Tree-shaking roots, derived from AIP rather than chosen by hand.")
			b.Line("//")
			b.Line("// An API's reachable surface is exactly its services plus its resources:")
			b.Line("// everything a caller can invoke hangs off a method, and everything a")
			b.Line("// resource carries hangs off the resource. Wire closes over these")
			b.Line("// transitively and prunes the rest, so a JVM app stops paying for")
			b.Line("// messages it never touches.")
			b.Block("roots(", ")", func() {
				// A trailing comma on the last entry is valid Kotlin and keeps a
				// later addition to a one-line diff.
				for _, root := range roots {
					b.Linef("%q, // %s", root, why[root])
				}
			})
			b.Line("")
		}

		switch r.lang {
		case "kotlin":
			b.Block("kotlin {", "}", func() {
				b.Line("// suspending is the call style for a coroutine codebase; use")
				b.Line("// \"blocking\" if the callers are not suspend functions.")
				b.Line(`rpcCallStyle = "suspending"`)
				b.Line(`rpcRole = "client"`)
				b.Line("javaInterop = false")
			})
		case "java":
			b.Block("java {", "}", func() {
				b.Line("android = false")
			})
		case "swift":
			b.Block("swift {", "}", func() {})
		}
	})

	if len(pkgs) > 0 {
		b.Line("")
		b.Line("// JVM packages the generated code will land in, one per proto package.")
		b.Line("// Taken from (buffers.v1.file).jvm_package, then java_package, then the proto")
		b.Line("// package — the same order Wire itself resolves them in, recorded here so a")
		b.Line("// reviewer can see the result without running the build.")
		for _, p := range pkgs {
			b.Linef("//   %s -> %s", p.Proto, p.JVM)
		}
	}

	return r.sink("wire.gradle.kts", b.Bytes())
}

// jvmPackage is one proto package and the JVM package it maps to.
type jvmPackage struct {
	// Proto is the proto package, and JVM the package Wire generates into.
	Proto, JVM string
}

// jvmPackages returns each proto package's JVM package, reporting one proto
// package that maps to two — Wire generates one package per proto package, so
// the last would silently win.
func (r *run) jvmPackages() []jvmPackage {
	seen := map[string]string{}
	for _, f := range r.schema.Files {
		if _, ok := seen[f.Package]; !ok {
			seen[f.Package] = f.JVMPackage
			continue
		}
		if seen[f.Package] != f.JVMPackage {
			r.collect(&buffers.Diagnostic{
				Rule: buffers.RuleLint,
				Node: buffers.NodeID(f.Package),
				Message: fmt.Sprintf("proto package %s maps to two JVM packages (%s and %s); Wire generates "+
					"one package per proto package and the last one wins", f.Package, seen[f.Package], f.JVMPackage),
				Hint: "set the same (buffers.v1.file).jvm_package on every file of the package",
			})
		}
	}

	out := make([]jvmPackage, 0, len(seen))
	for proto, jvm := range seen {
		out = append(out, jvmPackage{Proto: proto, JVM: jvm})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Proto < out[j].Proto })
	return out
}

// roots derives the tree-shaking roots and, for each, the reason it is one.
//
// The reason is emitted as a trailing comment. A roots list without them is a
// wall of fully qualified names that nobody can review; with them, a stale entry
// is visible as a line whose justification no longer holds.
func (r *run) roots() ([]string, map[string]string) {
	why := map[string]string{}

	for _, f := range r.schema.Files {
		for _, s := range f.Services {
			if s.Skip || !allows(s.Targets) {
				continue
			}
			why[s.Package+"."+s.Name] = "service"

			// A topic payload is reachable from its method, so rooting it is
			// redundant for tree shaking — but it is named here because a
			// publication is part of the API surface in a way a reviewer scanning
			// this list should see.
			for _, m := range s.Methods {
				if m.Transport == buffers.TransportTopic && m.Output != nil {
					why[string(m.Output.Node)] = fmt.Sprintf("payload of topic %q", m.Topic)
				}
			}
		}
		for _, m := range flattenMessages(f) {
			if m.Resource == nil {
				continue
			}
			if _, taken := why[string(m.Node)]; !taken {
				why[string(m.Node)] = "AIP-123 resource " + m.Resource.Type
			}
		}
	}

	roots := make([]string, 0, len(why))
	for root := range why {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots, why
}
