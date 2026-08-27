package bufir

// options_aip.go reads the google.api.* vocabulary: the annotations this plugin
// consumes but does not own.
//
// They are separate from buffers.v1's readers because they are a different kind
// of input. buffers.v1 says how a schema should be rendered; AIP says what the
// API *means*, and every target reads it — a REQUIRED field becomes a
// FlatBuffers attribute, a Cap'n Proto doc contract and a ROS comment, from this
// one source.

import (
	"strings"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// behaviors reads google.api.field_behavior (AIP-203).
func behaviors(fd protoreflect.FieldDescriptor) []Behavior {
	raw := extensionSlice[annotations.FieldBehavior](fd.Options(), annotations.E_FieldBehavior)
	if len(raw) == 0 {
		return nil
	}
	out := make([]Behavior, 0, len(raw))
	for _, b := range raw {
		switch b {
		case annotations.FieldBehavior_OPTIONAL:
			out = append(out, BehaviorOptional)
		case annotations.FieldBehavior_REQUIRED:
			out = append(out, BehaviorRequired)
		case annotations.FieldBehavior_OUTPUT_ONLY:
			out = append(out, BehaviorOutputOnly)
		case annotations.FieldBehavior_INPUT_ONLY:
			out = append(out, BehaviorInputOnly)
		case annotations.FieldBehavior_IMMUTABLE:
			out = append(out, BehaviorImmutable)
		case annotations.FieldBehavior_UNORDERED_LIST:
			out = append(out, BehaviorUnorderedList)
		case annotations.FieldBehavior_NON_EMPTY_DEFAULT:
			out = append(out, BehaviorNonEmptyDefault)
		case annotations.FieldBehavior_IDENTIFIER:
			out = append(out, BehaviorIdentifier)
		}
	}
	return out
}

// resourceOf reads google.api.resource (AIP-123) off a message.
//
// NameField is left for the caller to fill: AIP-122 says the resource name lives
// in `name`, but the authority is the field carrying IDENTIFIER, and only the
// field walk has seen those.
func resourceOf(md protoreflect.MessageDescriptor) *Resource {
	rd := extension[*annotations.ResourceDescriptor](md.Options(), annotations.E_Resource)
	if rd == nil {
		return nil
	}
	return &Resource{
		Type:     rd.GetType(),
		Patterns: append([]string(nil), rd.GetPattern()...),
		Singular: rd.GetSingular(),
		Plural:   rd.GetPlural(),
	}
}

// referenceOf reads google.api.resource_reference (AIP-124) off a field, and
// reports whether it was declared as a child_type.
func referenceOf(fd protoreflect.FieldDescriptor) (refType string, child bool) {
	rr := extension[*annotations.ResourceReference](fd.Options(), annotations.E_ResourceReference)
	if rr == nil {
		return "", false
	}
	if t := rr.GetChildType(); t != "" {
		return t, true
	}
	return rr.GetType(), false
}

// formatOf reads google.api.field_info's format, e.g. "UUID4".
func formatOf(fd protoreflect.FieldDescriptor) string {
	fi := extension[*annotations.FieldInfo](fd.Options(), annotations.E_FieldInfo)
	if fi == nil {
		return ""
	}
	if f := fi.GetFormat(); f != annotations.FieldInfo_FORMAT_UNSPECIFIED {
		return f.String()
	}
	return ""
}

// javaPackage reads the file's java_package option, which is the default the
// Wire target's JVM package falls back to.
func javaPackage(fd protoreflect.FileDescriptor) string {
	opts, ok := fd.Options().(interface{ GetJavaPackage() string })
	if !ok {
		return ""
	}
	return opts.GetJavaPackage()
}

// goPackage reads the file's go_package option and splits it into an import path
// and a package name.
//
// The option has two spellings — "path/to/pkg" and "path/to/pkg;alias" — and the
// alias, when present, is the package name rather than the last path segment.
// Both halves are needed: capnpc-go's $Go.import takes the path and $Go.package
// takes the name.
func goPackage(fd protoreflect.FileDescriptor) (importPath, pkgName string) {
	opts, ok := fd.Options().(interface{ GetGoPackage() string })
	if !ok {
		return "", ""
	}
	raw := opts.GetGoPackage()
	if raw == "" {
		return "", ""
	}
	if path, alias, found := strings.Cut(raw, ";"); found {
		return path, alias
	}
	if i := strings.LastIndex(raw, "/"); i >= 0 {
		return raw, raw[i+1:]
	}
	return raw, raw
}

// docOf renders a descriptor's leading comment as plain prose: the "//" markers
// and the single leading space protoc records are stripped, and the line
// structure is kept.
//
// Every target re-comments this in its own syntax — "//" for FlatBuffers, "#" for
// Cap'n Proto and ROS — so carrying the proto spelling through would mean each of
// them stripping it again, differently.
func docOf(c protogen.Comments) string {
	if c == "" {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(c), "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimPrefix(strings.TrimSpace(line), "//")
		out = append(out, strings.TrimPrefix(line, " "))
	}
	// Trailing blank lines carry no meaning and would render as an empty comment
	// line at the end of every doc block.
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}
