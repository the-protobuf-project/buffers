package langs

import (
	"strings"
	"testing"
)

// TestInstallLinePrefersTheDetectedManager checks that the message names the one
// command this machine takes, rather than a menu the reader has to search.
func TestInstallLinePrefersTheDetectedManager(t *testing.T) {
	in := Install{
		Docs: "https://example.invalid/install",
		By:   map[string]string{Manager(): "the right command"},
	}
	if Manager() == "" {
		t.Skip("no recognized package manager on this machine")
	}
	if got := in.Line(); got != "the right command" {
		t.Errorf("Line() = %q, want the detected manager's command", got)
	}
}

// TestInstallLineFallsBackToDocs checks the honest answer for a machine with no
// recipe: a link, rather than a command invented for a package manager that may
// not name the package that way.
func TestInstallLineFallsBackToDocs(t *testing.T) {
	in := Install{Docs: "https://example.invalid/install", By: map[string]string{"nonesuch": "x"}}
	got := in.Line()
	if !strings.Contains(got, "https://example.invalid/install") {
		t.Errorf("Line() = %q, want the docs URL", got)
	}
}

// TestInstallDetailCarriesNotes checks that the extra package people miss travels
// with the command rather than living only in a comment.
func TestInstallDetailCarriesNotes(t *testing.T) {
	in := Install{
		Docs:  "https://example.invalid",
		By:    map[string]string{Manager(): "install it"},
		Notes: []string{"and the -dev package too"},
	}
	got := in.Detail("  ")
	if !strings.Contains(got, "and the -dev package too") {
		t.Errorf("Detail() dropped the note:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "  ") {
			t.Errorf("Detail() line is not indented: %q", line)
		}
	}
}

// TestEveryToolHasARecipeForEveryManagerWeDetect is the check that keeps the
// detection and the recipes from drifting apart: detecting apt and then having
// nothing to say about it is worse than not detecting it, because the reader is
// told a link when a command exists two lines away.
func TestEveryToolHasARecipeForEveryManagerWeDetect(t *testing.T) {
	detectable := map[string]bool{}
	for _, list := range managers {
		for _, name := range list {
			detectable[name] = true
		}
	}

	for target, tool := range tools {
		for name := range detectable {
			if _, ok := tool.Install.By[name]; !ok {
				t.Errorf("%s: no install recipe for %q, which the detector can report", target, name)
			}
		}
		if tool.Install.Docs == "" {
			t.Errorf("%s: no docs URL to fall back to", target)
		}
	}
}

// TestManagerBinariesCoverEveryManager checks that a manager listed for an OS can
// actually be looked for.
func TestManagerBinariesCoverEveryManager(t *testing.T) {
	for goos, list := range managers {
		for _, name := range list {
			if managerBinaries[name] == "" {
				t.Errorf("%s lists %q, which has no binary to look up", goos, name)
			}
		}
	}
}
