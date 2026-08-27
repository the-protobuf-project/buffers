package capnp

// imports.go binds each imported proto file to a `using` alias.
//
// Cap'n Proto has no implicit cross-file scope: a type in another file is only
// reachable through an alias the importing file declares, so every reference to
// one has to be qualified by a name this file invents.

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/the-protobuf-project/buffers/plugin/factory/bufir"
)

// assignAliases binds each imported proto file to a `using` name.
//
// The alias is the file's base name in PascalCase, which reads well and is stable.
// Two imports whose base names collide — a `common.proto` in two packages — are
// disambiguated by prefixing the parent directory, so the alias stays derived
// rather than becoming a counter that shifts when an import is added.
func (r *run) assignAliases(f *bufir.File) map[string]string {
	imports := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		if strings.HasPrefix(imp, "google/protobuf/") {
			continue // substituted by the prelude
		}
		imports = append(imports, imp)
	}
	sort.Strings(imports)

	counts := map[string]int{}
	for _, imp := range imports {
		counts[baseAlias(imp)]++
	}

	out := make(map[string]string, len(imports))
	for _, imp := range imports {
		alias := baseAlias(imp)
		if counts[alias] > 1 {
			alias = typeName(path.Base(path.Dir(imp))) + alias
		}
		out[imp] = alias
	}
	return out
}

// baseAlias derives an alias from a proto path's base name.
func baseAlias(protoPath string) string {
	base := strings.TrimSuffix(path.Base(protoPath), path.Ext(protoPath))
	return typeName(base)
}

// aliasFor returns the `using` name bound to another file.
func (r *run) aliasFor(f *bufir.File) string {
	if alias, ok := r.aliases[f.Path]; ok {
		return alias
	}
	return baseAlias(f.Path)
}

// usings renders the import lines, including the prelude when the body needed it.
func (r *run) usings(f *bufir.File) []string {
	paths := make([]string, 0, len(r.aliases))
	for p := range r.aliases {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := make([]string, 0, len(paths)+1)
	for _, p := range paths {
		out = append(out, fmt.Sprintf("using %s = import \"/%s\";", r.aliases[p], capnpPath(p)))
	}
	if len(r.fileNeeds) > 0 {
		out = append(out, fmt.Sprintf("using %s = import \"/%s\";", preludeAlias, preludePath))
	}
	sort.Strings(out)
	return out
}
