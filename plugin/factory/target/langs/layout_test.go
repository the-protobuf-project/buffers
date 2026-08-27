package langs

import (
	"reflect"
	"testing"
)

// layout_test.go covers where generated code lands and what package it declares.
//
// This is the part of the pipeline that has no golden file: the schema is
// committed, the compiled language output is not, so a regression in the layout
// shows up only when someone tries to build the result. These assert the rules
// directly.
func TestPlanMirrorsSourceTreeForFlatLanguages(t *testing.T) {
	groups := []Group{
		{Dir: "sensors/v1", Files: []string{"sensors/v1/sensors.fbs"}, Namespace: "sensors.v1"},
		{Dir: "buffers", Files: []string{"buffers/wellknown.fbs"}, Namespace: "buffers.wellknown"},
	}

	// C++ emits one flat file per schema, so without per-directory invocation
	// every schema in the tree lands in one directory and same-named files
	// collide.
	got := Plan("flatbuffers", "cpp", "schema", "out", groups)
	if len(got) != 2 {
		t.Fatalf("got %d invocations, want one per source directory", len(got))
	}
	if got[0].OutDir != "out/sensors/v1" {
		t.Errorf("OutDir = %q, want out/sensors/v1 — output must mirror the proto tree", got[0].OutDir)
	}
	if got[1].OutDir != "out/buffers" {
		t.Errorf("OutDir = %q, want out/buffers", got[1].OutDir)
	}
}

func TestPlanDoesNotDoubleNestSelfNestingLanguages(t *testing.T) {
	groups := []Group{{Dir: "sensors/v1", Files: []string{"sensors/v1/sensors.fbs"}, Namespace: "sensors.v1"}}

	// flatc builds sensors/v1/ itself for these. Adding the directory again would
	// yield out/sensors/v1/sensors/v1/.
	for _, lang := range []string{"go", "java", "kotlin", "python"} {
		got := Plan("flatbuffers", lang, "schema", "out", groups)
		if got[0].OutDir != "out" {
			t.Errorf("%s: OutDir = %q, want out — the generator nests by namespace itself",
				lang, got[0].OutDir)
		}
	}
}

func TestFlatLanguagesKeepIncludePrefixes(t *testing.T) {
	groups := []Group{{Dir: "sensors/v1", Files: []string{"sensors/v1/sensors.fbs"}}}

	// The other half of mirroring the tree. flatc writes bare includes by
	// default, which resolve only while every file shares one directory; spread
	// them out and the generated C++ no longer compiles.
	got := Plan("flatbuffers", "cpp", "schema", "out", groups)
	if !hasFlag(got[0].Flags, "--keep-prefix") {
		t.Errorf("cpp flags %v lack --keep-prefix; cross-directory includes will not resolve", got[0].Flags)
	}

	// A generator that nests by namespace resolves imports by package name and
	// needs nothing.
	got = Plan("flatbuffers", "go", "schema", "out", groups)
	if hasFlag(got[0].Flags, "--keep-prefix") {
		t.Errorf("go flags %v carry --keep-prefix, which applies to path-based includes only", got[0].Flags)
	}
}

func TestPackageFlagsFollowTheProtoOptions(t *testing.T) {
	g := Group{
		Dir:        "sensors/v1",
		Files:      []string{"sensors/v1/sensors.fbs"},
		Namespace:  "sensors.v1",
		JVMPackage: "com.sensors.v1",
		GoPackage:  "sensorsv1",
	}

	// Java: flatc derives the package from the namespace, so java_package is
	// reached by passing the prefix that turns one into the other.
	java := Plan("flatbuffers", "java", "schema", "out", []Group{g})[0].Flags
	if !reflect.DeepEqual(java, []string{"--java-package-prefix", "com"}) {
		t.Errorf("java flags = %v, want --java-package-prefix com", java)
	}

	// Go: the package name is the go_package alias, matching protoc-gen-go, and
	// the module root is what makes cross-package imports resolve at all. Omitting
	// the second produces output flatc exits 0 on and the Go compiler rejects with
	// "package buffers/wellknown is not in std".
	//
	// The module flag is conditional on the installed flatc knowing it — an older
	// one rejects the flag outright — so the assertion follows the same capability
	// check the code does rather than pinning to whatever is on this machine.
	g.GoModule = "example.com/gen"
	golang := Plan("flatbuffers", "go", "schema", "out", []Group{g})[0].Flags

	want := []string{"--go-namespace", "sensorsv1"}
	if FlatcSupportsGoModule() {
		want = append(want, "--go-module-name", "example.com/gen")
	}
	if !reflect.DeepEqual(golang, want) {
		t.Errorf("go flags = %v, want %v", golang, want)
	}

	// Kotlin: flatc has no Kotlin package option and ignores the Java one, so
	// passing it would imply the package was honoured when it is not.
	kotlin := Plan("flatbuffers", "kotlin", "schema", "out", []Group{g})[0].Flags
	if hasFlag(kotlin, "--java-package-prefix") {
		t.Errorf("kotlin flags %v pass a Java-only option flatc ignores", kotlin)
	}
}

func TestJVMPrefixOnlyWhenReconcilable(t *testing.T) {
	for _, tc := range []struct {
		jvm, namespace, want string
		ok                   bool
	}{
		{"com.sensors.v1", "sensors.v1", "com", true},
		{"com.example.sensors.v1", "sensors.v1", "com.example", true},

		// java_package that does not end with the namespace cannot be produced by
		// prepending anything, so no flag is guessed at.
		{"com.example.api", "sensors.v1", "", false},

		// Nothing to do.
		{"sensors.v1", "sensors.v1", "", false},
		{"", "sensors.v1", "", false},
	} {
		got, ok := jvmPrefix(tc.jvm, tc.namespace)
		if ok != tc.ok || got != tc.want {
			t.Errorf("jvmPrefix(%q, %q) = (%q, %v), want (%q, %v)",
				tc.jvm, tc.namespace, got, ok, tc.want, tc.ok)
		}
	}

	// The reconcilable check is what the CLI warns on, so it must agree.
	if JVMPackageReconcilable("com.example.api", "sensors.v1") {
		t.Error("an unreachable java_package reported as reconcilable; no warning would be issued")
	}
	if !JVMPackageReconcilable("com.sensors.v1", "sensors.v1") {
		t.Error("a reachable java_package reported as unreconcilable; a spurious warning would be issued")
	}
}

func TestKotlinNeverHonoursADistinctJVMPackage(t *testing.T) {
	if KotlinHonoursJVMPackage("com.sensors.v1", "sensors.v1") {
		t.Error("claimed flatc emits java_package for Kotlin; it has no Kotlin package option")
	}
	if !KotlinHonoursJVMPackage("sensors.v1", "sensors.v1") {
		t.Error("a java_package equal to the namespace needs no warning")
	}
}

func TestGroupFilesBucketsByDirectory(t *testing.T) {
	got := GroupFiles([]string{
		"sensors/v1/sensors.fbs",
		"buffers/wellknown.fbs",
		"sensors/v1/enums.fbs",
		"root.fbs",
	})

	want := map[string][]string{
		"sensors/v1": {"sensors/v1/enums.fbs", "sensors/v1/sensors.fbs"},
		"buffers":    {"buffers/wellknown.fbs"},
		"":           {"root.fbs"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GroupFiles = %v, want %v", got, want)
	}
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}
