package wire

// topics.go renders Kotlin constants for the topics a schema publishes.
//
// It exists for the reason the ROS manifest does: the binding between a topic
// name and the message carried on it appears nowhere in the generated protobuf
// types, so subscribing otherwise means retyping a string literal that nothing
// checks.

import (
	"path"
	"sort"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"

	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/names"
)

// topics renders Kotlin constants for the server-streaming methods.
//
// It exists for the same reason the ROS target's manifest does: the binding
// between a topic name and the message on it is nowhere in the schema a JVM
// consumer compiles against, so subscribing means retyping a string literal that
// nothing checks.
func (r *run) topics() error {
	type entry struct {
		Const   string
		Topic   string
		Type    string
		Source  string
		Doc     string
		Package string
	}

	var entries []entry
	for _, f := range r.schema.Files {
		for _, s := range f.Services {
			if s.Skip || !allows(s.Targets) {
				continue
			}
			for _, m := range s.Methods {
				if m.Transport != buffers.TransportTopic || m.Skip || !allows(m.Targets) {
					continue
				}
				typ := "Unit"
				if m.Output != nil {
					typ = f.JVMPackage + "." + m.Output.Name
				}
				entries = append(entries, entry{
					Const:   names.ScreamingSnake(m.Name),
					Topic:   m.Topic,
					Type:    typ,
					Source:  s.Name + "." + m.Name,
					Doc:     firstLine(m.Doc),
					Package: f.JVMPackage,
				})
			}
		}
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Topic < entries[j].Topic })

	pkg := entries[0].Package

	var b emit.Buf
	b.Raw(r.banner("(every service in this run)"))
	b.Line("")
	b.Linef("package %s", pkg)
	b.Line("")
	b.Line("/**")
	b.Line(" * Topics published by the services in this schema.")
	b.Line(" *")
	b.Line(" * The binding between a topic name and the message carried on it exists nowhere")
	b.Line(" * in the generated protobuf types, so subscribing otherwise means retyping a")
	b.Line(" * string literal that nothing checks. [type] is the message a subscriber should")
	b.Line(" * decode with.")
	b.Line(" */")
	b.Block("object Topics {", "}", func() {
		for i, e := range entries {
			if i > 0 {
				b.Line("")
			}
			if e.Doc != "" {
				b.Linef("/** %s Carries [%s]. */", e.Doc, e.Type)
			} else {
				b.Linef("/** Carries [%s]. */", e.Type)
			}
			b.Linef("const val %s: String = %q", e.Const, e.Topic)
		}
	})

	return r.sink(path.Join(strings.ReplaceAll(pkg, ".", "/"), "Topics.kt"), b.Bytes())
}

// flattenMessages returns every message a file declares, nested ones included.
func flattenMessages(f *buffers.File) []*buffers.Message {
	var out []*buffers.Message
	var walk func(msgs []*buffers.Message)
	walk = func(msgs []*buffers.Message) {
		for _, m := range msgs {
			if !m.Skip && !m.IsMapEntry && allows(m.Targets) {
				out = append(out, m)
			}
			walk(m.Nested)
		}
	}
	walk(f.Messages)
	return out
}

// collect records a diagnostic, ignoring nil so callers can pass a projection's
// result directly.
func (r *run) collect(d *buffers.Diagnostic) {
	if d != nil {
		r.diags = append(r.diags, *d)
	}
}

// banner renders the file header, naming the .proto the schema came from — not
// the output path, since a reader who finds a surprising line needs to know
// which .proto to edit.
func (r *run) banner(source string) string {
	info := r.info
	info.Source = source
	return provenance.Render(provenance.Slash, info)
}

// firstLine returns a doc comment's first line, for a one-line summary.
func firstLine(doc string) string {
	if i := strings.Index(doc, "\n"); i >= 0 {
		return doc[:i]
	}
	return doc
}

// allows reports whether a target allow-list admits this target. An empty list
// admits everything, which is the common case.
func allows(list []string) bool {
	if len(list) == 0 {
		return true
	}
	for _, t := range list {
		if t == "wire" {
			return true
		}
	}
	return false
}
