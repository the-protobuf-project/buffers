// Package wellknown recognises the google.protobuf type protokit does not
// classify: NullValue, the one well-known type that is an enum.
//
// protokit marks a field's WellKnown only for message-typed fields, so a
// NullValue field arrives looking like any other enum, and each target used to
// render it as a reference into google/protobuf/struct — a file no target
// emits. The name is matched here, once, because matching a full name in four
// places is how one of them ends up spelled wrong. What each target writes in
// its place is that target's decision and stays with it.
//
// When protokit classifies enum fields too, this package goes away.
package wellknown

import "github.com/the-protobuf-project/protokit/buffers"

// nullValue is google.protobuf.NullValue's full name.
const nullValue = "google.protobuf.NullValue"

// IsNullValue reports whether f is a google.protobuf.NullValue: a field whose
// only possible value is "null", which carries presence and no payload.
func IsNullValue(f *buffers.Field) bool {
	return f != nil && f.Kind == buffers.KindEnum && f.Enum == nullValue
}
