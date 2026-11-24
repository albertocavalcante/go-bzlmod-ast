package ast

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestIncludePath(t *testing.T) {
	ok := map[string]string{
		"//src/bazel/modules:rules_antlr.MODULE.bazel": "src/bazel/modules/rules_antlr.MODULE.bazel",
		"//:deps.MODULE.bazel":                         "deps.MODULE.bazel",
		"//a:b/c.MODULE.bazel":                         "a/b/c.MODULE.bazel",
		"//a/x.MODULE.bazel":                           "a/x.MODULE.bazel/x.MODULE.bazel",
	}
	for in, want := range ok {
		if got, err := IncludePath(in); err != nil || got != want {
			t.Errorf("IncludePath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{
		"", ":deps.MODULE.bazel", "deps.MODULE.bazel", "@repo//:deps.MODULE.bazel", "@@//:deps.MODULE.bazel",
		"//", "//:", "//../x:deps.MODULE.bazel", "//a:../deps.MODULE.bazel", "//a:./deps.MODULE.bazel",
		"///a:deps.MODULE.bazel", "//a/:deps.MODULE.bazel", "//a:b//deps.MODULE.bazel", `//a\b:deps.MODULE.bazel`,
		"//a:b:deps.MODULE.bazel", "//:MODULE.bazel", "//:.MODULE.bazel", "//:deps.bzl", "//:deps.module.bazel",
		"//:deps.MODULE.bazel/", "//:a\x00.MODULE.bazel", "//a:\n.MODULE.bazel", "//\u00e9:x.MODULE.bazel",
		"//...:x.MODULE.bazel", "//a/.../b:x.MODULE.bazel", "//a/..../b:x.MODULE.bazel",
	} {
		if got, err := IncludePath(in); err == nil {
			t.Errorf("IncludePath(%q) = %q, want error", in, got)
		}
	}
}

func expand(t *testing.T, fsys fstest.MapFS, opts IncludeOptions) (*ExpandedFile, error) {
	t.Helper()
	return ParseWithIncludes(context.Background(), fsys, opts)
}

// summary renders each expanded statement as "file:line kind [chain]".
func summary(x *ExpandedFile) string {
	var b strings.Builder
	for _, s := range x.Statements {
		var chain []string
		for _, c := range s.Chain {
			chain = append(chain, fmt.Sprintf("%s@%d", c.Path, c.Pos.Start.Line))
		}
		fmt.Fprintf(&b, "%s:%d %T [%s]\n", s.File.Path, s.Span().Start.Line, s.Statement, strings.Join(chain, " "))
	}
	return b.String()
}

func TestExpandIncludes_OrderAndProvenance(t *testing.T) {
	fsys := fstest.MapFS{
		"MODULE.bazel": {Data: []byte(`module(name = "root")
bazel_dep(name = "a", version = "1")
include("//deps:go.MODULE.bazel")
bazel_dep(name = "z", version = "1")
`)},
		"deps/go.MODULE.bazel": {Data: []byte("bazel_dep(name = \"b\", version = \"1\")\r\n\r\ninclude(\"//deps:more/x.MODULE.bazel\")\r\nbazel_dep(name = \"d\", version = \"1\")\r\n")},
		"deps/more/x.MODULE.bazel": {Data: []byte(`go = use_extension("@rules_go//go:extensions.bzl", "go_sdk")
go.download(version = "1.23")
bazel_dep(name = "c", version = "1")
`)},
	}
	x, err := expand(t, fsys, IncludeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := `MODULE.bazel:1 *ast.ModuleDecl []
MODULE.bazel:2 *ast.BazelDep []
MODULE.bazel:3 *ast.Include []
deps/go.MODULE.bazel:1 *ast.BazelDep [deps/go.MODULE.bazel@3]
deps/go.MODULE.bazel:3 *ast.Include [deps/go.MODULE.bazel@3]
deps/more/x.MODULE.bazel:1 *ast.UseExtension [deps/go.MODULE.bazel@3 deps/more/x.MODULE.bazel@3]
deps/more/x.MODULE.bazel:3 *ast.BazelDep [deps/go.MODULE.bazel@3 deps/more/x.MODULE.bazel@3]
deps/go.MODULE.bazel:4 *ast.BazelDep [deps/go.MODULE.bazel@3]
MODULE.bazel:4 *ast.BazelDep []
`
	if got := summary(x); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if len(x.Files) != 3 || x.Files[2].Path != "deps/more/x.MODULE.bazel" {
		t.Errorf("Files = %v", x.Files)
	}
	if ue := x.Statements[5].Statement.(*UseExtension); len(ue.Tags) != 1 {
		t.Errorf("tags not linked within included file: %+v", ue)
	}
	if c := x.Statements[6].Chain[1]; c.Label != "//deps:more/x.MODULE.bazel" || c.Pos.Start.Filename != "deps/go.MODULE.bazel" {
		t.Errorf("chain site = %+v", c)
	}
	again, _ := expand(t, fsys, IncludeOptions{})
	if summary(again) != summary(x) {
		t.Error("expansion is not deterministic")
	}
}

func TestExpandIncludes_DuplicateAndDev(t *testing.T) {
	fsys := fstest.MapFS{
		"MODULE.bazel": {Data: []byte(`include("//:a.MODULE.bazel")
include("//:a.MODULE.bazel")
include("//:dev.MODULE.bazel", dev_dependency = True)
`)},
		"a.MODULE.bazel":   {Data: []byte(`bazel_dep(name = "a", version = "1")`)},
		"dev.MODULE.bazel": {Data: []byte(`bazel_dep(name = "d", version = "1")`)},
	}
	x, err := expand(t, fsys, IncludeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := `MODULE.bazel:1 *ast.Include []
a.MODULE.bazel:1 *ast.BazelDep [a.MODULE.bazel@1]
MODULE.bazel:2 *ast.Include []
a.MODULE.bazel:1 *ast.BazelDep [a.MODULE.bazel@2]
MODULE.bazel:3 *ast.Include []
dev.MODULE.bazel:1 *ast.BazelDep [dev.MODULE.bazel@3]
`
	if got := summary(x); got != want || len(x.Files) != 3 {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if x.Statements[1].Statement != x.Statements[3].Statement {
		t.Error("a file included twice should be parsed once and spliced twice")
	}
	fsys["MODULE.bazel"] = &fstest.MapFile{Data: []byte(`include("//:dev.MODULE.bazel", dev_dependency = True)
include("//:a.MODULE.bazel")
`)}
	for _, opts := range []IncludeOptions{{IgnoreDevDependency: true}, {Role: OverrideModule}} {
		x, err := expand(t, fsys, opts)
		if err != nil || len(x.Statements) != 3 {
			t.Errorf("%+v: dev include not skipped: %v\n%s", opts, err, summary(x))
		}
	}
}

func TestExpandIncludes_Errors(t *testing.T) {
	root := func(body string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(body)} }
	big := root(strings.Repeat("# padding\n", 200))
	tests := []struct {
		name string
		fsys fstest.MapFS
		opts IncludeOptions
		want error
		msg  string
	}{
		{"missing", fstest.MapFS{"MODULE.bazel": root("\ninclude(\"//x:gone.MODULE.bazel\")")}, IncludeOptions{}, ErrIncludeNotFound, "MODULE.bazel:2:1: included file not found: \"//x:gone.MODULE.bazel\" (x/gone.MODULE.bazel)"},
		{"nested missing", fstest.MapFS{"MODULE.bazel": root(`include("//:a.MODULE.bazel")`), "a.MODULE.bazel": root(`include("//b:gone.MODULE.bazel")`)}, IncludeOptions{}, ErrIncludeNotFound, "a.MODULE.bazel:1:1: included file not found: \"//b:gone.MODULE.bazel\" (b/gone.MODULE.bazel)"},
		{"root missing", fstest.MapFS{}, IncludeOptions{}, ErrRootModuleNotFound, "MODULE.bazel: root MODULE.bazel not found: open MODULE.bazel"},
		{"directory", fstest.MapFS{"MODULE.bazel": root(`include("//:d.MODULE.bazel")`), "d.MODULE.bazel/f": root("")}, IncludeOptions{}, ErrIncludeUnreadable, "not a regular file"},
		{"unknown role", fstest.MapFS{"MODULE.bazel": root("")}, IncludeOptions{Role: 7}, ErrInvalidIncludeOptions, "unknown ModuleRole 7"},
		{"self cycle", fstest.MapFS{"MODULE.bazel": root(`include("//:a.MODULE.bazel")`), "a.MODULE.bazel": root(`include("//:a.MODULE.bazel")`)}, IncludeOptions{}, ErrIncludeCycle, "//:a.MODULE.bazel -> //:a.MODULE.bazel"},
		{"indirect cycle", fstest.MapFS{"MODULE.bazel": root(`include("//:a.MODULE.bazel")`), "a.MODULE.bazel": root(`include("//b:b.MODULE.bazel")`), "b/b.MODULE.bazel": root(`include("//:a.MODULE.bazel")`)}, IncludeOptions{}, ErrIncludeCycle, "b/b.MODULE.bazel:1:1: include() cycle: \"//:a.MODULE.bazel\" (a.MODULE.bazel): //:a.MODULE.bazel -> //b:b.MODULE.bazel -> //:a.MODULE.bazel"},
		{"wrong suffix", fstest.MapFS{"MODULE.bazel": root(`include("//:deps.bzl")`)}, IncludeOptions{}, ErrInvalidIncludeLabel, "must end in"},
		{"escapes root", fstest.MapFS{"MODULE.bazel": root(`include("//..:x.MODULE.bazel")`)}, IncludeOptions{}, ErrInvalidIncludeLabel, "invalid package"},
		{"repo qualified", fstest.MapFS{"MODULE.bazel": root(`include("@other//:x.MODULE.bazel")`)}, IncludeOptions{}, ErrInvalidIncludeLabel, `must start with "//"`},
		{"relative", fstest.MapFS{"MODULE.bazel": root(`include(":x.MODULE.bazel")`)}, IncludeOptions{}, ErrInvalidIncludeLabel, `must start with "//"`},
		{"module in include", fstest.MapFS{"MODULE.bazel": root(`include("//:a.MODULE.bazel")`), "a.MODULE.bazel": root("\n\nmodule(name = \"x\")")}, IncludeOptions{}, ErrIncludeForbiddenStatement, "a.MODULE.bazel:3:1: statement not allowed"},
		{"registry module", fstest.MapFS{"MODULE.bazel": root(`include("//:a.MODULE.bazel")`)}, IncludeOptions{Role: RegistryModule}, ErrIncludeNotAllowed, "only the root module"},
		{"depth", fstest.MapFS{"MODULE.bazel": root(`include("//:a.MODULE.bazel")`), "a.MODULE.bazel": root(`include("//:b.MODULE.bazel")`), "b.MODULE.bazel": root("")}, IncludeOptions{MaxDepth: 1}, ErrIncludeLimit, "MaxDepth (1) reached"},
		{"files", fstest.MapFS{"MODULE.bazel": root("include(\"//:a.MODULE.bazel\")\ninclude(\"//:a.MODULE.bazel\")"), "a.MODULE.bazel": root("")}, IncludeOptions{MaxFiles: 1}, ErrIncludeLimit, "MaxFiles (1) reached"},
		{"statements", fstest.MapFS{"MODULE.bazel": root(strings.Repeat("include(\"//:a.MODULE.bazel\")\n", 10)), "a.MODULE.bazel": root(strings.Repeat("x()\n", 50))}, IncludeOptions{MaxStatements: 100}, ErrIncludeLimit, "MaxStatements (100) reached"},
		{"file bytes", fstest.MapFS{"MODULE.bazel": root(`include("//:a.MODULE.bazel")`), "a.MODULE.bazel": big}, IncludeOptions{MaxFileBytes: 1000}, ErrIncludeLimit, "MaxFileBytes (1000) exceeded"},
		{"total bytes", fstest.MapFS{"MODULE.bazel": root(`include("//:a.MODULE.bazel")`), "a.MODULE.bazel": big}, IncludeOptions{MaxTotalBytes: 1500}, ErrIncludeLimit, "MaxTotalBytes (1500) exceeded"},
		{"syntax error", fstest.MapFS{"MODULE.bazel": root(`include("//:a.MODULE.bazel")`), "a.MODULE.bazel": root("bazel_dep(")}, IncludeOptions{}, ErrIncludeUnreadable, "syntax error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, err := expand(t, tt.fsys, tt.opts)
			if x == nil {
				t.Fatal("result must be non-nil on failure")
			}
			var ie *IncludeError
			if !errors.Is(err, tt.want) || !errors.As(err, &ie) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.msg) {
				t.Errorf("err = %q, want it to contain %q", err, tt.msg)
			}
		})
	}
}

func TestExpandIncludes_MalformedIncludeIsParseError(t *testing.T) {
	for _, call := range []string{
		`include(label = "//:x.MODULE.bazel")`,
		`include("//:x.MODULE.bazel", dev_dependency = 1)`,
		`include("//:x.MODULE.bazel", "//:y.MODULE.bazel")`,
	} {
		fsys := fstest.MapFS{"MODULE.bazel": {Data: []byte(call)}, "x.MODULE.bazel": {Data: []byte(`bazel_dep(name = "x", version = "1")`)}}
		x, err := expand(t, fsys, IncludeOptions{})
		if err != nil || !x.HasErrors() || len(x.Statements) != 0 || !strings.Contains(x.Errors[0].Error(), "MODULE.bazel:1:1: include: must be called with one positional") {
			t.Errorf("%s: err = %v, errors = %v, statements = %d", call, err, x.Errors, len(x.Statements))
		}
	}
}

func TestExpandIncludes_PartialResultAndHasErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"MODULE.bazel":   {Data: []byte("bazel_dep(name = \"r\", version = \"1\")\ninclude(\"//:a.MODULE.bazel\")\nbazel_dep(name = \"never\", version = \"1\")")},
		"a.MODULE.bazel": {Data: []byte("bazel_dep(version = \"1\")\ninclude(\"//:gone.MODULE.bazel\")")},
	}
	x, err := expand(t, fsys, IncludeOptions{})
	if !errors.Is(err, ErrIncludeNotFound) {
		t.Fatalf("err = %v", err)
	}
	want := `MODULE.bazel:1 *ast.BazelDep []
MODULE.bazel:2 *ast.Include []
a.MODULE.bazel:2 *ast.Include [a.MODULE.bazel@2]
`
	if got := summary(x); got != want || len(x.Files) != 2 || !x.HasErrors() {
		t.Errorf("partial result:\n%s files=%d errors=%v", got, len(x.Files), x.Errors)
	}
	delete(fsys, "a.MODULE.bazel")
	fsys["a.MODULE.bazel"] = &fstest.MapFile{Data: []byte(`bazel_dep(version = "1")`)}
	if x, err := expand(t, fsys, IncludeOptions{}); err != nil || !x.HasErrors() {
		t.Errorf("invalid statement in an included file: err = %v, HasErrors = %v", err, x.HasErrors())
	}
}

