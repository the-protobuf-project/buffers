package thrift

import (
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// coreGenerators are the ones every Thrift this repository supports has had for
// years, and the ones a user is most likely to ask for.
//
// They are asserted because the interesting failure is not drift, it is an edit
// that drops one — `kotlin` was missing from the original list for exactly that
// reason, and `--lang kotlin` was refused for a generator that works. Everything
// outside this set moves between releases and is reported rather than asserted.
var coreGenerators = []string{
	"cpp", "dart", "go", "java", "js", "netstd", "perl", "php", "py", "rb", "rs",
}

// TestLanguagesCoversTheCoreGenerators is the version-stable half of the check.
func TestLanguagesCoversTheCoreGenerators(t *testing.T) {
	declared := declaredLanguages()
	for _, want := range coreGenerators {
		if !declared[want] {
			t.Errorf("Languages() no longer offers %q, which every supported thrift can generate", want)
		}
	}
}

// TestLanguagesIsWellFormed catches the ordinary editing mistakes a long literal
// list invites: a duplicate, or an entry that is not a generator name.
func TestLanguagesIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	name := regexp.MustCompile(`^[a-z_0-9]+$`)

	for _, lang := range (&Target{}).Languages() {
		if lang == schemaOnly {
			continue
		}
		if seen[lang] {
			t.Errorf("Languages() lists %q twice", lang)
		}
		seen[lang] = true
		if !name.MatchString(lang) {
			t.Errorf("Languages() lists %q, which is not a thrift generator name", lang)
		}
	}
}

// TestLanguagesAgainstInstalledThrift reports how the declared superset differs
// from the thrift on this machine, and deliberately fails on neither direction.
//
// An earlier version of this test asserted the two matched, and that was wrong in
// a way worth recording. Thrift 0.24 generates `mmd` and has no `swift`; the
// thrift Ubuntu packages generates `swift` and has no `mmd`. The assertion passed
// locally and failed in CI, for a list that was correct — the set is a property
// of the installed binary, not of this repository, and the code now treats it
// that way. What is left here is a drift report worth reading when the list is
// next edited.
func TestLanguagesAgainstInstalledThrift(t *testing.T) {
	have := installedGenerators(t)
	declared := declaredLanguages()

	var claimedNotHere, hereNotClaimed []string
	for lang := range declared {
		if !have[lang] {
			claimedNotHere = append(claimedNotHere, lang)
		}
	}
	for lang := range have {
		if !declared[lang] {
			hereNotClaimed = append(hereNotClaimed, lang)
		}
	}
	sort.Strings(claimedNotHere)
	sort.Strings(hereNotClaimed)

	if len(claimedNotHere) > 0 {
		t.Logf("listed but absent from this thrift: %s — a run asking for one gets "+
			"langs.checkThriftGenerator's message naming what this build does offer",
			strings.Join(claimedNotHere, ", "))
	}
	if len(hereNotClaimed) > 0 {
		t.Logf("this thrift also generates: %s — add them to Languages() if they are "+
			"worth offering", strings.Join(hereNotClaimed, ", "))
	}
}

// declaredLanguages is Languages() as a set, without the schema-only entry.
func declaredLanguages() map[string]bool {
	out := map[string]bool{}
	for _, lang := range (&Target{}).Languages() {
		if lang != schemaOnly {
			out[lang] = true
		}
	}
	return out
}

// installedGenerators reads the generator names out of `thrift --help`, or skips.
func installedGenerators(t *testing.T) map[string]bool {
	t.Helper()

	if _, err := exec.LookPath("thrift"); err != nil {
		t.Skip("thrift is not on PATH; skipping the generator report")
	}
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
