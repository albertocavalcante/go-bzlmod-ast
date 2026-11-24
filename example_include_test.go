package ast_test

import (
	"context"
	"fmt"
	"testing/fstest"

	ast "github.com/albertocavalcante/go-bzlmod-ast"
)

func ExampleParseWithIncludes() {
	// In a real tool: root, err := os.OpenRoot(workspaceRoot); fsys := root.FS()
	// (it confines symlinks to the root, which os.DirFS does not).
	fsys := fstest.MapFS{
		"MODULE.bazel":            {Data: []byte(`include("//bazel:deps.MODULE.bazel")`)},
		"bazel/deps.MODULE.bazel": {Data: []byte(`bazel_dep(name = "rules_go", version = "0.50.1")`)},
	}
	x, err := ast.ParseWithIncludes(context.Background(), fsys, ast.IncludeOptions{})
	if err != nil || x.HasErrors() {
		panic(fmt.Sprint(err, x.Errors))
	}
	for _, s := range x.Statements {
		if dep, ok := s.Statement.(*ast.BazelDep); ok {
			fmt.Printf("%s %s from %s:%d\n", dep.Name, dep.Version, s.File.Path, dep.Pos.Start.Line)
		}
	}
	// Output: rules_go 0.50.1 from bazel/deps.MODULE.bazel:1
}