// cancelFS cancels its context when a given file is opened.
type cancelFS struct {
	fstest.MapFS
	name   string
	cancel context.CancelFunc
}

func (c cancelFS) Open(name string) (fs.File, error) {
	if name == c.name {
		c.cancel()
	}
	return c.MapFS.Open(name)
}

func TestExpandIncludes_CanceledDuringParse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fsys := cancelFS{fstest.MapFS{
		"MODULE.bazel":   {Data: []byte(`include("//:a.MODULE.bazel")`)},
		"a.MODULE.bazel": {Data: []byte(`bazel_dep(name = "a", version = "1")`)},
	}, "a.MODULE.bazel", cancel}
	x, err := ParseWithIncludes(ctx, fsys, IncludeOptions{})
	if !errors.Is(err, context.Canceled) || x == nil || len(x.Files) != 2 {
		t.Fatalf("err = %v, result = %+v", err, x)
	}
}

func TestExpandIncludes_RegistryDevOnlyAndRootUntouched(t *testing.T) {
	res, _ := ParseContent("MODULE.bazel", []byte(`include("//:d.MODULE.bazel", dev_dependency = True)`))
	x, err := ExpandIncludes(context.Background(), res, fstest.MapFS{}, IncludeOptions{Role: RegistryModule})
	if err != nil || len(x.Statements) != 1 || len(res.File.Statements) != 1 || x.Statements[0].Statement != res.File.Statements[0] {
		t.Fatalf("err = %v, expanded %d, root %d", err, len(x.Statements), len(res.File.Statements))
	}
}

