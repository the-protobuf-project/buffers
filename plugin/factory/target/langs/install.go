package langs

// install.go turns "that compiler is not on PATH" into the one
// line this particular machine can paste.
//
// The reason this is more than a constant string is that the useful answer
// differs three ways at once: by operating system, by the package manager
// actually installed on it, and by compiler. A single line naming brew, apt, dnf,
// pacman and apk at once is technically complete and is read by nobody — the
// reader has to find their own case in it before they can act, which is the work
// this package is supposed to do for them.
//
// So the recipes are declared per manager, the manager is detected by looking for
// its binary rather than by guessing from GOOS alone, and what gets printed is
// the one command that will work here. When nothing matches — an unrecognized
// distribution, or Windows, where these three projects do not have package names
// stable enough to name honestly — the upstream install page is printed instead,
// which is a worse answer and an accurate one.

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Install describes how to obtain one compiler.
type Install struct {
	// By maps a package manager to the command that installs the compiler there.
	// A manager absent from the map has no recipe and falls back to Docs.
	By map[string]string

	// Docs is the upstream install page, printed when no recipe matches this
	// machine.
	Docs string

	// Notes are extra lines shown alongside whichever recipe is chosen — the
	// second package people miss, and similar.
	Notes []string
}

// managers lists the package managers worth trying, per operating system, in the
// order to prefer them.
//
// brew appears under linux as well as darwin, and last rather than first: it runs
// there, and someone who has it usually also has the distribution's own manager,
// which is the one they would rather be told about.
var managers = map[string][]string{
	"darwin": {"brew", "port"},
	"linux":  {"apt", "dnf", "pacman", "apk", "brew"},
}

// zypper is deliberately absent, and the omission is the rule this file follows
// rather than an oversight. Detecting a manager and then having no recipe for it
// is worse than not detecting it: the reader is told a documentation link while a
// command sits two lines away, and the obvious fix — inventing a package name —
// hands them something that fails. openSUSE therefore falls through to the docs
// and the full recipe list, which is a worse answer and a true one. A verified
// package name is all it takes to add it; install_test.go enforces the pairing.

// managerBinaries maps a manager to the executable whose presence proves it.
//
// apt is looked up as apt-get rather than apt: both exist on a Debian system, and
// apt-get is the one that has always been there and the one a script should use.
var managerBinaries = map[string]string{
	"apt":    "apt-get",
	"dnf":    "dnf",
	"pacman": "pacman",
	"apk":    "apk",
	"brew":   "brew",
	"port":   "port",
}

// managerCache memoizes the detection, which shells out to look for binaries and
// is asked once per target by the `targets` and `doctor` listings.
var managerCache struct {
	sync.Once
	name string
}

// Manager reports the package manager to give instructions for on this machine,
// or "" when none is recognized.
func Manager() string {
	managerCache.Do(func() {
		for _, name := range managers[runtime.GOOS] {
			if _, err := exec.LookPath(managerBinaries[name]); err == nil {
				managerCache.name = name
				return
			}
		}
	})
	return managerCache.name
}

// Line returns the single instruction to show on this machine.
//
// It is one line and not a menu: the caller has already established that the
// compiler is missing, and the next thing that should happen is a paste.
func (in Install) Line() string {
	if cmd, ok := in.By[Manager()]; ok {
		return cmd
	}
	return "see " + in.Docs
}

// Detail returns the instruction with any notes beneath it, indented for an error
// message.
func (in Install) Detail(indent string) string {
	pad := indent + strings.Repeat(" ", len("install: "))

	lines := []string{indent + "install: " + in.Line()}
	for _, note := range in.Notes {
		lines = append(lines, pad+note)
	}
	if Manager() == "" && len(in.By) > 0 {
		lines = append(lines, pad+"no package manager recognized here; the recipes this build knows:")
		lines = append(lines, in.recipes(pad+"  ")...)
	}
	return strings.Join(lines, "\n")
}

// recipes lists every known command, one per line and aligned, for the case where
// none of them applies and the reader has to pick or translate one.
//
// One per line rather than joined: a machine that reaches this branch is one this
// build does not recognize, so the reader is scanning for their own case, and a
// single line of six semicolon-separated commands is the shape that makes that
// hardest.
func (in Install) recipes(indent string) []string {
	names := make([]string, 0, len(in.By))
	width := 0
	for name := range in.By {
		names = append(names, name)
		if len(name) > width {
			width = len(name)
		}
	}
	sort.Strings(names)

	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, fmt.Sprintf("%s%-*s  %s", indent, width, name, in.By[name]))
	}
	return out
}

// InstallFor returns the install recipe for a target's compiler, and whether the
// target has one at all.
func InstallFor(target string) (Install, bool) {
	t, ok := tools[target]
	return t.Install, ok
}
