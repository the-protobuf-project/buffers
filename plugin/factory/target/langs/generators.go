package langs

// generators.go maps a language to the flag or plugin that produces it, and
// reports what a missing one costs.
//
// The two toolchains differ in a way worth stating. flatc has every backend
// built in, so a language it lists is a language it can produce. Cap'n Proto
// ships only its C++ generator; every other language is a separate capnpc-<lang>
// binary the user installs, so appearing here is a claim that the *schema* suits
// that generator, not that the generator is present.

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// flatcFlags maps a language to flatc's generator flag.
var flatcFlags = map[string]string{
	"cpp":    "--cpp",
	"c++":    "--cpp",
	"csharp": "--csharp",
	"dart":   "--dart",
	"go":     "--go",
	"java":   "--java",
	"kotlin": "--kotlin",
	// Kotlin Multiplatform, for a shared module targeting JVM and native at once.
	"kotlin-kmp": "--kotlin-kmp",
	"lobster":    "--lobster",
	"lua":        "--lua",
	"nim":        "--nim",
	"php":        "--php",
	"python":     "--python",
	"rust":       "--rust",
	"swift":      "--swift",
	"ts":         "--ts",
	"typescript": "--ts",
}

// capnpGenerators maps a language to the capnpc plugin that produces it.
//
// Only c++ ships with Cap'n Proto. Every other entry is a separate executable
// that capnp looks up on PATH as capnpc-<name>, which is why a missing one is
// reported as a missing generator with an install line rather than as a capnp
// failure.
var capnpGenerators = map[string]string{
	"c++":        "c++",
	"cpp":        "c++",
	"go":         "go",
	"rust":       "rust",
	"java":       "java",
	"kotlin":     "java", // capnproto-java output is used from Kotlin via JVM interop
	"ts":         "ts",
	"typescript": "ts", // flatcFlags accepts both spellings; so does this
	"csharp":     "csharp",
}

// capnpSpellingAliases are keys of capnpGenerators that name a generator another
// key already names. They are skipped when the set is *listed* — for `buffers
// doctor`, say — because one binary shown twice reads as two things to install.
var capnpSpellingAliases = map[string]bool{
	"cpp":        true, // same generator as c++
	"typescript": true, // same generator as ts
}

// capnpPlugins gives the install line for each generator that does not ship with
// Cap'n Proto, keyed by the capnpc-<name> binary capnp will look for.
var capnpPlugins = map[string]string{
	"capnpc-go":     "go install capnproto.org/go/capnp/v3/capnpc-go@latest",
	"capnpc-rust":   "cargo install capnpc",
	"capnpc-java":   "see https://github.com/capnproto/capnproto-java",
	"capnpc-ts":     "npm install -g capnpc-ts",
	"capnpc-csharp": "see https://github.com/c80k/capnproto-dotnet",
}

// checkGenerator reports a missing capnp language plugin before capnp is run.
//
// capnp resolves a generator by looking for capnpc-<name> on PATH, and its own
// message when that fails is a bare exec error naming a binary the caller has
// probably never heard of. Catching it here turns that into the install line.
func checkGenerator(r Request) error {
	if r.Target == "thrift" {
		return checkThriftGenerator(r)
	}
	if r.Target != "capnp" {
		return nil
	}
	gen, ok := capnpGenerators[r.Language]
	if !ok || gen == "c++" {
		// c++ is built into capnp itself; there is no separate binary to find.
		return nil
	}

	binary := "capnpc-" + gen
	if _, err := exec.LookPath(binary); err == nil {
		return nil
	}

	install, known := capnpPlugins[binary]
	if !known {
		install = "no install line is recorded for this generator"
	}
	return fmt.Errorf("%s is not on PATH, and capnp needs it to produce %s.\n"+
		"    Cap'n Proto ships only the C++ generator; every other language is a separate plugin.\n"+
		"    install: %s", binary, r.Language, install)
}

// Available reports whether a target has a compiler this package can drive, and
// whether it is installed.
func Available(target string) (tool string, driveable, installed bool) {
	t, ok := tools[target]
	if !ok {
		return "", false, false
	}
	_, err := exec.LookPath(t.Binary)
	return t.Binary, true, err == nil
}

// CapnpLanguages lists the languages capnp can generate, in a stable order.
func CapnpLanguages() []string {
	out := make([]string, 0, len(capnpGenerators))
	for lang := range capnpGenerators {
		if capnpSpellingAliases[lang] {
			continue
		}
		out = append(out, lang)
	}
	sort.Strings(out)
	return out
}

// CapnpGenerator reports the binary that produces a language, the line to install
// it, and whether it is already on PATH.
//
// c++ reports as installed whenever capnp itself is, because it is built in
// rather than being a separate plugin.
func CapnpGenerator(lang string) (binary, install string, installed bool) {
	gen, ok := capnpGenerators[lang]
	if !ok {
		return "", "", false
	}
	if gen == "c++" {
		_, err := exec.LookPath("capnp")
		return "built into capnp", "", err == nil
	}
	binary = "capnpc-" + gen
	_, err := exec.LookPath(binary)
	return binary, capnpPlugins[binary], err == nil
}

// keys renders a map's keys sorted, for an error listing what is supported.
func keys(m map[string]string) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// checkThriftGenerator reports a generator this thrift does not have.
//
// Thrift's own message for it is "Unable to get a generator for \"swift\"",
// which is accurate and leaves the reader to discover the alternatives by
// running --help. Since the set genuinely differs between releases — the
// language they asked for may exist in a thrift one version away — the useful
// message names the version in hand and lists what it does offer.
func checkThriftGenerator(r Request) error {
	have := ThriftGenerators()
	if len(have) == 0 || have[r.Language] {
		// Nil means the probe could not tell; let thrift answer for itself
		// rather than refuse a run that would have worked.
		return nil
	}
	return fmt.Errorf("this thrift has no %q generator.\n"+
		"    Thrift's generator set changes between releases — 0.24 dropped swift and added mmd, "+
		"for one — so a language missing here may exist in another version.\n"+
		"    this build offers: %s",
		r.Language, strings.Join(ThriftGeneratorNames(), ", "))
}
