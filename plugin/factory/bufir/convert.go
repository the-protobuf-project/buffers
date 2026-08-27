package bufir

// convert.go translates buffers.v1's generated enums into this package's own, and
// holds the small helpers the walk uses throughout.
//
// The translation is deliberately one-directional and confined here: nothing
// downstream of the build imports bufferspbv1, so a target renders a Layout
// without knowing which proto option it arrived in.

import (
	bufferspbv1 "github.com/the-protobuf-project/buffers/plugin/pb/bufferspbv1"
)

// orDefault returns got, or fallback when got is empty.
func orDefault(got, fallback string) string {
	if got == "" {
		return fallback
	}
	return got
}

// nonNegative clamps a bound to zero. The options are int32 because AIP-141
// forbids unsigned types in an API surface, so a negative value is expressible
// and means nothing.
func nonNegative(n int32) uint32 {
	if n < 0 {
		return 0
	}
	return uint32(n)
}

// layoutOf converts a buffers.v1 layout into this package's own.
func layoutOf(l bufferspbv1.Layout) Layout {
	switch l {
	case bufferspbv1.Layout_LAYOUT_TABLE:
		return LayoutTable
	case bufferspbv1.Layout_LAYOUT_STRUCT:
		return LayoutStruct
	}
	return LayoutUnspecified
}

// widthOf converts a buffers.v1 integer width into this package's own.
func widthOf(w bufferspbv1.IntWidth) IntWidth {
	switch w {
	case bufferspbv1.IntWidth_INT_WIDTH_INT8:
		return IntWidthInt8
	case bufferspbv1.IntWidth_INT_WIDTH_UINT8:
		return IntWidthUint8
	case bufferspbv1.IntWidth_INT_WIDTH_INT16:
		return IntWidthInt16
	case bufferspbv1.IntWidth_INT_WIDTH_UINT16:
		return IntWidthUint16
	case bufferspbv1.IntWidth_INT_WIDTH_INT32:
		return IntWidthInt32
	case bufferspbv1.IntWidth_INT_WIDTH_UINT32:
		return IntWidthUint32
	case bufferspbv1.IntWidth_INT_WIDTH_INT64:
		return IntWidthInt64
	case bufferspbv1.IntWidth_INT_WIDTH_UINT64:
		return IntWidthUint64
	}
	return IntWidthUnspecified
}

// transportOf converts a buffers.v1 transport into this package's own.
func transportOf(t bufferspbv1.Transport) Transport {
	switch t {
	case bufferspbv1.Transport_TRANSPORT_CALL:
		return TransportCall
	case bufferspbv1.Transport_TRANSPORT_TOPIC:
		return TransportTopic
	case bufferspbv1.Transport_TRANSPORT_ACTION:
		return TransportAction
	}
	return TransportUnspecified
}
