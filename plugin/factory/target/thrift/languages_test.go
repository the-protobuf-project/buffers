package thrift

import (
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestDeclaredLanguagesExist checks Languages() against the installed compiler
// rather than against Thrift's documentation.
//
// It exists because the first version of that list was transcribed from docs and
// was wrong in both directions: it claimed `swift` and `hs`, which this compiler
// does not have, and omitted `kotlin`, which it does — so `buffers generate
// --lang kotlin` was refused for a generator that works, and `--lang swift` was
// accepted for one that does not, failing later inside thrift with a message
// about generation rather than about the flag.
//
// Only the dangerous direction fails. Claiming a generator the compiler lacks is
// a broken promise made by this repository. The reverse — the compiler having one
// this list omits — is reported and not failed, because the generator set grows
// between Thrift releases and a contributor on an older or newer thrift should
// not get a red build for it.
func TestDeclaredLanguagesExist(t *testing.T) {
	have := installedGenerators(t)

	var declared []string
	for _, lang := range (&Target{}).Languages() {
		if lang != schemaOnly {
			declared = append(declared, lang)
		}
	}

	for _, lang := range declared {
		if !have[lang] {
			t.Errorf("Languages() claims %q, which the installed thrift cannot generate; "+
				"a --lang using it would be accepted here and then fail inside thrift", lang)
		}
	}

	claimed := map[string]bool{}
	for _, lang := range declared {
		claimed[lang] = true
	}
	var missing []string
	for lang := range have {
		if !claimed[lang] {
			missing = append(missing, lang)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Logf("the installed thrift also generates %s, which Languages() does not offer; "+
			"add them if this build's thrift is the one to track", strings.Join(missing, ", "))
	}
}

// installedGenerators reads the generator names out of `thrift --help`, or skips.
//
// The section lists one generator per stanza, as an indented name followed by a
// parenthesised description, with its options indented further beneath it — so
// the name is matched at a known depth rather than by scanning for words.
func installedGenerators(t *testing.T) map[string]bool {
	t.Helper()

	if _, err := exec.LookPath("thrift"); err != nil {
		t.Skip("thrift is not on PATH; skipping the generator check")
	}
	// thrift prints its usage and exits non-zero, so the error is ignored and
	// only the output read.
	out, _ := exec.Command("thrift", "--help").CombinedOutput()

	line := regexp.MustCompile(`(?m)^  ([a-z_0-9]+) \(`)
	have := map[string]bool{}
	for _, m := range line.FindAllStringSubmatch(string(out), -1) {
		have[m[1]] = true
	}
	if len(have) == 0 {
		t.Skip("could not read the generator list out of `thrift --help`")
	}
	return have
}
