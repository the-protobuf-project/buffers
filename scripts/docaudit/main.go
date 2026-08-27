// Command docaudit reports Go declarations that carry no doc comment.
//
// It exists because no linter checks what this repository wants checked.
// golangci-lint's revive rule covers exported declarations only, and most of this
// codebase is unexported: the targets, the IR internals, the helpers. A reader
// arriving at an unexported field needs the same explanation an exported one
// gets, and often more, because there is no external documentation to fall back
// on.
//
// It reports, per declaration: functions and methods, types, struct fields, and
// package-level vars and consts. Test files are skipped — a test's name is its
// documentation, and requiring a comment on every table entry would be noise.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// finding is one undocumented declaration.
type finding struct {
	File string // path relative to the repository root
	Line int    // 1-based line the declaration starts on
	What string // "func", "type", "field", "var", "const"
	Name string // the declaration's name
}

func main() {
	findings, err := scan("plugin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, f := range findings {
		fmt.Printf("  %s:%d  %s %s\n", f.File, f.Line, f.What, f.Name)
	}
	if len(findings) > 0 {
		os.Exit(1)
	}
}

// scan walks a directory tree and reports every undocumented declaration in it.
func scan(root string) ([]finding, error) {
	var out []finding

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Generated stubs are protoc's output, not this repository's source.
			if d.Name() == "pb" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		found, err := scanFile(path)
		if err != nil {
			return err
		}
		out = append(out, found...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

// scanFile reports the undocumented declarations in one file.
func scanFile(path string) ([]finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	var out []finding
	report := func(pos token.Pos, what, name string) {
		out = append(out, finding{
			File: path,
			Line: fset.Position(pos).Line,
			What: what,
			Name: name,
		})
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Doc == nil {
				report(d.Pos(), "func", funcName(d))
			}
		case *ast.GenDecl:
			scanGenDecl(d, report)
		}
	}
	return out, nil
}

// scanGenDecl reports undocumented types, vars, consts and struct fields.
//
// A grouped declaration — `const ( a = 1; b = 2 )` — may be documented either on
// the group or on each spec, so a spec inside a documented group is accepted.
func scanGenDecl(d *ast.GenDecl, report func(token.Pos, string, string)) {
	grouped := d.Doc != nil

	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Doc == nil && !grouped {
				report(s.Pos(), "type", s.Name.Name)
			}
			scanFields(s, report)
		case *ast.ValueSpec:
			if s.Doc == nil && !grouped {
				report(s.Pos(), kindOf(d.Tok), strings.Join(names(s.Names), ", "))
			}
		}
	}
}

// scanFields reports undocumented fields of a struct type.
//
// An embedded field is skipped: it has no name of its own and its meaning is the
// embedded type's, which is documented where that type is declared.
func scanFields(s *ast.TypeSpec, report func(token.Pos, string, string)) {
	st, ok := s.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return
	}
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue
		}
		if f.Doc == nil && f.Comment == nil {
			report(f.Pos(), "field", s.Name.Name+"."+strings.Join(names(f.Names), ", "))
		}
	}
}

// funcName renders a function or method name, qualified by its receiver.
func funcName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	return "(" + typeName(d.Recv.List[0].Type) + ")." + d.Name.Name
}

// typeName renders a receiver's type, unwrapping a pointer.
func typeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return "*" + typeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return typeName(t.X)
	}
	return "?"
}

// kindOf maps a declaration token to the word used in a finding.
func kindOf(tok token.Token) string {
	if tok == token.CONST {
		return "const"
	}
	return "var"
}

// names renders a list of identifiers.
func names(ids []*ast.Ident) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.Name
	}
	return out
}
