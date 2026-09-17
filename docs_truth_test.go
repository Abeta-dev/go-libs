// SPDX-License-Identifier: MIT

package golibs_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentationCatalogMatchesExportedAPIs(t *testing.T) {
	t.Parallel()

	for _, assertion := range []struct {
		packageName string
		document    string
		exports     []string
	}{
		{
			packageName: "sliceutil",
			document:    "README.md",
			exports:     []string{"Reduce", "GroupBy", "Chunk", "Unique", "Flatten", "First"},
		},
		{
			packageName: "maputil",
			document:    "README.md",
			exports:     []string{"Merge", "Filter"},
		},
		{
			packageName: "idempotency",
			document:    "README.md",
			exports:     []string{"NewMemoryStore", "NewPGStore", "Store"},
		},
	} {
		assertion := assertion
		t.Run(assertion.packageName, func(t *testing.T) {
			t.Parallel()
			document, err := os.ReadFile(assertion.document)
			if err != nil {
				t.Fatal(err)
			}
			exported := exportedNames(t, assertion.packageName)
			for _, name := range assertion.exports {
				if !strings.Contains(string(document), "`"+name+"`") && !strings.Contains(string(document), "`"+name+"(") && !strings.Contains(string(document), "`"+assertion.packageName+"."+name) {
					t.Errorf("%s must document %s.%s", assertion.document, assertion.packageName, name)
				}
				if !exported[name] {
					t.Errorf("%s documents missing export %s.%s", assertion.document, assertion.packageName, name)
				}
			}
		})
	}
}

func TestDocumentationDoesNotClaimRemovedUtilityAPIs(t *testing.T) {
	t.Parallel()

	removed := []struct {
		packageName string
		exportName  string
	}{
		{packageName: "sliceutil", exportName: "Map"},
		{packageName: "sliceutil", exportName: "Filter"},
		{packageName: "maputil", exportName: "Keys"},
		{packageName: "maputil", exportName: "Values"},
	}
	for _, path := range []string{
		"README.md",
		"CHANGELOG.md",
		"BENCHMARKS.md",
		"CONTRIBUTING.md",
		"TESTING_PARAMETERS.md",
		"docs",
		"maputil",
		"sliceutil",
	} {
		for _, file := range markdownAndHTMLFiles(t, path) {
			contents, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, removedAPI := range removed {
				qualified := removedAPI.packageName + "." + removedAPI.exportName
				if strings.Contains(string(contents), qualified) {
					t.Errorf("%s claims removed API %s", file, qualified)
				}
			}
		}
	}
}

func exportedNames(t *testing.T, packageName string) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(packageName, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool)
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range parsed.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil && ast.IsExported(declaration.Name.Name) {
					names[declaration.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if ast.IsExported(name.Name) {
								names[name.Name] = true
							}
						}
					case *ast.TypeSpec:
						if ast.IsExported(spec.Name.Name) {
							names[spec.Name.Name] = true
						}
					}
				}
			}
		}
	}
	return names
}

func markdownAndHTMLFiles(t *testing.T, path string) []string {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		return []string{path}
	}
	var files []string
	err = filepath.WalkDir(path, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || (!strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".html")) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
