package registry_test

// generated_thrift_test.go compiles the Go the Thrift compiler produces.
//
// It sits beside generated_test.go rather than in it because the two answer the
// same question about different toolchains, and the shared answer is worth
// stating once: a schema compiler exiting zero says nothing about whether the
// code it wrote builds. Both of these compilers get that wrong the same way — a
// cross-package reference written as a bare path that resolves against no Go
// module — and in both cases the only check that notices is a build.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/langs"
)

// TestGeneratedThriftGoCompiles is the same claim for Thrift, and it exists
// because Thrift has the identical trap under a different spelling.
//
// The Thrift compiler also exits 0 on Go it cannot resolve: without
// package_prefix it writes a cross-package reference as `import "wellknown"`,
// which fails with "package wellknown is not in std" — the same failure flatc
// produces without --go-module-name, from the same cause. The generator knows the
// package tree and not the module root it hangs from, so somebody has to tell it,
// and the only check that notices is a build.
func TestGeneratedThriftGoCompiles(t *testing.T) {
	thrift := requireTool(t, "thrift")
	goBin := requireTool(t, "go")

	const module = "example.com/gen"

	schemaDir := writeTree(t, render(t, "thrift"))
	outDir := t.TempDir()

	var groups []langs.Group
	for dir, files := range langs.GroupFiles(schemaFiles(t, schemaDir, ".thrift")) {
		groups = append(groups, langs.Group{Dir: dir, Files: files, GoModule: module})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Dir < groups[j].Dir })

	for _, req := range langs.Plan("thrift", "go", schemaDir, outDir, groups) {
		// One invocation per file rather than per group: the Thrift compiler
		// takes exactly one input. langs.Run already does this; the plan is
		// replayed by hand here for the same reason the flatc tests replay
		// theirs — to assert the flags the plan produced, not the runner.
		for _, file := range req.Files {
			gen := "go"
			if len(req.Options) > 0 {
				gen += ":" + strings.Join(req.Options, ",")
			}
			args := []string{"--gen", gen, "-out", req.OutDir, "-I", schemaDir, file}
			cmd := exec.Command(thrift, args...)
			cmd.Dir = schemaDir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("thrift %v: %v\n%s", args, err, out)
			}
		}
	}

	gomod := "module " + module + "\n\ngo 1.26\n"
	if err := os.WriteFile(filepath.Join(outDir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	// The Thrift runtime has to be resolvable, which needs the network. A machine
	// without it should skip rather than fail: this asserts the generated code,
	// not the module proxy.
	tidy := exec.Command(goBin, "mod", "tidy")
	tidy.Dir = outDir
	tidy.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Skipf("cannot resolve the thrift runtime (offline?); skipping: %v\n%s", err, firstLines(out, 4))
	}

	build := exec.Command(goBin, "build", "./...")
	build.Dir = outDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Errorf("generated Go does not compile: %v\n%s", err, firstLines(out, 12))
	}
}