func TestExpandIncludes_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, _ := ParseContent("MODULE.bazel", []byte(`include("//:a.MODULE.bazel")`))
	if _, err := ExpandIncludes(ctx, res, fstest.MapFS{}, IncludeOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// TestExpandIncludes_DirFS proves slash-separated include paths join
// correctly on the host OS, Windows included, through os.DirFS.
func TestExpandIncludes_DirFS(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("MODULE.bazel", `include("//src/bazel/modules:rules_antlr.MODULE.bazel")`)
	write("src/bazel/modules/rules_antlr.MODULE.bazel", `include("//src/bazel/modules:nested/more.MODULE.bazel")`)
	write("src/bazel/modules/nested/more.MODULE.bazel", `bazel_dep(name = "rules_antlr", version = "0.5.0")`)
	x, err := ParseWithIncludes(context.Background(), os.DirFS(dir), IncludeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	last := x.Statements[len(x.Statements)-1]
	if dep, ok := last.Statement.(*BazelDep); !ok || dep.Name.String() != "rules_antlr" || last.File.Path != "src/bazel/modules/nested/more.MODULE.bazel" {
		t.Fatalf("unexpected expansion:\n%s", summary(x))
	}
}

func TestIsAvailableAt_IncludeDevDependency(t *testing.T) {
	for v, want := range map[string]bool{"7.7.3": false, "8.9.0": false, "9.2.0": false, "9.3.0": true, "9.3.3": true} {
		if got := IsAvailableAt("include", "dev_dependency", v); got != want {
			t.Errorf("IsAvailableAt(include, dev_dependency, %s) = %v, want %v", v, got, want)
		}
	}
}
