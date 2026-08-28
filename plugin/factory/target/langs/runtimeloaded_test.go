package langs

import (
	"strings"
	"testing"
)

// TestRuntimeLoadedIsNotAGeneratorClaim guards the two halves of the capnp Python
// story staying consistent: it must be reported as needing no generator, and it
// must not also appear as a plugin the user could install.
//
// The bug this replaces was the pair being inconsistent. `python` was mapped to a
// `capnpc-python` plugin that does not exist, and the install line offered when
// it turned up missing was `pip install pycapnp` — which installs
// `capnpc-cython`, an internal tool for building pycapnp, and never the binary
// capnp was looking for. Following the instruction did not fix the error.
func TestRuntimeLoadedIsNotAGeneratorClaim(t *testing.T) {
	note, ok := RuntimeLoaded("capnp", "python")
	if !ok {
		t.Fatal("capnp/python is not reported as runtime-loaded")
	}
	if !strings.Contains(note, "capnp.load") {
		t.Errorf("the note does not say how to consume the schema: %q", note)
	}

	if _, claimed := capnpGenerators["python"]; claimed {
		t.Error("python is still mapped to a capnpc plugin; capnp would look for capnpc-python, " +
			"which pycapnp does not provide")
	}
	for binary := range capnpPlugins {
		if strings.Contains(binary, "python") {
			t.Errorf("%s still has an install line, for a plugin that does not exist", binary)
		}
	}
}

// TestRuntimeLoadedForListsTheLanguage checks the listing helper, which is what
// keeps a runtime-loaded language from silently vanishing out of `buffers doctor`
// once it is no longer in the generator table.
func TestRuntimeLoadedForListsTheLanguage(t *testing.T) {
	got := RuntimeLoadedFor("capnp")
	if len(got) != 1 || got[0] != "python" {
		t.Errorf("RuntimeLoadedFor(capnp) = %v, want [python]", got)
	}
	if got := RuntimeLoadedFor("flatbuffers"); len(got) != 0 {
		t.Errorf("RuntimeLoadedFor(flatbuffers) = %v, want none", got)
	}
}
