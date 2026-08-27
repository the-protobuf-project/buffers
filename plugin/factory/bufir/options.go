package bufir

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	bufferspbv1 "github.com/the-protobuf-project/buffers/plugin/pb/bufferspbv1"
)

// fileOpts returns the buffers.v1 file options, or nil.
func fileOpts(fd protoreflect.FileDescriptor) *bufferspbv1.FileOptions {
	return extension[*bufferspbv1.FileOptions](fd.Options(), bufferspbv1.E_File)
}

// messageOpts returns the buffers.v1 message options, or nil.
func messageOpts(md protoreflect.MessageDescriptor) *bufferspbv1.MessageOptions {
	return extension[*bufferspbv1.MessageOptions](md.Options(), bufferspbv1.E_Message)
}

// fieldOpts returns the buffers.v1 field options, or nil.
func fieldOpts(fd protoreflect.FieldDescriptor) *bufferspbv1.FieldOptions {
	return extension[*bufferspbv1.FieldOptions](fd.Options(), bufferspbv1.E_Field)
}

// enumOpts returns the buffers.v1 enum options, or nil.
func enumOpts(ed protoreflect.EnumDescriptor) *bufferspbv1.EnumOptions {
	return extension[*bufferspbv1.EnumOptions](ed.Options(), bufferspbv1.E_Enumeration)
}

// enumValueOpts returns the buffers.v1 enum value options, or nil.
func enumValueOpts(vd protoreflect.EnumValueDescriptor) *bufferspbv1.EnumValueOptions {
	return extension[*bufferspbv1.EnumValueOptions](vd.Options(), bufferspbv1.E_EnumValue)
}

// oneofOpts returns the buffers.v1 oneof options, or nil.
func oneofOpts(od protoreflect.OneofDescriptor) *bufferspbv1.OneofOptions {
	return extension[*bufferspbv1.OneofOptions](od.Options(), bufferspbv1.E_Oneof)
}

// serviceOpts returns the buffers.v1 service options, or nil.
func serviceOpts(sd protoreflect.ServiceDescriptor) *bufferspbv1.ServiceOptions {
	return extension[*bufferspbv1.ServiceOptions](sd.Options(), bufferspbv1.E_Service)
}

// methodOpts returns the buffers.v1 method options, or nil.
func methodOpts(md protoreflect.MethodDescriptor) *bufferspbv1.MethodOptions {
	return extension[*bufferspbv1.MethodOptions](md.Options(), bufferspbv1.E_Method)
}

// extension is the one place that reaches into proto's extension machinery.
//
// proto.GetExtension panics on a nil message and returns the extension's zero
// value — a typed nil pointer — when the option is absent, neither of which a
// caller wants to think about. This funnels both into "nil means not declared".
func extension[T proto.Message](opts proto.Message, xt protoreflect.ExtensionType) T {
	var zero T
	if opts == nil || !opts.ProtoReflect().IsValid() {
		return zero
	}
	if !proto.HasExtension(opts, xt) {
		return zero
	}
	got, ok := proto.GetExtension(opts, xt).(T)
	if !ok {
		return zero
	}
	return got
}

// extensionSlice reads a repeated extension, which proto.GetExtension returns as
// a slice rather than a message.
func extensionSlice[T any](opts proto.Message, xt protoreflect.ExtensionType) []T {
	if opts == nil || !opts.ProtoReflect().IsValid() || !proto.HasExtension(opts, xt) {
		return nil
	}
	got, _ := proto.GetExtension(opts, xt).([]T)
	return got
}
