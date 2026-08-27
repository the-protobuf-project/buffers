package langs

// capability.go asks a toolchain what it can do, rather than assuming.
//
// The flag this exists for is --go-module-name, added to flatc in November 2022
// (google/flatbuffers#7651) and absent from anything older — including the 2.0.8
// that Ubuntu still ships. Passing it to a flatc that predates it is not a
// degraded result, it is a hard error:
//
//	error: unknown commandline argument: --go-module-name
//
// And omitting it is not free either: it is what supplies the module root that
// makes a generated Go package's cross-file imports resolve, so without it the
// output compiles only for a single-package schema.
//
// A version comparison would be the obvious check and a bad one. flatc's
// numbering changed schemes partway through — 2.0.8 is *newer* than 1.12 and
// *older* than 23.1.4 — so a parser has to know that 2.x sorts before 23.x, and
// gets it wrong the first time nobody remembers. Asking the binary what flags it
// accepts has no such failure mode.

import (
	"os/exec"
	"strings"
	"sync"
)

// flatcFlagCache memoizes one `flatc --help` per process, since Plan calls the
// probe once per source directory per language.
var flatcFlagCache struct {
	sync.Mutex
	help map[string]string
}

// flatcSupports reports whether the flatc at the given path accepts a flag.
//
// A flatc that cannot be run at all reports false, which is the safe answer: the
// caller omits the flag and the invocation fails on its own terms, with the
// compiler's message rather than one invented here.
func flatcSupports(binary, flag string) bool {
	flatcFlagCache.Lock()
	defer flatcFlagCache.Unlock()

	if flatcFlagCache.help == nil {
		flatcFlagCache.help = map[string]string{}
	}
	help, ok := flatcFlagCache.help[binary]
	if !ok {
		// flatc prints its usage to stdout and exits non-zero for --help on some
		// versions, so the error is deliberately ignored and only the output read.
		out, _ := exec.Command(binary, "--help").CombinedOutput()
		help = string(out)
		flatcFlagCache.help[binary] = help
	}
	return strings.Contains(help, flag)
}

// FlatcSupportsGoModule reports whether the installed flatc accepts
// --go-module-name, which is what makes generated Go's cross-package imports
// resolvable.
//
// Exported so a caller can say why the Go it is about to receive will not
// compile, rather than leaving them to find out from the Go compiler.
func FlatcSupportsGoModule() bool {
	path, err := exec.LookPath("flatc")
	if err != nil {
		return false
	}
	return flatcSupports(path, "--go-module-name")
}

// GoModuleFlagHint explains what an older flatc costs and how to fix it.
const GoModuleFlagHint = "this flatc predates --go-module-name (added in flatbuffers 23.1.4); " +
	"generated Go will import cross-package types by bare namespace path, which resolves " +
	"against no module. Upgrade flatc, or keep the schema to one package."
