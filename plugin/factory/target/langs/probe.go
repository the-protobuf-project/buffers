package langs

// probe.go asks an installed compiler what version it is, for a listing that
// reports what a machine actually has rather than only whether something with the
// right name is on PATH.
//
// Every one of these three prints its version and exits zero for --version, but
// they disagree about the shape of the line — `flatc version 24.3.25`,
// `Cap'n Proto version 1.0.2`, `Thrift version 0.24.0` — so the string is
// reported as the tool gave it rather than parsed. Nothing here compares
// versions; capability.go explains why asking a binary what it accepts is the
// check worth making, and version parsing is the one that quietly goes wrong.

import (
	"os/exec"
	"strings"
	"sync"
)

// versionCache memoizes one probe per binary per process.
var versionCache struct {
	sync.Mutex
	byBinary map[string]string
}

// Version returns the version string a target's compiler reports, or "" when the
// compiler is absent or does not answer.
func Version(target string) string {
	tool, ok := tools[target]
	if !ok {
		return ""
	}

	versionCache.Lock()
	defer versionCache.Unlock()
	if versionCache.byBinary == nil {
		versionCache.byBinary = map[string]string{}
	}
	if got, ok := versionCache.byBinary[tool.Binary]; ok {
		return got
	}

	out, err := exec.Command(tool.Binary, "--version").Output()
	got := ""
	if err == nil {
		// The first line only: some builds print a banner underneath, and a
		// listing has one column for this.
		got = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	}
	versionCache.byBinary[tool.Binary] = got
	return got
}

// Targets lists the targets that have a compiler this package can drive, in a
// stable order.
func Targets() []string {
	out := make([]string, 0, len(tools))
	for name := range tools {
		out = append(out, name)
	}
	// Not sorted alphabetically: this is the order the targets are presented in
	// everywhere else, and a listing that reorders them between commands is
	// harder to read than one that does not.
	order := map[string]int{"flatbuffers": 0, "capnp": 1, "thrift": 2}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if order[out[j]] < order[out[i]] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
