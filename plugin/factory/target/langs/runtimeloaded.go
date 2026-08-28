package langs

// runtimeloaded.go covers the languages where the emitted schema is already the
// deliverable and there is nothing to compile.
//
// Cap'n Proto's Python support is the case this exists for, and it is not a gap.
// pycapnp does not generate code — it wraps the C++ library and parses the schema
// when the program starts:
//
//	import capnp
//	sensors = capnp.load("sensors/v1/sensors.capnp")
//
// so the .capnp `buffers` already wrote is the artifact, and the Python side
// needs no build step at all. capnproto.org describes this as the recommended
// shape for a dynamic language.
//
// Without this the behaviour was worse than unsupported, it was misleading:
// `capnp compile -o python` looks for a `capnpc-python` plugin, and the install
// line offered for the missing plugin was `pip install pycapnp` — which installs
// `capnpc-cython`, an internal tool for building pycapnp itself, and never the
// binary capnp was looking for. Following the instruction did not fix the error.

import (
	"fmt"
	"sort"
	"strings"
)

// runtimeLoaded names the target and language pairs whose "generation" is the
// consumer loading the schema at run time, mapped to the explanation to print.
var runtimeLoaded = map[string]string{
	"capnp/python": "pycapnp loads the .capnp at run time rather than generating code, so the " +
		"emitted schema is the whole deliverable: `capnp.load(\"<path>.capnp\")`",
}

// RuntimeLoaded reports whether a language consumes the emitted schema directly,
// and the note explaining how.
func RuntimeLoaded(target, lang string) (string, bool) {
	note, ok := runtimeLoaded[key(target, lang)]
	return note, ok
}

// RuntimeLoadedFor lists a target's runtime-loaded languages, sorted.
//
// It exists so a listing can say the language is reachable rather than leave it
// out. Dropping it would be the more misleading answer of the two: absent from a
// generator table reads as unsupported, when in fact it needs less than the
// others, not more.
func RuntimeLoadedFor(target string) []string {
	var out []string
	for k := range runtimeLoaded {
		t, lang, found := strings.Cut(k, "/")
		if found && t == target {
			out = append(out, lang)
		}
	}
	sort.Strings(out)
	return out
}

// key builds the lookup key for a target and language pair.
func key(target, lang string) string { return fmt.Sprintf("%s/%s", target, lang) }
