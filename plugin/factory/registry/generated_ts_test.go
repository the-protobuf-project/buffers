package registry_test

// generated_ts_test.go typechecks the TypeScript flatc produces, rather than
// trusting its exit code.
//
// flatc exits 0 on TypeScript whose imports resolve against nothing, exactly as
// it does for Go without --go-module-name. Its --ts backend builds the namespace
// tree itself and writes cross-namespace references as relative paths climbing
// back to the output root:
//
//	import { Timestamp } from '../../buffers/wellknown/timestamp.js';
//
// Mirroring the source tree on top of that — which is what the layout rule did
// before, TypeScript having been recorded as a flat language — puts the file two
// directories deeper than the path assumes:
//
//	sensors/v1/sensors/v1/reading.ts(7,27): error TS2307:
//	  Cannot find module '../../buffers/wellknown/timestamp.js'
//
// Nothing else in the suite sees that. The golden tests cover the .fbs, and
// TestFlatcAcceptsTheSchema compiles into a throwaway directory where the
// question does not arise. This builds the real tree and asks tsc to resolve
// across it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/the-protobuf-project/buffers/plugin/factory/target/langs"
)

// The npm packages this test typechecks against, pinned so a release of either
// cannot turn the build red on a day nothing in this repository changed.
const (
	// tscVersion is the TypeScript compiler the generated code is checked with.
	tscVersion = "typescript@7.0.2"

	// flatbuffersVersion is the runtime every generated module imports. It has to
	// resolve for the imports to typecheck at all.
	flatbuffersVersion = "flatbuffers@25.9.23"
)

// TestGeneratedTypeScriptCompiles renders the schema, runs flatc over it the way
// the CLI does, and typechecks the result under `strict`.
func TestGeneratedTypeScriptCompiles(t *testing.T) {
	flatc := requireTool(t, "flatc")
	npm := requireTool(t, "npm")

	schemaDir := writeTree(t, render(t, "flatbuffers"))
	project := t.TempDir()
	outDir := filepath.Join(project, "gen")

	// The same plan the CLI runs. TypeScript nests by namespace, so every group
	// writes into one output root and flatc builds the tree beneath it.
	var groups []langs.Group
	for dir, files := range langs.GroupFiles(schemaFiles(t, schemaDir, ".fbs")) {
		groups = append(groups, langs.Group{Dir: dir, Files: files})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Dir < groups[j].Dir })

	for _, req := range langs.Plan("flatbuffers", "ts", schemaDir, outDir, groups) {
		args := append([]string{"--ts", "-o", req.OutDir, "-I", schemaDir}, req.Flags...)
		args = append(args, req.Files...)
		cmd := exec.Command(flatc, args...)
		cmd.Dir = schemaDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("flatc %v: %v\n%s", args, err, out)
		}
	}

	// The flatbuffers runtime and tsc have to be resolvable, which needs the
	// network. A machine without it should skip rather than fail: this asserts the
	// generated code, not the registry.
	//
	// Both are pinned exactly. Unpinned, a tsc release that tightened a check
	// would fail this test on a day nothing here changed, and the failure would
	// point at the generated code rather than at the compiler that moved — the
	// same reason the golden tests pin their output. Resolution is deterministic
	// without a lockfile: flatbuffers has no dependencies, and TypeScript's are
	// its own platform binaries, pinned to its exact version.
	//
	// Bump them deliberately. A new major here is worth running by hand first: it
	// is the one signal that a newer tsc rejects what flatc emits.
	install := exec.Command(npm, "install", "--silent", "--no-audit", "--no-fund",
		"--prefix", project, tscVersion, flatbuffersVersion)
	if out, err := install.CombinedOutput(); err != nil {
		t.Skipf("cannot install %s and %s (offline?); skipping: %v\n%s",
			tscVersion, flatbuffersVersion, err, firstLines(out, 4))
	}

	// A consumer module rather than the generated files alone: tsc treats a bare
	// .ts entry point as a script and would not resolve its imports the way an
	// importing module does. Reaching the deepest type pulls the whole tree in.
	probe := filepath.Join(project, "probe.ts")
	source := "import { Reading } from './gen/sensors/v1/reading.js';\n" +
		"import { Sensor } from './gen/sensors/v1/sensor.js';\n" +
		"export const _p: [typeof Reading, typeof Sensor] = [Reading, Sensor];\n"
	if err := os.WriteFile(probe, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	const tsconfig = `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "noEmit": true,
    "skipLibCheck": false
  },
  "files": ["probe.ts"]
}`
	if err := os.WriteFile(filepath.Join(project, "tsconfig.json"), []byte(tsconfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// skipLibCheck is off deliberately. The generated tree is reached through
	// .d.ts-shaped declarations, and leaving it on skips exactly the files under
	// test — an earlier version of this check passed against output whose imports
	// did not resolve at all.
	tsc := exec.Command(filepath.Join(project, "node_modules", ".bin", "tsc"),
		"-p", filepath.Join(project, "tsconfig.json"))
	tsc.Dir = project
	if out, err := tsc.CombinedOutput(); err != nil {
		t.Errorf("generated TypeScript does not typecheck from its tree: %v\n%s",
			err, firstLines(out, 12))
	}
}
