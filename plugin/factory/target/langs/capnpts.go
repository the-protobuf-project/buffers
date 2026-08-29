package langs

// capnpts.go records where Cap'n Proto's TypeScript generators stop working, so
// the run can say so instead of leaving it to `tsc`.
//
// Both generators that exist convert a Cap'n Proto import into a TypeScript one
// by taking the schema's own import path and making it relative — dropping the
// leading slash and prefixing "./". That path is relative to the *schema root*,
// while the file it is written into sits wherever capnp put it, which is the
// mirrored schema tree. The two agree only at the root.
//
// So sensors/v1/sensors.capnp, importing "/buffers/wellknown.capnp", generates
// sensors/v1/sensors.capnp.d.ts containing:
//
//	import { Timestamp } from "./buffers/wellknown.capnp.js";
//
// which resolves against sensors/v1/buffers/wellknown.capnp.js and finds nothing:
//
//	sensors/v1/sensors.capnp.d.ts(6,60): error TS2307:
//	  Cannot find module './buffers/wellknown.capnp.js'
//
// Verified on capnp 1.5.0 against this repository's own emitted schema, with both
// generators: capnpc-ts 0.9.3 (jdiaz5513/capnp-ts) and capnp-es 0.0.16, which is
// the maintained fork. capnp-es writes the same path with a doubled separator
// (".//buffers/wellknown.js") and fails identically. A single-directory schema is
// unaffected — there the two roots coincide and the import resolves — which is
// why this is a caveat rather than a missing generator.
//
// Neither the schema nor this repository can correct it. capnp chooses the output
// path and the generator chooses the import text; nothing in between sees both.

import "strings"

// capnpTSHint is the advice printed with the warning. It is a const rather than
// part of the message so `buffers doctor` and the README can quote one wording.
const capnpTSHint = "keep the schema to one directory, or take TypeScript from the " +
	"FlatBuffers target, whose --ts output resolves across a nested tree"

// isTypeScript reports whether a language name is one of TypeScript's spellings.
//
// Both are accepted everywhere a language is named — flatcFlags and
// capnpGenerators each carry the pair — so a check on one spelling alone would
// silently pass the other through.
func isTypeScript(lang string) bool {
	switch strings.ToLower(lang) {
	case "ts", "typescript":
		return true
	}
	return false
}

// CapnpTypeScriptNestedImports reports whether a capnp TypeScript run will emit
// imports that do not resolve, which is the case as soon as any schema file sits
// below the schema root.
//
// The check is per-directory rather than per-import because a Group does not
// carry the file's imports, and the distinction rarely matters: a schema spread
// over directories that never references across them is not one this generator
// produces, since it mirrors the proto package tree.
func CapnpTypeScriptNestedImports(target, lang string, groups []Group) bool {
	if target != "capnp" || !isTypeScript(lang) {
		return false
	}
	for _, g := range groups {
		if g.Dir != "" && len(g.Files) > 0 {
			return true
		}
	}
	return false
}

// CapnpTypeScriptHint explains what breaks and what to do instead.
func CapnpTypeScriptHint() string { return capnpTSHint }
