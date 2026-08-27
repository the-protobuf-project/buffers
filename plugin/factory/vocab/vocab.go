// Package vocab reads the buffers.v1 annotation vocabulary into protokit's
// neutral buffers-IR types.
//
// It is this repository's half of protokit's annotation seam, and the only place
// that knows both spellings. protokit's engine imports no annotation module — its
// import boundary test fails the build if it does — so the IR is configured
// through a [buffers.AnnotationReader] a generator supplies, and this is ours.
//
// The translation is deliberately one-directional and confined here: nothing
// downstream of the build imports bufferspbv1, so a target renders a Layout
// without knowing which proto option the value arrived in.
//
// It does no normalization. An empty name means "derive one" and a negative bound
// means nothing, and protokit applies both rules on its side so that every
// vocabulary applies them identically. The job here is spelling, not policy.
package vocab

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/pb/bufferspbv1"
)

// Reader reads buffers.v1 off a descriptor.
type Reader struct{}

// Reader must satisfy protokit's seam; the compiler is what keeps it in step
// when protokit adds a node kind.
var _ buffers.AnnotationReader = Reader{}

// ReadFile reads (buffers.v1.file).
func (Reader) ReadFile(fd protoreflect.FileDescriptor) buffers.FileAnnotations {
	o := buffers.Extension[*bufferspbv1.FileOptions](fd.Options(), bufferspbv1.E_File)
	return buffers.FileAnnotations{
		Namespace:  o.GetNamespace(),
		ROSPackage: o.GetRosPackage(),
		JVMPackage: o.GetJvmPackage(),
		CapnpID:    o.GetCapnpId(),
		Identifier: o.GetFileId(),
		Extension:  o.GetFileExtension(),
		Includes:   o.GetFbsInclude(),
	}
}

// ReadMessage reads (buffers.v1.message).
func (Reader) ReadMessage(md protoreflect.MessageDescriptor) buffers.MessageAnnotations {
	o := buffers.Extension[*bufferspbv1.MessageOptions](md.Options(), bufferspbv1.E_Message)
	return buffers.MessageAnnotations{
		Layout:        layoutOf(o.GetLayout()),
		CapnpID:       o.GetCapnpId(),
		ROSName:       o.GetRosType(),
		Targets:       o.GetTargets(),
		Skip:          o.GetSkip(),
		OriginalOrder: o.GetFbsOriginalOrder(),
		FBSRoot:       o.GetFbsRoot(),
	}
}

// ReadField reads (buffers.v1.field).
func (Reader) ReadField(fd protoreflect.FieldDescriptor) buffers.FieldAnnotations {
	o := buffers.Extension[*bufferspbv1.FieldOptions](fd.Options(), bufferspbv1.E_Field)
	return buffers.FieldAnnotations{
		Ordinal:    o.GetOrdinal(),
		Skip:       o.GetSkip(),
		Key:        o.GetKey(),
		Shared:     o.GetShared(),
		MaxLen:     o.GetMaxLen(),
		FixedLen:   o.GetFixedLen(),
		ROSDefault: o.GetRosDefault(),
		CapnpGroup: o.GetCapnpGroup(),
		Targets:    o.GetTargets(),
	}
}

// ReadEnum reads (buffers.v1.enumeration).
func (Reader) ReadEnum(ed protoreflect.EnumDescriptor) buffers.EnumAnnotations {
	o := buffers.Extension[*bufferspbv1.EnumOptions](ed.Options(), bufferspbv1.E_Enumeration)
	return buffers.EnumAnnotations{
		Underlying: widthOf(o.GetUnderlying()),
		BitFlags:   o.GetBitFlags(),
		Skip:       o.GetSkip(),
	}
}

// ReadEnumValue reads (buffers.v1.enum_value).
func (Reader) ReadEnumValue(vd protoreflect.EnumValueDescriptor) buffers.EnumValueAnnotations {
	o := buffers.Extension[*bufferspbv1.EnumValueOptions](vd.Options(), bufferspbv1.E_EnumValue)
	return buffers.EnumValueAnnotations{
		Ordinal: o.GetOrdinal(),
		Skip:    o.GetSkip(),
	}
}

// ReadOneof reads (buffers.v1.oneof).
func (Reader) ReadOneof(od protoreflect.OneofDescriptor) buffers.OneofAnnotations {
	o := buffers.Extension[*bufferspbv1.OneofOptions](od.Options(), bufferspbv1.E_Oneof)
	return buffers.OneofAnnotations{
		UnionName: o.GetUnionType(),
		Skip:      o.GetSkip(),
	}
}

// ReadService reads (buffers.v1.service).
//
// CapnpInterface and RosService stay pointers all the way through: both have a
// default protokit computes from the methods, and a declaration overrides it in
// either direction. Flattening either to a bool here would lose the difference
// between "not declared" and "declared false", and "false" is a meaningful thing
// to say about both.
func (Reader) ReadService(sd protoreflect.ServiceDescriptor) buffers.ServiceAnnotations {
	o := buffers.Extension[*bufferspbv1.ServiceOptions](sd.Options(), bufferspbv1.E_Service)
	a := buffers.ServiceAnnotations{
		CapnpID: o.GetCapnpId(),
		Targets: o.GetTargets(),
		Skip:    o.GetSkip(),
	}
	// Read off the fields rather than the getters, which flatten a nil *bool to
	// false — the one thing these two must not do. That costs the nil check the
	// getters would have performed: an absent option is a typed nil pointer, not a
	// nil interface, so the field access panics without it.
	if o != nil {
		a.CapnpInterface = o.CapnpInterface
		a.ROSService = o.RosService
	}
	return a
}

// ReadMethod reads (buffers.v1.method).
func (Reader) ReadMethod(md protoreflect.MethodDescriptor) buffers.MethodAnnotations {
	o := buffers.Extension[*bufferspbv1.MethodOptions](md.Options(), bufferspbv1.E_Method)
	return buffers.MethodAnnotations{
		Ordinal:   o.GetOrdinal(),
		ROSName:   o.GetRosType(),
		Targets:   o.GetTargets(),
		Skip:      o.GetSkip(),
		Transport: transportOf(o.GetTransport()),
		Topic:     o.GetTopic(),
	}
}

// Spellings names buffers.v1's options for the diagnostics protokit emits.
//
// A hint is only useful if it names the thing to type, and protokit owns no
// annotation module — so without these the messages fall back to describing the
// option's effect rather than naming it.
func Spellings() buffers.Vocabulary {
	return buffers.Vocabulary{
		FieldOrdinal:   "(buffers.v1.field).ordinal",
		FieldFixedLen:  "(buffers.v1.field).fixed_len",
		EnumUnderlying: "(buffers.v1.enumeration).underlying",
		MethodSkip:     "(buffers.v1.method).skip",
		FileROSPackage: "(buffers.v1.file).ros_package",
	}
}

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
