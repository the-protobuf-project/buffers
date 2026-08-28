package langs

// thriftgen.go asks the installed thrift which generators it has, because the
// answer moves between releases and no list compiled into this repository can
// track it.
//
// The evidence for that is concrete. Thrift 0.24 generates `mmd` (Mermaid) and
// has no `swift`; the thrift Ubuntu currently packages generates `swift` and has
// no `mmd`. Both are ordinary, supported versions. A static list is therefore
// wrong on one of them whatever it says, and a test asserting the list matches
// the binary fails on whichever machine is not the one it was written on.
//
// So Target.Languages() is a superset — every generator any supported thrift is
// known to offer — and this is what decides whether a particular run can go
// ahead. The compiler is the only authority on what the compiler can do.
//
// The same reasoning as capability.go, arrived at from the other direction:
// there it was a flag that a version comparison would get wrong, here it is a
// generator set. Both are answered by asking the binary.

import (
	"os/exec"
	"regexp"
	"sort"
	"sync"
)

// thriftGenCache memoizes one `thrift --help` per process.
var thriftGenCache struct {
	sync.Once
	names map[string]bool
}

// thriftGenLine matches a generator's name in the "Available generators" section,
// which lists each as an indented name followed by a parenthesised description,
// with its options indented further beneath.
var thriftGenLine = regexp.MustCompile(`(?m)^  ([a-z_0-9]+) \(`)

// ThriftGenerators returns the generator names the installed thrift offers.
//
// A thrift that cannot be run, or whose help cannot be parsed, returns nil — and
// every caller treats that as "cannot tell", allowing the run to proceed so the
// compiler reports on its own terms. Guessing "unsupported" from a failed probe
// would refuse work that would have succeeded.
func ThriftGenerators() map[string]bool {
	thriftGenCache.Do(func() {
		path, err := exec.LookPath("thrift")
		if err != nil {
			return
		}
		// thrift prints its usage and exits non-zero for --help, so the error is
		// deliberately ignored and only the output read.
		out, _ := exec.Command(path, "--help").CombinedOutput()

		names := map[string]bool{}
		for _, m := range thriftGenLine.FindAllStringSubmatch(string(out), -1) {
			names[m[1]] = true
		}
		if len(names) > 0 {
			thriftGenCache.names = names
		}
	})
	return thriftGenCache.names
}

// ThriftGeneratorNames returns the installed generators sorted, for a message
// that has to list them.
func ThriftGeneratorNames() []string {
	have := ThriftGenerators()
	out := make([]string, 0, len(have))
	for name := range have {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
