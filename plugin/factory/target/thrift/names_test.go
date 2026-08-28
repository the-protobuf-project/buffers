package thrift

import "testing"

// TestIdentLeavesProtoNamesAlone is the property this target is built on: where a
// grammar does not force a rename, the proto name is the mapping.
func TestIdentLeavesProtoNamesAlone(t *testing.T) {
	for _, name := range []string{"display_name", "SENSOR_KIND_LIDAR", "GetSensor", "rate_hz", "_value"} {
		if got := ident(name); got != name {
			t.Errorf("ident(%q) = %q, want it unchanged", name, got)
		}
	}
}

// TestIdentSuffixesReservedWords covers the words the Thrift compiler refuses,
// including the host-language ones that are not Thrift keywords at all — a field
// named `from` is legal proto and breaks thrift's Python backend, so thrift
// rejects it up front.
func TestIdentSuffixesReservedWords(t *testing.T) {
	cases := map[string]string{
		"union":  "union_",
		"list":   "list_",
		"binary": "binary_",
		"from":   "from_",
		"class":  "class_",
		"Struct": "Struct_", // the check is case-insensitive; thrift's is too
		"":       "field",
	}
	for in, want := range cases {
		if got := ident(in); got != want {
			t.Errorf("ident(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTypeNameFlattensNesting checks that a nested proto message folds its path
// into one name, since Thrift has a single flat scope per file.
func TestTypeNameFlattensNesting(t *testing.T) {
	cases := []struct {
		full, pkg, want string
	}{
		{"sensors.v1.Sensor", "sensors.v1", "Sensor"},
		{"sensors.v1.Sensor.Calibration", "sensors.v1", "SensorCalibration"},
		{"sensors.v1.A.B.C", "sensors.v1", "ABC"},
		{"sensors.v1.point_cloud", "sensors.v1", "PointCloud"},
	}
	for _, c := range cases {
		if got := typeName(c.full, c.pkg); got != c.want {
			t.Errorf("typeName(%q, %q) = %q, want %q", c.full, c.pkg, got, c.want)
		}
	}
}

// TestUnionNameAgreesWithFlatBuffersAtTopLevel checks the naming this target
// derives matches protokit's FlatBuffers default for a top-level message, so one
// oneof does not end up with two names across two targets.
func TestUnionNameAgreesWithFlatBuffersAtTopLevel(t *testing.T) {
	if got := unionName("Reading", "payload"); got != "ReadingPayload" {
		t.Errorf("unionName = %q, want ReadingPayload", got)
	}
	// A nested owner is where the two deliberately part: protokit builds the
	// FlatBuffers name from the message's short name, which would collide here.
	if got := unionName("SensorCalibration", "source"); got != "SensorCalibrationSource" {
		t.Errorf("unionName = %q, want SensorCalibrationSource", got)
	}
}

// TestNormalizeLangResolvesAliases checks that the spellings the other targets
// use reach thrift's own generator names.
func TestNormalizeLangResolvesAliases(t *testing.T) {
	cases := map[string]string{
		"python": "py", "rust": "rs", "c++": "cpp", "csharp": "netstd",
		"go": "go", "java": "java", "unknown": "unknown",
	}
	for in, want := range cases {
		if got := normalizeLang(in); got != want {
			t.Errorf("normalizeLang(%q) = %q, want %q", in, got, want)
		}
	}
}
