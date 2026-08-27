package names

import "testing"

// names_test.go covers the casing conversions every target's identifiers go
// through.
//
// They are worth testing directly rather than through a golden file: a rendered
// `displayName` looks equally correct whether it was derived or hardcoded, so a
// golden test agrees with whatever the code currently does. These assert the rule.
func TestPascal(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"point_cloud", "PointCloud"},
		{"sensor", "Sensor"},
		{"a_b_c", "ABC"},

		// Already-PascalCase input, and an acronym, both pass through. Folding
		// `IMU` to `Imu` would be a rename rather than a normalization.
		{"PointCloud", "PointCloud"},
		{"IMU", "IMU"},

		{"a__b", "AB"},
		{"_leading", "Leading"},
		{"trailing_", "Trailing"},
		{"", ""},
	} {
		if got := Pascal(tc.in); got != tc.want {
			t.Errorf("Pascal(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCamel(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"display_name", "displayName"},
		{"name", "name"},
		{"rate_hz", "rateHz"},
		{"displayName", "displayName"},

		// The case that motivated tracking the first *emitted* segment rather
		// than the first index. Keying on the index treats `value` as a later
		// word and capitalizes it, and an uppercase initial is a Cap'n Proto
		// parse error rather than a cosmetic difference.
		{"_value", "value"},
		{"__value", "value"},

		{"a__b", "aB"},
		{"", ""},
	} {
		if got := Camel(tc.in); got != tc.want {
			t.Errorf("Camel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestScreamingSnake(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"SensorKind", "SENSOR_KIND"},
		{"HealthState", "HEALTH_STATE"},
		{"Color", "COLOR"},

		// An acronym is one word. Splitting on every capital gives `I_M_U`, which
		// matches no AIP-126 value prefix any proto declares — so the prefix is
		// never stripped and every value of the enum keeps it.
		{"IMU", "IMU"},
		{"GPS", "GPS"},

		// An acronym followed by a word: the boundary is the last capital of the
		// run, not every capital in it.
		{"IMUReading", "IMU_READING"},
		{"HTTPStatus", "HTTP_STATUS"},
		{"GPSFixQuality", "GPS_FIX_QUALITY"},

		{"", ""},
	} {
		if got := ScreamingSnake(tc.in); got != tc.want {
			t.Errorf("ScreamingSnake(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestScreamingSnakeFeedsEnumPrefixStripping is the property the conversion
// exists for: an AIP-126 enum value is named <ENUM>_<VALUE>, and recovering
// <ENUM> from the PascalCase type name is what lets the prefix be removed.
func TestScreamingSnakeFeedsEnumPrefixStripping(t *testing.T) {
	for _, tc := range []struct{ enum, value string }{
		{"SensorKind", "SENSOR_KIND_LIDAR"},
		{"HealthState", "HEALTH_STATE_DEGRADED"},
		{"IMU", "IMU_UNSPECIFIED"},
		{"GPSFixQuality", "GPS_FIX_QUALITY_RTK"},
	} {
		prefix := ScreamingSnake(tc.enum) + "_"
		if len(tc.value) <= len(prefix) || tc.value[:len(prefix)] != prefix {
			t.Errorf("ScreamingSnake(%q) = %q, which does not prefix its own value %q",
				tc.enum, ScreamingSnake(tc.enum), tc.value)
		}
	}
}
