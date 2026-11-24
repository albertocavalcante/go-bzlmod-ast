# go-bzlmod-ast

Parse and walk `MODULE.bazel` files into structured Go types via the
[bazelbuild `buildtools`](https://github.com/bazelbuild/buildtools)
parser. Standalone library; no runtime dependencies beyond a vendored
copy of buildtools under `third_party/`.

## Install

```bash
go get github.com/albertocavalcante/go-bzlmod-ast
```

## Usage

```go
import (
    "os"

    ast "github.com/albertocavalcante/go-bzlmod-ast"
    "github.com/albertocavalcante/go-bzlmod-ast/label"
)

content, _ := os.ReadFile("MODULE.bazel")
result, err := ast.ParseContent("MODULE.bazel", content)
if err != nil { /* handle */ }
```

From here there are two consumption paths.

**Walk with a `Handler`.** Embed `BaseHandler` and override only the
statements you care about:

```go
type depCollector struct {
    ast.BaseHandler
    Deps []string
}

func (c *depCollector) BazelDep(name label.Module, _ label.Version,
    _ int, _ label.ApparentRepo, _ bool) error {
    c.Deps = append(c.Deps, name.String())
    return nil
}

c := &depCollector{}
_ = ast.Walk(result.File, c)
```

**Type-switch on `result.File.Statements`** when you need the full
typed struct (for example `UseExtension.Tags`, which isn't exposed
through a Handler callback):

```go
for _, stmt := range result.File.Statements {
    switch s := stmt.(type) {
    case *ast.BazelDep:
        // ...
    case *ast.UseExtension:
        for _, tag := range s.Tags {
            // ...
        }
    }
}
```

Each typed statement exposes its source `Span` via the `Statement`
interface — `stmt.Span().Start` for the start, `.End` for the end.
The same `Span` is reachable as the typed struct's `Pos` field.

### Following `include()`

`ParseContent` records `include()` as an `*ast.Include` and does not
open the named file. To see dependencies declared in included files,
expand them over an `io/fs.FS` rooted at the workspace:

```go
root, err := os.OpenRoot(workspaceRoot) // confines symlinks, unlike os.DirFS
if err != nil { /* handle */ }
defer root.Close()
x, err := ast.ParseWithIncludes(ctx, root.FS(), ast.IncludeOptions{})
if err != nil { /* *ast.IncludeError; match with errors.Is(err, ast.ErrIncludeNotFound) etc. */ }
for _, s := range x.Statements {
    if dep, ok := s.Statement.(*ast.BazelDep); ok {
        fmt.Println(dep.Name, "declared in", s.File.Path) // edit s.File, not the root
    }
}
```

`ExpandIncludes(ctx, parseResult, fsys, opts)` does the same for an
already parsed file. Statements come back in Bazel's evaluation order
with their owning file and include chain; `IncludeOptions` bounds
depth, file count, bytes and statements. Unlike Bazel, a BUILD file in
the included file's package is not required.

## Package layout

- `parser.go` — `ParseFile(path)` and `ParseContent(filename, data)`;
  returns a `*ParseResult` with the typed `*ModuleFile`, plus errors
  and warnings.
- `types.go` — `Statement` interface and the concrete types for every
  recognized MODULE.bazel directive: `ModuleDecl`, `BazelDep`,
  `UseExtension`, `UseRepo`, `SingleVersionOverride`,
  `MultipleVersionOverride`, `GitOverride`, `ArchiveOverride`,
  `LocalPathOverride`, `RegisterToolchains`,
  `RegisterExecutionPlatforms`, `Include`, `ExtensionTagCall`,
  `UseRepoRule`, `InjectRepo`, `OverrideRepo`, `FlagAlias`,
  `UnknownStatement`.
- `include.go` — `ExpandIncludes`, `ParseWithIncludes`, `IncludePath`
  and the typed `IncludeError`.
- `handler.go` — the `Handler` interface, `Walk(file, handler)`,
  `BaseHandler` no-op embed, and two ready-made collectors
  (`DependencyCollector`, `OverrideCollector`).
- `label/` — typed `Module`, `Version`, `ApparentRepo`,
  `ApparentLabel`, `StarlarkIdentifier`. Used in the Handler
  signatures so callers don't have to re-validate strings.
- `buildutil/` — generic attribute-extraction helpers over the
  buildtools AST (`String`, `Bool`, `Int`, `StringList`,
  `ExtractValue`).
- `third_party/buildtools/` — vendored buildtools parser
  (Apache-2.0).

## Status

Pre-1.0. The API may change.

## License

MIT for code in this repository's own packages. The vendored
`third_party/buildtools/` tree carries its upstream Apache-2.0 license
(`third_party/buildtools/LICENSE`).
