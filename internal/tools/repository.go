package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RepoMap(root string) (string, error) {
	if root == "" {
		root = "."
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	count := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == "dist" || d.Name() == "build" || d.Name() == "target") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintln(&out, filepath.ToSlash(rel))
		count++
		if filepath.Ext(path) == ".go" {
			info, e := d.Info()
			if e == nil && info.Size() < 300000 {
				fs := token.NewFileSet()
				file, e := parser.ParseFile(fs, path, nil, parser.SkipObjectResolution)
				if e == nil {
					for _, decl := range file.Decls {
						switch n := decl.(type) {
						case *ast.FuncDecl:
							fmt.Fprintf(&out, "  %d: func %s\n", fs.Position(n.Pos()).Line, n.Name.Name)
						case *ast.GenDecl:
							for _, spec := range n.Specs {
								if typ, ok := spec.(*ast.TypeSpec); ok {
									fmt.Fprintf(&out, "  %d: type %s\n", fs.Position(typ.Pos()).Line, typ.Name.Name)
								}
							}
						}
					}
				}
			}
		}
		if count >= 200 || out.Len() >= 16000 {
			out.WriteString("(map truncated; use glob/grep for more)\n")
			return filepath.SkipAll
		}
		return nil
	})
	return out.String(), err
}

func GitSummary(ctx context.Context, path string) (string, error) {
	if path == "" {
		path = "."
	}
	var out strings.Builder
	for _, args := range [][]string{{"status", "--short", "--branch"}, {"diff", "--stat"}, {"diff", "--cached", "--stat"}, {"log", "-5", "--oneline"}} {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", path, "--no-pager"}, args...)...)
		data, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %w: %s", args[0], err, data)
		}
		fmt.Fprintf(&out, "git %s\n%s\n", strings.Join(args, " "), data)
	}
	return out.String(), nil
}
