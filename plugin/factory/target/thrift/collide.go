package thrift

// collide.go reports the cost of Thrift having one flat scope per file.
//
// Proto scopes a nested type under its parent, so `FooBar` and `Foo.Bar` are two
// distinct names. Thrift has no nesting, so both fold onto `FooBar` — and a file
// declaring both emits two structs with one name, which the compiler rejects with
// a message about a duplicate rather than about the nesting that caused it.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"
)

// reportCollisions reports two declarations that flatten onto one Thrift name.
//
// Thrift has a single flat scope per file, so a nested proto type folds its path
// into its name — and `p.FooBar` and `p.Foo.Bar` both fold onto `FooBar`. Proto
// allows both because they are in different scopes; Thrift then sees two structs
// with one name.
//
// It is reported rather than resolved. Renaming one of them would have to pick a
// loser, and which one lost would depend on the presence of the other — so adding
// an unrelated nested message would rename a type that consumers already compile
// against. A diagnostic naming both is the honest outcome: the collision is in the
// proto, and only its author can say which name should give way.
func (r *run) reportCollisions(f *buffers.File) {
	byName := map[string][]string{}
	add := func(name, node string) { byName[name] = append(byName[name], node) }

	for _, m := range flattenMessages(f) {
		add(typeName(string(m.Node), m.Package), string(m.Node))
		for _, one := range liveOneofs(m) {
			add(r.unionTypeName(m, one), string(m.Node)+"."+one.Name+" (union)")
		}
	}
	for _, e := range flattenEnums(f) {
		add(typeName(string(e.Node), e.Package), string(e.Node))
	}
	for _, s := range emittableServices(f) {
		add(typeName(string(s.Node), s.Package), string(s.Node))
	}

	names := make([]string, 0, len(byName))
	for name, nodes := range byName {
		if len(nodes) > 1 {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		nodes := byName[name]
		sort.Strings(nodes)
		r.collect(&buffers.Diagnostic{
			Rule: buffers.RuleTarget,
			Node: buffers.NodeID(nodes[0]),
			Message: fmt.Sprintf("%s all render as Thrift %q; Thrift has one flat scope per file, so a "+
				"nested type folds its path into its name and these collide",
				strings.Join(nodes, ", "), name),
			Hint: "rename one of them in the .proto — nothing here can choose, because renaming the " +
				"loser would change a name consumers already compile against",
		})
	}
}
