package langs

// typescript_test.go pins the layout rules TypeScript needs on both toolchains,
// and the one place it does not work.
//
// The nesting entry is the load-bearing one. It was NestFlat, which is what a
// language that writes one file per schema wants, and TypeScript is not that on
// either toolchain: flatc --ts builds the namespace tree itself, and capnp mirrors
// the schema tree for every generator it drives. Mirroring on top of either
// doubles the path, and the generated imports — which climb back to the output
// root — then resolve against the wrong directory.

import "testing"

// tsGroups is the two-directory shape the doubling shows up in. A single group at
// the root would pass under either nesting rule.
var tsGroups = []Group{
	{Dir: "sensors/v1", Files: []string{"sensors/v1/sensors.fbs"}, Namespace: "sensors.v1"},
	{Dir: "buffers", Files: []string{"buffers/wellknown.fbs"}, Namespace: "buffers.wellknown"},
}

// TestTypeScriptNestsByNamespaceOnBothToolchains is the regression guard for the
// doubled path.
//
// Verified against flatc 25.12.19: `flatc --ts -o out -I . sensors/v1/sensors.fbs`
// writes out/sensors/v1/sensor.ts on its own. Adding the group directory would
// write out/sensors/v1/sensors/v1/sensor.ts, and reading.ts there imports
// '../../buffers/wellknown/timestamp.js', which resolves to
// out/sensors/v1/buffers/... and fails with TS2307.
func TestTypeScriptNestsByNamespaceOnBothToolchains(t *testing.T) {
	for _, target := range []string{"flatbuffers", "capnp"} {
		for _, lang := range []string{"ts", "typescript"} {
			if got := NestingOf(target, lang); got != NestByNamespace {
				t.Errorf("NestingOf(%q, %q) = %v, want NestByNamespace — "+
					"the generator builds the tree itself and mirroring doubles it",
					target, lang, got)
			}
			for _, req := range Plan(target, lang, "schema", "out", tsGroups) {
				if req.OutDir != "out" {
					t.Errorf("%s/%s: OutDir = %q, want out", target, lang, req.OutDir)
				}
			}
		}
	}
}

// TestTypeScriptTakesNoIncludePrefix guards the other half.
//
// --keep-prefix rewrites C++ include statements to carry their path. flatc
// accepts it alongside --ts and it means nothing there; passing it would only
// suggest the TypeScript imports were being managed when they are the
// generator's own business.
func TestTypeScriptTakesNoIncludePrefix(t *testing.T) {
	for _, req := range Plan("flatbuffers", "ts", "schema", "out", tsGroups) {
		if hasFlag(req.Flags, "--keep-prefix") {
			t.Errorf("ts flags %v carry --keep-prefix, which applies to path-based includes only",
				req.Flags)
		}
	}
}

// TestBothTypeScriptSpellingsReachAGenerator checks the two spellings stay in
// step.
//
// flatcFlags carried "ts" and "typescript" while capnpGenerators carried only
// "ts", so `languages: [typescript]` produced FlatBuffers output and failed the
// capnp entry with "capnp has no generator for \"typescript\"" — a difference in
// spelling reported as a difference in support.
func TestBothTypeScriptSpellingsReachAGenerator(t *testing.T) {
	for _, lang := range []string{"ts", "typescript"} {
		if _, ok := flatcFlags[lang]; !ok {
			t.Errorf("flatcFlags has no %q", lang)
		}
		if _, ok := capnpGenerators[lang]; !ok {
			t.Errorf("capnpGenerators has no %q", lang)
		}
	}
	if flatcFlags["ts"] != flatcFlags["typescript"] {
		t.Error("the two flatc spellings resolve to different flags")
	}
	if capnpGenerators["ts"] != capnpGenerators["typescript"] {
		t.Error("the two capnp spellings resolve to different generators")
	}
}

// TestCapnpLanguagesListsEachSpellingOnce keeps `buffers doctor` honest.
//
// The listing names one binary to install per row, so a generator appearing under
// two spellings of one language — ts and typescript, cpp and c++ — reads as two
// separate things to go and get.
//
// Two languages sharing a generator is a different case and stays: capnp has no
// Kotlin backend, so Kotlin is produced by capnpc-java and consumed over JVM
// interop. Dropping its row would report Kotlin as unreachable, which is wrong.
func TestCapnpLanguagesListsEachSpellingOnce(t *testing.T) {
	listed := map[string]bool{}
	for _, lang := range CapnpLanguages() {
		if capnpSpellingAliases[lang] {
			t.Errorf("%q is an alias spelling and should not be listed separately", lang)
		}
		listed[lang] = true
	}

	// Every language that is not an alias must still appear, or a supported
	// generator would be invisible in the one place that reports them.
	for lang := range capnpGenerators {
		if !capnpSpellingAliases[lang] && !listed[lang] {
			t.Errorf("%q has a generator but is missing from CapnpLanguages()", lang)
		}
	}
	if !listed["ts"] {
		t.Error("ts is missing from CapnpLanguages(); doctor would not report capnpc-ts")
	}
}

// TestCapnpTypeScriptNestedImportsFiresOnlyWhenNested covers the warning's
// trigger.
//
// Both capnp TypeScript generators write a cross-file import relative to the
// schema root while capnp writes the file into the mirrored tree, so the two
// agree only at the root. See capnpts.go.
func TestCapnpTypeScriptNestedImportsFiresOnlyWhenNested(t *testing.T) {
	root := []Group{{Dir: "", Files: []string{"sensors.capnp"}}}

	for _, tc := range []struct {
		target, lang string
		groups       []Group
		want         bool
	}{
		{"capnp", "ts", tsGroups, true},
		{"capnp", "typescript", tsGroups, true},

		// A single-directory schema is the case that works: the schema root and
		// the generated file's directory coincide, so the import resolves.
		{"capnp", "ts", root, false},

		// An empty group names no files and generates nothing to break.
		{"capnp", "ts", []Group{{Dir: "sensors/v1"}}, false},

		// flatc resolves its own TypeScript imports correctly once the tree is
		// not doubled, so the warning must not follow the language across targets.
		{"flatbuffers", "ts", tsGroups, false},

		// Other capnp languages are unaffected; capnpc-go and capnpc-rust write
		// imports the tree supports.
		{"capnp", "go", tsGroups, false},
	} {
		if got := CapnpTypeScriptNestedImports(tc.target, tc.lang, tc.groups); got != tc.want {
			t.Errorf("CapnpTypeScriptNestedImports(%q, %q, %d groups) = %v, want %v",
				tc.target, tc.lang, len(tc.groups), got, tc.want)
		}
	}
}
