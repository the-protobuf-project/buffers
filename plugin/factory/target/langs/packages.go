package langs

// packages.go computes the per-invocation flags that make generated code declare
// the package the proto asked for, and reports the cases where it cannot.
//
// Both toolchains derive a package from the schema rather than from the proto, so
// left alone they emit `package sensors.v1` where protoc would have written
// `package com.sensors.v1`. The flags here correct that where a flag exists —
// and where none does, the caller is told rather than left to discover it.

import (
	"strings"
)

// packageFlags computes the per-group flags that make the generated package match
// what the proto declares.
func packageFlags(target, lang string, g Group) []string {
	if target != "flatbuffers" {
		// capnp derives its packages from the annotations already written into the
		// schema, so there is nothing to pass on the command line.
		return nil
	}

	switch lang {
	case "java":
		if prefix, ok := jvmPrefix(g.JVMPackage, g.Namespace); ok {
			return []string{"--java-package-prefix", prefix}
		}

	case "kotlin", "kotlin-kmp":
		// Deliberately no flag. --java-package-prefix is Java-only: flatc accepts
		// it alongside --kotlin and ignores it, emitting `package sensors.v1`
		// regardless. Passing it anyway would look like the package was being
		// honoured while it was not, which is worse than not passing it — so the
		// caller is warned instead. See KotlinHonoursJVMPackage.

	case "go":
		var flags []string
		if g.GoPackage != "" {
			flags = append(flags, "--go-namespace", g.GoPackage)
		}
		// Only when the installed flatc knows the flag. An older one rejects it
		// outright rather than ignoring it, which turns a degraded result into a
		// failed run. See capability.go.
		if g.GoModule != "" && FlatcSupportsGoModule() {
			flags = append(flags, "--go-module-name", g.GoModule)
		}
		return flags
	}
	return nil
}

// includeFlags keeps a generated file's cross-file references resolvable once the
// output has been spread across directories.
//
// This is the other half of mirroring the source tree, and skipping it produces a
// tree that looks right and does not build. flatc writes bare include statements
// by default — `#include "wellknown_generated.h"` — which resolve only while
// every generated file sits in one directory. Move them into
// buffers/ and sensors/v1/ and the include finds nothing:
//
//	sensors/v1/sensors_generated.h:16:10: fatal error:
//	  'wellknown_generated.h' file not found
//
// --keep-prefix carries the schema's own include path through to the generated
// one, so it becomes `#include "buffers/wellknown_generated.h"` and resolves
// against the output root.
//
// It applies only where output was actually spread out. A generator that nests by
// namespace resolves its own imports by package name and needs nothing here.
func includeFlags(target, lang string) []string {
	if target != "flatbuffers" || NestingOf(lang) != NestFlat {
		return nil
	}
	return []string{"--keep-prefix"}
}

// jvmPrefix recovers the prefix that turns the schema namespace into the proto's
// java_package.
//
// flatc has no "set the package" flag — only --java-package-prefix, which is
// prepended to the namespace it derived. So `namespace sensors.v1` with
// java_package `com.sensors.v1` needs the prefix `com`.
//
// When java_package does not end with the namespace the two cannot be reconciled
// this way — java_package `com.example.api` over namespace `sensors.v1` has no
// prefix P where P + ".sensors.v1" is the target — and the caller is told rather
// than being handed a package that is subtly not the one the proto asked for.
func jvmPrefix(jvmPackage, namespace string) (string, bool) {
	if jvmPackage == "" || namespace == "" || jvmPackage == namespace {
		return "", false
	}
	suffix := "." + namespace
	if !strings.HasSuffix(jvmPackage, suffix) {
		return "", false
	}
	prefix := strings.TrimSuffix(jvmPackage, suffix)
	if prefix == "" {
		return "", false
	}
	return prefix, true
}

// JVMPackageReconcilable reports whether a java_package can be expressed as a
// flatc package prefix over the given namespace, so a caller can warn once rather
// than silently emitting the wrong package.
func JVMPackageReconcilable(jvmPackage, namespace string) bool {
	if jvmPackage == "" || jvmPackage == namespace {
		return true // nothing to reconcile
	}
	_, ok := jvmPrefix(jvmPackage, namespace)
	return ok
}

// KotlinHonoursJVMPackage reports whether flatc will emit the proto's
// java_package for Kotlin output.
//
// It will not, unless java_package already equals the namespace: flatc has no
// Kotlin package option, and the Java one does not apply. This exists so the
// caller can say so once rather than leaving someone to discover that their
// Kotlin sources declare a different package from their Java ones.
func KotlinHonoursJVMPackage(jvmPackage, namespace string) bool {
	return jvmPackage == "" || jvmPackage == namespace
}

// Why --gen-onefile is not used, though it would give Go idiomatic file names.
//
// flatc names one file per generated type, so a Go package comes out as
// CreateSensorRequest.go, Sensor.go, and so on. Go convention — and what
// protoc-gen-go does — is one lowercase file per source schema: sensors.pb.go.
// flatc has --gen-onefile, which produces exactly that: sensors_generated.go.
//
// It emits Go that does not compile. The generated file carries a fixed import
// block, `flatbuffers` and `strconv`, whatever the schema actually contains:
//
//	enums_generated.go:6:2:
//	  "github.com/google/flatbuffers/go" imported as flatbuffers and not used
//	sensors_service_generated.go:7:2: "strconv" imported and not used
//
// A schema with no tables never touches flatbuffers; one with no enums never
// touches strconv. Go treats an unused import as an error rather than a warning,
// so any schema that is not both breaks — which is nearly all of them. The worse
// file names with code that builds is the right way round.
//
// If flatc gains per-file imports, switching is a one-line change here plus
// placing Go output by namespace path rather than source path, since --gen-onefile
// derives its import paths from the namespace.
