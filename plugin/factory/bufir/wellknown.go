package bufir

// wellknown.go classifies the google.protobuf.* types that every target has an
// opinion about.
//
// A serialization target cannot pass these through as ordinary messages. Cap'n
// Proto has no Timestamp and no Duration; FlatBuffers has neither and also no
// Any; ROS has its own builtin_interfaces/Time whose field names differ from
// proto's. Each target decides what to do — the timestamp targets mostly
// substitute a native or conventional type — but all of them need to know which
// messages these are, and matching on a full name in four places is how one of
// them ends up spelled wrong.

// WellKnown identifies a google.protobuf.* message.
type WellKnown uint8

const (
	// WKNone means the message is not a google.protobuf type.
	WKNone WellKnown = iota
	// WKAny is google.protobuf.Any: a type URL and an opaque payload. It stays
	// opaque in every target, since resolving it needs a descriptor pool.
	WKAny
	// WKTimestamp is google.protobuf.Timestamp, substituted everywhere.
	WKTimestamp
	// WKDuration is google.protobuf.Duration, substituted everywhere.
	WKDuration
	// WKStruct is google.protobuf.Struct: dynamically typed JSON, which no static
	// schema format here can express.
	WKStruct
	// WKValue is google.protobuf.Value, one dynamically typed JSON value.
	WKValue
	// WKListValue is google.protobuf.ListValue, a dynamically typed JSON array.
	WKListValue
	// WKNullValue is google.protobuf.NullValue, JSON's null as an enum.
	WKNullValue
	// WKFieldMask is google.protobuf.FieldMask: a list of paths, which every
	// target renders as a list of strings.
	WKFieldMask
	// WKEmpty is google.protobuf.Empty, the absence of a payload.
	WKEmpty
	// WKDoubleValue boxes a double to give it explicit presence.
	WKDoubleValue
	// WKFloatValue boxes a float to give it explicit presence.
	WKFloatValue
	// WKInt64Value boxes an int64 to give it explicit presence.
	WKInt64Value
	// WKUint64Value boxes a uint64 to give it explicit presence.
	WKUint64Value
	// WKInt32Value boxes an int32 to give it explicit presence.
	WKInt32Value
	// WKUint32Value boxes a uint32 to give it explicit presence.
	WKUint32Value
	// WKBoolValue boxes a bool to give it explicit presence.
	WKBoolValue
	// WKStringValue boxes a string to give it explicit presence.
	WKStringValue
	// WKBytesValue boxes bytes to give it explicit presence.
	WKBytesValue
)

// wellKnownByName is the lookup. Keys are full proto names.
var wellKnownByName = map[string]WellKnown{
	"google.protobuf.Any":         WKAny,
	"google.protobuf.Timestamp":   WKTimestamp,
	"google.protobuf.Duration":    WKDuration,
	"google.protobuf.Struct":      WKStruct,
	"google.protobuf.Value":       WKValue,
	"google.protobuf.ListValue":   WKListValue,
	"google.protobuf.NullValue":   WKNullValue,
	"google.protobuf.FieldMask":   WKFieldMask,
	"google.protobuf.Empty":       WKEmpty,
	"google.protobuf.DoubleValue": WKDoubleValue,
	"google.protobuf.FloatValue":  WKFloatValue,
	"google.protobuf.Int64Value":  WKInt64Value,
	"google.protobuf.UInt64Value": WKUint64Value,
	"google.protobuf.Int32Value":  WKInt32Value,
	"google.protobuf.UInt32Value": WKUint32Value,
	"google.protobuf.BoolValue":   WKBoolValue,
	"google.protobuf.StringValue": WKStringValue,
	"google.protobuf.BytesValue":  WKBytesValue,
}

// classifyWellKnown returns the WellKnown for a full proto name, or WKNone.
func classifyWellKnown(fullName string) WellKnown {
	return wellKnownByName[fullName]
}

// Wrapper reports the Kind a google.protobuf wrapper type boxes, and whether the
// WellKnown is a wrapper at all.
//
// Every target unwraps these rather than emitting a one-field record: the whole
// purpose of a wrapper in proto3 is to express presence, and each target already
// has its own way to say that.
func (w WellKnown) Wrapper() (Kind, bool) {
	switch w {
	case WKDoubleValue:
		return KindDouble, true
	case WKFloatValue:
		return KindFloat, true
	case WKInt64Value:
		return KindInt64, true
	case WKUint64Value:
		return KindUint64, true
	case WKInt32Value:
		return KindInt32, true
	case WKUint32Value:
		return KindUint32, true
	case WKBoolValue:
		return KindBool, true
	case WKStringValue:
		return KindString, true
	case WKBytesValue:
		return KindBytes, true
	}
	return 0, false
}

// Temporal reports whether the type is a point in time or a span of it, which is
// the group every target substitutes rather than emits.
func (w WellKnown) Temporal() bool { return w == WKTimestamp || w == WKDuration }

// String returns the short proto name, for diagnostics.
func (w WellKnown) String() string {
	for name, got := range wellKnownByName {
		if got == w {
			return name
		}
	}
	return "none"
}
