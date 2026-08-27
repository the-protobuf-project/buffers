package bufir

// build_enum.go walks enums, and rejects a declared integer width that cannot
// hold every value rather than letting a downstream compiler truncate it.

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
)

// enum walks one enum and its values.
func (b *builder) enum(e *protogen.Enum, file *File, parent *Message) *Enum {
	ed := e.Desc
	full := NodeID(ed.FullName())
	opts := enumOpts(ed)

	enum := &Enum{
		Node:       full,
		Name:       string(ed.Name()),
		Package:    string(ed.ParentFile().Package()),
		File:       file,
		Doc:        docOf(e.Comments.Leading),
		Underlying: widthOf(opts.GetUnderlying()),
		BitFlags:   opts.GetBitFlags(),
		Skip:       opts.GetSkip(),
	}

	for _, v := range e.Values {
		vopts := enumValueOpts(v.Desc)
		value := &EnumValue{
			Node:   NodeID(v.Desc.FullName()),
			Name:   string(v.Desc.Name()),
			Number: int32(v.Desc.Number()),
			Doc:    docOf(v.Comments.Leading),
			Skip:   vopts.GetSkip(),
		}
		if vopts != nil && vopts.Ordinal != 0 {
			value.Ordinal = vopts.GetOrdinal()
		}
		enum.Values = append(enum.Values, value)
	}

	if file.Generate {
		b.checkEnumWidth(enum)
	}

	b.schema.Enums[full] = enum
	b.owner[full] = file.Path
	if parent == nil {
		file.Enums = append(file.Enums, enum)
	} else {
		parent.Enums = append(parent.Enums, enum)
	}
	return enum
}

// checkEnumWidth rejects a declared width that cannot hold every value, rather
// than letting flatc truncate it into a different enum.
func (b *builder) checkEnumWidth(enum *Enum) {
	lo, hi := enum.Underlying.Range()
	for _, v := range enum.Values {
		n := int64(v.Number)
		if n < lo || n > hi {
			b.report(Diagnostic{
				Rule: RuleTarget,
				Node: v.Node,
				Message: fmt.Sprintf("value %d does not fit the declared underlying width (%d-bit, %d to %d)",
					v.Number, enum.Underlying.Bits(), lo, hi),
				Hint: "widen (buffers.v1.enumeration).underlying, or renumber the value",
			})
		}
	}
}
