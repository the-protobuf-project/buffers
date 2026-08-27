package bufir

// build_file.go walks one .proto file into a File, and works out which of its
// imports actually contribute a type.

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

// file walks one .proto into a File, resolving the per-target names it carries.
func (b *builder) file(f *protogen.File) *File {
	fd := f.Desc
	generate := f.Generate
	pkg := string(fd.Package())
	opts := fileOpts(fd)

	file := &File{
		Path:       fd.Path(),
		Package:    pkg,
		Doc:        fileDoc(fd),
		Namespace:  orDefault(opts.GetNamespace(), pkg),
		ROSPackage: orDefault(opts.GetRosPackage(), strings.ReplaceAll(pkg, ".", "_")),
		JVMPackage: orDefault(opts.GetJvmPackage(), orDefault(javaPackage(fd), pkg)),
		CapnpID:    resolveCapnpID(opts.GetCapnpId(), func() uint64 { return fileCapnpID(fd.Path()) }),
		Identifier: opts.GetFileId(),
		Extension:  opts.GetFileExtension(),
		Includes:   append([]string(nil), opts.GetFbsInclude()...),
	}
	file.GoImport, file.GoPackage = goPackage(fd)

	// A FlatBuffers file_identifier is exactly four bytes in the buffer header.
	// flatc rejects anything else, and finding that out from flatc rather than
	// from here costs a round trip through a toolchain the plugin may not have
	// even run.
	if id := file.Identifier; generate && id != "" && len(id) != 4 {
		b.report(Diagnostic{
			Rule:    RuleTarget,
			Node:    NodeID(file.Path),
			Message: fmt.Sprintf("file_id %q is %d characters; a FlatBuffers file_identifier is exactly 4", id, len(id)),
			Hint:    "pick a 4-character ASCII tag, or leave file_id empty to emit no identifier",
		})
	}
	return file
}

// resolveImports fills in the proto paths a file's own types actually reference.
//
// It is computed from the references rather than copied from the descriptor's
// import list because the two differ, and the difference matters: an unused
// `include` in an emitted .fbs is a compile-time dependency the consumer did not
// need to take, and a .proto routinely imports things only its options use.
func (b *builder) resolveImports(file *File) {
	seen := map[string]bool{}
	var walk func(m *Message)
	walk = func(m *Message) {
		for _, f := range m.Fields {
			for _, ref := range []string{f.Message, f.Enum} {
				if ref == "" {
					continue
				}
				if path := b.owner[NodeID(ref)]; path != "" && path != file.Path {
					seen[path] = true
				}
			}
			for _, entry := range []*Field{f.MapKey, f.MapValue} {
				if entry == nil {
					continue
				}
				for _, ref := range []string{entry.Message, entry.Enum} {
					if ref == "" {
						continue
					}
					if path := b.owner[NodeID(ref)]; path != "" && path != file.Path {
						seen[path] = true
					}
				}
			}
		}
		for _, n := range m.Nested {
			walk(n)
		}
	}
	for _, m := range file.Messages {
		walk(m)
	}
	for _, s := range file.Services {
		for _, method := range s.Methods {
			for _, msg := range []*Message{method.Input, method.Output} {
				if msg == nil {
					continue
				}
				if path := b.owner[msg.Node]; path != "" && path != file.Path {
					seen[path] = true
				}
			}
		}
	}

	file.Imports = make([]string, 0, len(seen))
	for path := range seen {
		file.Imports = append(file.Imports, path)
	}
	sort.Strings(file.Imports)
}
