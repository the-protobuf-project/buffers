package vocab

// convert.go translates buffers.v1's generated enums into protokit's neutral
// ones.
//
// One function per proto enum, and deliberately nothing cleverer. A table or a
// numeric cast would couple the two numberings, and they are owned by different
// repositories on different release cycles — a value added to one is a compile
// error here, which is where it should surface.

import (
	"github.com/the-protobuf-project/protokit/buffers"

	bufferspbv1 "github.com/the-protobuf-project/buffers/plugin/pb/bufferspbv1"
)

// layoutOf converts a buffers.v1 layout into protokit's own.
func layoutOf(l bufferspbv1.Layout) buffers.Layout {
	switch l {
	case bufferspbv1.Layout_LAYOUT_TABLE:
		return buffers.LayoutTable
	case bufferspbv1.Layout_LAYOUT_STRUCT:
		return buffers.LayoutStruct
	}
	return buffers.LayoutUnspecified
}

// widthOf converts a buffers.v1 integer width into protokit's own.
func widthOf(w bufferspbv1.IntWidth) buffers.IntWidth {
	switch w {
	case bufferspbv1.IntWidth_INT_WIDTH_INT8:
		return buffers.IntWidthInt8
	case bufferspbv1.IntWidth_INT_WIDTH_UINT8:
		return buffers.IntWidthUint8
	case bufferspbv1.IntWidth_INT_WIDTH_INT16:
		return buffers.IntWidthInt16
	case bufferspbv1.IntWidth_INT_WIDTH_UINT16:
		return buffers.IntWidthUint16
	case bufferspbv1.IntWidth_INT_WIDTH_INT32:
		return buffers.IntWidthInt32
	case bufferspbv1.IntWidth_INT_WIDTH_UINT32:
		return buffers.IntWidthUint32
	case bufferspbv1.IntWidth_INT_WIDTH_INT64:
		return buffers.IntWidthInt64
	case bufferspbv1.IntWidth_INT_WIDTH_UINT64:
		return buffers.IntWidthUint64
	}
	return buffers.IntWidthUnspecified
}

// transportOf converts a buffers.v1 transport into protokit's own.
func transportOf(t bufferspbv1.Transport) buffers.Transport {
	switch t {
	case bufferspbv1.Transport_TRANSPORT_CALL:
		return buffers.TransportCall
	case bufferspbv1.Transport_TRANSPORT_TOPIC:
		return buffers.TransportTopic
	case bufferspbv1.Transport_TRANSPORT_ACTION:
		return buffers.TransportAction
	}
	return buffers.TransportUnspecified
}
