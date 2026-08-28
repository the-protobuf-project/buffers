package langs

// schemafiles.go finds the schema a target emitted, so a compile does not have to
// be told what to compile.

import (
	"os"
	"path/filepath"
	"sort"
)

// SchemaFiles lists the schema files a target emitted, relative to its directory,
// so a compile does not have to be told what to compile.
func SchemaFiles(dir, ext string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ext {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// Extension returns the schema file extension a target emits.
func Extension(target string) string {
	switch target {
	case "flatbuffers":
		return ".fbs"
	case "capnp":
		return ".capnp"
	case "thrift":
		return ".thrift"
	}
	return ""
}
