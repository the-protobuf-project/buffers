// Package names converts proto identifiers into the casings the target formats
// use.
//
// It exists because the same three conversions were written four times — once per
// target — and the copies had drifted. Two of them carried the same bug in
// SCREAMING_SNAKE, and one of the camelCase copies could return an identifier
// with an uppercase initial, which Cap'n Proto rejects outright. A shared,
// tested implementation is the difference between fixing that once and fixing it
// wherever someone happens to notice.
//
// What stays out: anything a specific format decides. Cap'n Proto's reserved
// words, FlatBuffers' map-entry naming, ROS's field-name sanitizing — those are
// each one target's rules and live with that target.
package names

import "strings"

// Pascal upper-camel-cases a snake_case proto name: `point_cloud` -> `PointCloud`.
//
// Interior capitalization survives, so a name that is already PascalCase passes
// through unchanged and an acronym keeps its shape. Lowercasing `IMU` to `Imu`
// would be a rename, not a normalization.
func Pascal(s string) string { return convert(s, true) }

// Camel lower-camel-cases a snake_case proto name: `display_name` ->
// `displayName`.
//
// The lowercase initial is a Cap'n Proto grammar requirement rather than a
// preference — a member name starting with a capital is a parse error — which is
// why the leading-underscore case below is handled rather than left to chance.
func Camel(s string) string { return convert(s, false) }

// convert joins underscore-separated words, capitalizing each one's initial and
// deciding the first from upperFirst.
//
// "First" means the first *emitted* segment, not the first element of the split.
// An input like `_value` splits to ["", "value"], and keying on the index would
// treat `value` as a later word and capitalize it — turning a lowercase-initial
// request into `Value`. Legal in most targets, a parse error in Cap'n Proto, and
// a silent rename everywhere else.
func convert(s string, upperFirst bool) string {
	var b strings.Builder
	first := true

	for _, part := range strings.Split(s, "_") {
		if part == "" {
			continue
		}
		if first && !upperFirst {
			b.WriteString(strings.ToLower(part[:1]))
		} else {
			b.WriteString(strings.ToUpper(part[:1]))
		}
		b.WriteString(part[1:])
		first = false
	}
	return b.String()
}

// ScreamingSnake converts a PascalCase name to SCREAMING_SNAKE, which is how
// AIP-126 spells an enum's value prefix and how Kotlin spells a constant.
//
// A word boundary is not simply "an uppercase letter". Splitting on every capital
// turns `IMU` into `I_M_U`, which matches no prefix a real proto declares — so
// `IMU_UNSPECIFIED` keeps its prefix and is emitted as `imuUnspecified` rather
// than `unspecified`, for every value of every acronym-named enum. In a sensor or
// vehicle schema that is most of them: IMU, GPS, CAN, RTK, LIDAR.
//
// The boundary is therefore either end of a run of capitals:
//
//	SensorKind  -> SENSOR_KIND   (lower, then upper)
//	IMUReading  -> IMU_READING   (upper, then upper followed by lower)
//	HTTPStatus  -> HTTP_STATUS
//	IMU         -> IMU           (no boundary at all)
//
// protokit's naming.ScreamingSnake is deliberately not used here: it splits on
// lower-to-upper only, so it gets `IMU` right and renders `IMUReading` as
// `IMUREADING`.
func ScreamingSnake(s string) string {
	runes := []rune(s)
	var b strings.Builder

	for i, r := range runes {
		if i > 0 && isUpper(r) {
			prev := runes[i-1]
			// A capital following a lowercase letter or a digit always starts a
			// word. A capital inside a run of capitals starts one only when the
			// next character is lowercase, which makes it the first letter of the
			// following word rather than part of the acronym.
			startsWord := !isUpper(prev)
			if isUpper(prev) && i+1 < len(runes) && isLower(runes[i+1]) {
				startsWord = true
			}
			if startsWord {
				b.WriteByte('_')
			}
		}
		b.WriteRune(r)
	}
	return strings.ToUpper(b.String())
}

// isUpper reports whether a rune is an ASCII capital.
func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }

// isLower reports whether a rune is an ASCII lowercase letter.
func isLower(r rune) bool { return r >= 'a' && r <= 'z' }
