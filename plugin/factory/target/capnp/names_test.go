package capnp

import "testing"

// names_test.go covers the identifier rewriting every emitted field name goes
// through.
//
// It is worth testing in isolation for two reasons. Cap'n Proto's grammar makes
// these rules a parse requirement rather than a style choice — a member name with
// an uppercase initial does not compile — so a mistake here is not cosmetic. And
// the rewriting is invisible in a golden file: `displayName` looks equally correct
// whether it was derived or hardcoded, so a golden test agrees with whatever the
// code currently does.
func TestMember(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"name", "name"},
		{"display_name", "displayName"},
		{"observe_time", "observeTime"},
		{"rate_hz", "rateHz"},
		{"a", "a"},

		// Already camelCase passes through with its interior capitals intact.
		{"displayName", "displayName"},

		// Consecutive and trailing underscores are legal in proto and produce no
		// empty segments here.
		{"a__b", "aB"},
		{"trailing_", "trailing"},

		// A name colliding with capnp's grammar is suffixed rather than emitted.
		{"union", "union_"},
		{"group", "group_"},
		{"interface", "interface_"},

		// Degenerate input still yields a legal identifier.
		{"", "field"},
		{"_", "field"},
	} {
		if got := member(tc.in); got != tc.want {
			t.Errorf("member(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTypeName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Sensor", "Sensor"},
		{"PointCloud", "PointCloud"},
		{"point_cloud", "PointCloud"},

		// An acronym keeps its shape; lowercasing it would be a rename.
		{"IMU", "IMU"},

		// A type shadowing a builtin would resolve to the builtin at every use
		// site, so it is suffixed too.
		{"List", "List_"},
		{"Text", "Text_"},

		{"", "Type"},
	} {
		if got := typeName(tc.in); got != tc.want {
			t.Errorf("typeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEnumerant(t *testing.T) {
	for _, tc := range []struct{ enum, value, want string }{
		// The ordinary AIP-126 case: the value repeats the enum name, and
		// carrying that through would read as SensorKind.sensorKindLidar.
		{"SensorKind", "SENSOR_KIND_UNSPECIFIED", "unspecified"},
		{"SensorKind", "SENSOR_KIND_LIDAR", "lidar"},
		{"SensorKind", "SENSOR_KIND_WHEEL_ENCODER", "wheelEncoder"},
		{"HealthState", "HEALTH_STATE_DEGRADED", "degraded"},

		// An acronym enum. The prefix has to be recognized for the value to read
		// as `unspecified` rather than `imuUnspecified`.
		{"IMU", "IMU_UNSPECIFIED", "unspecified"},
		{"IMUReading", "IMU_READING_RAW", "raw"},

		// No prefix to strip: the value stands alone.
		{"Color", "RED", "red"},

		// Stripping would leave an identifier starting with a digit, which capnp
		// rejects — so the prefix stays.
		{"Kind", "KIND_2D", "kind2d"},

		// Stripping would leave nothing at all.
		{"Kind", "KIND_", "kind"},

		// A stripped value colliding with the grammar is still suffixed.
		{"Mode", "MODE_UNION", "union_"},
	} {
		if got := enumerant(tc.enum, tc.value); got != tc.want {
			t.Errorf("enumerant(%q, %q) = %q, want %q", tc.enum, tc.value, got, tc.want)
		}
	}
}

func TestCxxNamespace(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"sensors.v1", "sensors::v1"},
		{"a.b.c.d", "a::b::c::d"},
		{"flat", "flat"},
	} {
		if got := cxxNamespace(tc.in); got != tc.want {
			t.Errorf("cxxNamespace(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
