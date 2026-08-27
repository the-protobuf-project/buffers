package registry_test

// toolchain_test.go compiles the emitted schema with the real flatc and capnp.
//
// It is a different claim from the golden tests, and the more important one. A
// golden test asserts the output has not changed; this asserts it is *valid* —
// that flatc accepts every id, attribute and include, and that capnp accepts
// every ordinal, identifier and type ID. A schema can be byte-stable and still
// not compile, and without this the first person to find out would be a user.
//
// Each skips when its toolchain is absent, because a machine without flatc is a
// normal machine and this repository's correctness does not depend on what
// happens to be installed on it.

import (
	"os/exec"
	"testing"
)

func TestFlatcAcceptsTheSchema(t *testing.T) {
	flatc := requireTool(t, "flatc")
	dir := writeTree(t, render(t, "flatbuffers"))

	for _, file := range schemaFiles(t, dir, ".fbs") {
		t.Run(file, func(t *testing.T) {
			// --cpp is the least demanding backend and the one flatc always
			// ships; the language does not matter, only that the schema parses
			// and resolves.
			cmd := exec.Command(flatc, "--cpp", "-o", t.TempDir(), "-I", dir, file)
			cmd.Dir = dir

			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("flatc rejected the generated schema: %v\n%s", err, out)
			}
		})
	}
}

func TestCapnpAcceptsTheSchema(t *testing.T) {
	capnp := requireTool(t, "capnp")
	requireCapnpSchemas(t)
	dir := writeTree(t, render(t, "capnp"))

	for _, file := range schemaFiles(t, dir, ".capnp") {
		t.Run(file, func(t *testing.T) {
			// -o- parses, links and writes the compiled schema to stdout without
			// invoking a language generator, so this needs no capnpc-* plugin
			// installed.
			cmd := exec.Command(capnp, "compile", "-I", dir, "-o-", file)
			cmd.Dir = dir

			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("capnp rejected the generated schema: %v\n%s", err, firstLines(out, 12))
			}
		})
	}
}
