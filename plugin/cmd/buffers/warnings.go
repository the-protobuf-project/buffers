package main

// warnings.go holds the checks that report a degraded result the run is going to
// produce anyway.
//
// Each exists for the same reason: the toolchain's own symptom appears far from
// its cause. A Kotlin package that silently differs from the Java one, generated
// Go that fails with "not in std", a TypeScript import that resolves to nothing —
// all three are discovered long after the generate that caused them, by someone
// with no reason to suspect their compiler. Saying it once, here, is cheaper than
// any of those.
//
// None of them stop the run. They describe what is about to be produced, and the
// output is still worth having.

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/the-protobuf-project/buffers/plugin/factory/config"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/langs"
)

// warnUnreconcilableJVM reports a java_package flatc cannot be made to emit.
//
// flatc has no "set the package" flag, only a prefix prepended to the namespace
// it derived. A java_package that does not end with the namespace therefore
// cannot be reproduced, and the generated code would declare a package subtly
// different from the one the rest of that proto's output uses — the kind of
// mismatch that surfaces as an unresolved import much later.
func warnUnreconcilableJVM(cmd *cobra.Command, groups []langs.Group, languages []string) {
	var java, kotlin bool
	for _, l := range languages {
		switch l {
		case "java":
			java = true
		case "kotlin", "kotlin-kmp":
			kotlin = true
		}
	}

	for _, g := range groups {
		if java && !langs.JVMPackageReconcilable(g.JVMPackage, g.Namespace) {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"warning: %s declares java_package %q, which flatc cannot emit over namespace %q "+
					"— it supports only a prefix prepended to the namespace, and %q is not one. "+
					"The generated package will be %q.\n"+
					"    fix: make java_package end with the proto package, or set "+
					"(buffers.v1.file).namespace to match\n",
				g.Dir, g.JVMPackage, g.Namespace, g.JVMPackage, g.Namespace)
		}
		if kotlin && !langs.KotlinHonoursJVMPackage(g.JVMPackage, g.Namespace) {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"warning: %s declares java_package %q, but flatc has no Kotlin package option "+
					"— the Java one does not apply — so the Kotlin output will declare %q. "+
					"Java and Kotlin generated from this schema will not share a package.\n"+
					"    fix: set (buffers.v1.file).namespace to %q if the two must agree\n",
				g.Dir, g.JVMPackage, g.Namespace, g.JVMPackage)
		}
	}
}

// warnOldFlatcForGo reports a flatc too old to make generated Go compile.
//
// The flag it needs, --go-module-name, arrived in flatbuffers 23.1.4, and Ubuntu
// still ships 2.0.8. Omitting it is what the run has to do — an older flatc
// rejects an unknown flag rather than ignoring it — so the output is produced and
// the limitation stated, instead of the user meeting "package buffers/wellknown
// is not in std" with nothing to connect it to their toolchain.
func warnOldFlatcForGo(cmd *cobra.Command, e config.Entry, languages []string) {
	if e.Target != "flatbuffers" || langs.FlatcSupportsGoModule() {
		return
	}
	for _, l := range languages {
		if l == "go" {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", langs.GoModuleFlagHint)
			return
		}
	}
}

// warnCapnpTypeScript reports Cap'n Proto TypeScript output whose imports will
// not resolve.
//
// Both generators write a cross-file import relative to the schema root while
// capnp writes the file itself into the mirrored schema tree, so every generated
// file below the root imports a path that does not exist. The code is still
// emitted — the types in each file are correct and a single-directory schema is
// unaffected — so this describes the limit rather than refusing the run. See
// langs/capnpts.go for the reproduction.
func warnCapnpTypeScript(cmd *cobra.Command, e config.Entry, groups []langs.Group, languages []string) {
	for _, l := range languages {
		if !langs.CapnpTypeScriptNestedImports(e.Target, l, groups) {
			continue
		}
		fmt.Fprintf(cmd.ErrOrStderr(),
			"warning: capnp %s output is emitted into a nested tree, and both TypeScript "+
				"generators write cross-file imports relative to the schema root rather than to "+
				"the generated file — every file below the root will import a path that does not "+
				"exist (error TS2307).\n"+
				"    fix: %s\n",
			l, langs.CapnpTypeScriptHint())
		return
	}
}
