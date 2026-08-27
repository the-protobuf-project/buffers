package langs

// gofiles_test.go covers the PascalCase-to-snake_case rename applied to flatc's
// Go output, and the build-constraint guard that makes it safe.
//
// The guard is the case that matters. A naive rename is not merely untidy — it
// silently removes types from the build, and the error surfaces in the consumer's
// project as "undefined: SensorTest", nowhere near the generator that caused it.

import (
	"reflect"
	"testing"
)

func TestRenameLowercasesAndSnakes(t *testing.T) {
	dir := seed(t, map[string]string{
		"Sensor.go":                "package p\n\ntype Sensor struct{}\n",
		"CreateSensorRequest.go":   "package p\n\ntype CreateSensorRequest struct{}\n",
		"ImageFrame.go":            "package p\n\ntype ImageFrame struct{}\n",
		"sensors/v1/PointCloud.go": "package v1\n\ntype PointCloud struct{}\n",
	})

	n, err := RenameGoFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("renamed %d files, want 4", n)
	}

	want := []string{
		"create_sensor_request.go",
		"image_frame.go",
		"sensor.go",
		"sensors/v1/point_cloud.go",
	}
	if got := tree(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("tree = %v, want %v", got, want)
	}
}

func TestBuildConstrainedNamesAreGuarded(t *testing.T) {
	// The hazard. Each of these snakes to a name the Go toolchain reads as a
	// build constraint, which drops the type from the package with no error until
	// something references it.
	dir := seed(t, map[string]string{
		"SensorTest.go":    "package p\n\ntype SensorTest struct{}\n",
		"SensorLinux.go":   "package p\n\ntype SensorLinux struct{}\n",
		"SensorWindows.go": "package p\n\ntype SensorWindows struct{}\n",
		"SensorAmd64.go":   "package p\n\ntype SensorAmd64 struct{}\n",
	})

	if _, err := RenameGoFiles(dir); err != nil {
		t.Fatal(err)
	}

	for _, name := range tree(t, dir) {
		base := name[:len(name)-len(".go")]
		for _, bad := range []string{"_test", "_linux", "_windows", "_amd64"} {
			if len(base) >= len(bad) && base[len(base)-len(bad):] == bad {
				t.Errorf("%s ends in %q, which Go reads as a build constraint — the type is silently excluded",
					name, bad)
			}
		}
	}
}

func TestGuardedNamesStillCompileAsOnePackage(t *testing.T) {
	// The guard is only worth anything if the result is still one buildable
	// package: a rename that avoided the constraint but broke the build would be
	// no better.
	goBin := requireGo(t)

	dir := seed(t, map[string]string{
		"go.mod":         "module probe\n\ngo 1.26\n",
		"SensorTest.go":  "package probe\n\ntype SensorTest struct{ X int }\n",
		"SensorLinux.go": "package probe\n\ntype SensorLinux struct{ X int }\n",
		"use.go":         "package probe\n\nvar _ = SensorTest{}\nvar _ = SensorLinux{}\n",
	})

	if _, err := RenameGoFiles(dir); err != nil {
		t.Fatal(err)
	}

	build := execCommand(goBin, "build", "./...")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Errorf("the renamed package does not build: %v\n%s", err, out)
	}
}

func TestOnlyGoOutputIsRenamed(t *testing.T) {
	// Java is the counter-case and the reason this is not applied everywhere:
	// Sensor.java must contain class Sensor, so renaming it breaks the build.
	if GoFilesNeedRenaming("flatbuffers", "java") {
		t.Error("Java output would be renamed; a Java file name is part of the language")
	}
	if GoFilesNeedRenaming("capnp", "go") {
		t.Error("capnpc-go already names files after the schema; renaming would be churn")
	}
	if !GoFilesNeedRenaming("flatbuffers", "go") {
		t.Error("flatc Go output is the case this exists for")
	}
}
