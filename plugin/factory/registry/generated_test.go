package registry_test

// generated_test.go compiles the *language code* the toolchains produce, not just
// the schema they accept.
//
// Both cases here shipped as real bugs, and neither was visible to any other
// check. flatc exits 0 on Go whose cross-package imports resolve against no
// module; and a C++ tree laid out to mirror the protos does not build unless the
// include statements were told to carry their paths.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/langs"
)

// TestGeneratedCppCompilesFromAMirroredTree is the test that would have caught
// the include breakage.
//
// Laying output out like protoc — one directory per proto package — is only half
// the job. flatc writes bare include statements by default, which resolve while
// every generated file shares a directory and stop resolving the moment they do
// not:
//
//	sensors/v1/sensors_generated.h:16:10: fatal error:
//	  'wellknown_generated.h' file not found
//
// Nothing else in the suite sees that. The golden tests cover the .fbs, not the
// code flatc produces from it, and TestFlatcAcceptsTheSchema compiles each schema
// into a throwaway directory where the flat layout still works. This one builds
// the real tree and asks a C++ compiler to resolve across it.
func TestGeneratedCppCompilesFromAMirroredTree(t *testing.T) {
	flatc := requireTool(t, "flatc")
	cxx := requireTool(t, "c++")

	includes, err := exec.Command("brew", "--prefix", "flatbuffers").Output()
	if err != nil {
		t.Skip("flatbuffers headers not locatable via brew; skipping the compile check")
	}
	fbInclude := filepath.Join(strings.TrimSpace(string(includes)), "include")

	schemaDir := writeTree(t, render(t, "flatbuffers"))
	outDir := t.TempDir()

	// The same plan the CLI runs: one invocation per source directory, with the
	// include-prefix flag that keeps cross-directory references resolvable.
	var groups []langs.Group
	for dir, files := range langs.GroupFiles(schemaFiles(t, schemaDir, ".fbs")) {
		groups = append(groups, langs.Group{Dir: dir, Files: files})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Dir < groups[j].Dir })

	for _, req := range langs.Plan("flatbuffers", "cpp", schemaDir, outDir, groups) {
		args := append([]string{"--cpp", "-o", req.OutDir, "-I", schemaDir}, req.Flags...)
		args = append(args, req.Files...)
		cmd := exec.Command(flatc, args...)
		cmd.Dir = schemaDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("flatc %v: %v\n%s", args, err, out)
		}
	}

	// Including the deepest header pulls in every other one through the generated
	// include statements, so one translation unit exercises the whole tree.
	probe := filepath.Join(t.TempDir(), "probe.cc")
	source := "#include \"sensors/v1/sensors_generated.h\"\nint main() { return 0; }\n"
	if err := os.WriteFile(probe, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(cxx, "-std=c++17", "-I", outDir, "-I", fbInclude, "-fsyntax-only", probe).CombinedOutput()
	if err != nil {
		t.Errorf("generated C++ does not compile from its mirrored tree: %v\n%s", err, firstLines(out, 12))
	}
}

// TestGeneratedGoCompiles builds the generated Go rather than trusting flatc's
// exit code.
//
// flatc exits 0 on output that does not build. Cross-package references are
// written as bare namespace paths — `import "buffers/wellknown"` — which resolve
// against no module and fail with "package buffers/wellknown is not in std"
// unless --go-module-name supplies the root. That shipped once, undetected,
// because every check in the suite stopped at the schema.
func TestGeneratedGoCompiles(t *testing.T) {
	flatc := requireTool(t, "flatc")
	goBin := requireTool(t, "go")

	// Without --go-module-name the generated Go imports cross-package types by
	// bare namespace path, which resolves against no module — so this cannot pass
	// on a flatc that predates the flag, and the honest signal is a skip naming
	// the reason rather than a failure implying the generator is wrong.
	if !langs.FlatcSupportsGoModule() {
		t.Skip("flatc predates --go-module-name (added in flatbuffers 23.1.4); " +
			"generated Go cannot resolve cross-package imports without it")
	}

	const module = "example.com/gen"

	schemaDir := writeTree(t, render(t, "flatbuffers"))
	outDir := t.TempDir()

	var groups []langs.Group
	for dir, files := range langs.GroupFiles(schemaFiles(t, schemaDir, ".fbs")) {
		groups = append(groups, langs.Group{
			Dir:      dir,
			Files:    files,
			GoModule: module,
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Dir < groups[j].Dir })

	for _, req := range langs.Plan("flatbuffers", "go", schemaDir, outDir, groups) {
		args := append([]string{"--go", "-o", req.OutDir, "-I", schemaDir}, req.Flags...)
		args = append(args, req.Files...)
		cmd := exec.Command(flatc, args...)
		cmd.Dir = schemaDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("flatc %v: %v\n%s", args, err, out)
		}
	}

	gomod := "module " + module + "\n\ngo 1.26\n"
	if err := os.WriteFile(filepath.Join(outDir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	// The flatbuffers runtime has to be resolvable, which needs the network. A
	// machine without it should skip rather than fail: this asserts the generated
	// code, not the module proxy.
	tidy := exec.Command(goBin, "mod", "tidy")
	tidy.Dir = outDir
	tidy.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Skipf("cannot resolve the flatbuffers runtime (offline?); skipping: %v\n%s", err, firstLines(out, 4))
	}

	build := exec.Command(goBin, "build", "./...")
	build.Dir = outDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Errorf("generated Go does not compile: %v\n%s", err, firstLines(out, 12))
	}
}
