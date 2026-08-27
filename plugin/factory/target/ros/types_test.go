package ros

import (
	"testing"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
)

// types_test.go covers the ROS field-name sanitizer and the bounds that become
// part of a ROS type.
//
// fieldName is the interesting one: ROS's rules are stricter than proto's, so a
// legal proto name can be an illegal ROS one. The failure is not a build error
// here — it is a .msg that rosidl rejects in the consumer's colcon build, a long
// way from anything this repository runs.
func TestFieldName(t *testing.T) {
	for _, tc := range []struct{ in, want, why string }{
		// Proto names are already snake_case, so the common case is identity.
		{"name", "name", "plain"},
		{"display_name", "display_name", "snake_case passes through"},
		{"rate_hz", "rate_hz", "digits are legal"},

		// ROS requires lowercase.
		{"DisplayName", "displayname", "uppercase is folded"},

		// ROS allows single underscores between words only: not doubled, not
		// leading, not trailing. All three are legal proto.
		{"a__b", "a_b", "doubled underscores collapse"},
		{"_leading", "leading", "a leading underscore is dropped"},
		{"trailing_", "trailing", "a trailing underscore is dropped"},
		{"__both__", "both", "both ends"},

		// A name that would start with a digit is not a legal identifier.
		{"2d_pose", "field_2d_pose", "a leading digit is prefixed"},

		// Degenerate input still yields something legal rather than an empty
		// field name, which would produce a .msg line of just a type.
		// The fallback must itself be legal: "field_" would end in an underscore,
		// which is the very thing being sanitized away.
		{"_", "field", "only underscores"},
		{"", "field", "empty"},
		{"!!!", "field", "nothing survives sanitizing"},
	} {
		if got := fieldName(tc.in); got != tc.want {
			t.Errorf("fieldName(%q) = %q, want %q (%s)", tc.in, got, tc.want, tc.why)
		}
	}
}

func TestFieldNameOutputIsAlwaysLegal(t *testing.T) {
	// Whatever comes in, what comes out must satisfy ROS's rules — the sanitizer
	// exists for the inputs nobody thought of.
	for _, in := range []string{
		"", "_", "__", "123", "a.b", "a-b", "a b", "ALLCAPS", "_x_", "x__y__z",
		"9", "_9a", "a!b@c",
	} {
		got := fieldName(in)
		if got == "" {
			t.Errorf("fieldName(%q) is empty", in)
			continue
		}
		if got[0] >= '0' && got[0] <= '9' {
			t.Errorf("fieldName(%q) = %q starts with a digit", in, got)
		}
		if got[0] == '_' || got[len(got)-1] == '_' {
			t.Errorf("fieldName(%q) = %q begins or ends with an underscore", in, got)
		}
		for i := range len(got) {
			c := got[i]
			ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
			if !ok {
				t.Errorf("fieldName(%q) = %q contains %q", in, got, string(c))
				break
			}
			if c == '_' && i+1 < len(got) && got[i+1] == '_' {
				t.Errorf("fieldName(%q) = %q has doubled underscores", in, got)
				break
			}
		}
	}
}

func TestFlatten(t *testing.T) {
	// ROS has no nested types: one message per file, named after the file. A
	// nested proto message is flattened by joining the enclosing names, which is
	// what keeps Outer.Inner and a separate OuterInner from colliding on one file.
	for _, tc := range []struct{ full, pkg, want string }{
		{"sensors.v1.Sensor", "sensors.v1", "Sensor"},
		{"sensors.v1.Sensor.Reading", "sensors.v1", "SensorReading"},
		{"sensors.v1.A.B.C", "sensors.v1", "ABC"},
		{"sensors.v1.point_cloud", "sensors.v1", "PointCloud"},
	} {
		if got := flatten(tc.full, tc.pkg); got != tc.want {
			t.Errorf("flatten(%q, %q) = %q, want %q", tc.full, tc.pkg, got, tc.want)
		}
	}
}

func TestQualifyOmitsTheOwnPackage(t *testing.T) {
	// ROS names a type as package/Type, and omits the package within it. Always
	// qualifying is legal and reads badly; never qualifying does not compile
	// across packages.
	r := &run{}
	file := &bufir.File{ROSPackage: "sensors_msgs"}

	if got := r.qualify(rosName{Package: "sensors_msgs", Type: "Pose"}, file); got != "Pose" {
		t.Errorf("same-package reference = %q, want the bare type", got)
	}
	if got := r.qualify(rosName{Package: "geometry_msgs", Type: "Pose"}, file); got != "geometry_msgs/Pose" {
		t.Errorf("cross-package reference = %q, want it qualified", got)
	}
	if got := r.qualify(rosName{Type: "Pose"}, file); got != "Pose" {
		t.Errorf("unqualified name = %q, want the bare type", got)
	}
}
