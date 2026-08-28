package registry_test

// toolchain_test.go compiles the emitted schema with the real flatc, capnp and
// thrift.
//
// It is a different claim from the golden tests, and the more important one. A
// golden test asserts the output has not changed; this asserts it is *valid* —
// that flatc accepts every id, attribute and include, and that capnp accepts
// every ordinal, identifier and type ID. A schema can be byte-stable and still
// not compile, and without this the first person to find out would be a user.
//
// The Thrift check earns its place twice over, because that target has a
// constraint the others do not: Thrift resolves a type name where it is used, so
// a struct declared before something it names fails here and nowhere else. No
// golden comparison can catch that; only the compiler can.
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

func TestThriftAcceptsTheSchema(t *testing.T) {
	thrift := requireTool(t, "thrift")
	dir := writeTree(t, render(t, "thrift"))

	for _, file := range schemaFiles(t, dir, ".thrift") {
		t.Run(file, func(t *testing.T) {
			// --gen cpp for the same reason flatc gets --cpp above: it is the
			// backend every build of the compiler ships, and the language does
			// not matter here — only that the schema parses, that every include
			// resolves, and that nothing is used before it is declared.
			//
			// One file per invocation, unlike the other two: the Thrift compiler
			// takes exactly one input. See langs/tools.go.
			cmd := exec.Command(thrift, "--gen", "cpp", "-out", t.TempDir(), "-I", dir, file)
			cmd.Dir = dir

			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("thrift rejected the generated schema: %v\n%s", err, firstLines(out, 12))
			}
		})
	}
}
