# go-bzlmod-ast

Parse and walk `MODULE.bazel` files into structured Go types via
the bazelbuild `buildtools` parser. Standalone library; depends only
on [`go-starlark-syntaxutil`](https://github.com/albertocavalcante/go-starlark-syntaxutil)
for cross-cutting AST helpers and a vendored copy of
[bazelbuild/buildtools](https://github.com/bazelbuild/buildtools).

## What's here

- **`parser.go`** — `Parse(filename, data)` returns a `*ModuleFile`
  with statements + comments + a back-pointer to the underlying
  `*build.File` for advanced use cases
- **`types.go`** — `Statement` interface + concrete types for
  every recognized MODULE.bazel directive: `Module`, `BazelDep`,
  `UseExtension`, `UseRepo`, `SingleVersionOverride`,
  `MultipleVersionOverride`, `GitOverride`, `ArchiveOverride`,
  `LocalPathOverride`, `RegisterToolchains`,
  `RegisterExecutionPlatforms`
- **`handler.go`** — `Handler` interface + `Walk(file, handler)`
  that dispatches per-statement callbacks. Consumers implement
  one of these to project the parsed file into their own report
  shape
- **`bridge.go`** — connects the buildtools AST to the typed
  statements
- **`label/`** — typed `label.Module`, `label.Version`,
  `label.ApparentRepo`, `label.ApparentLabel`,
  `label.StarlarkIdentifier`. Used in the Handler signatures
- **`buildutil/`** — generic attribute-extraction helpers for
  buildtools AST nodes. Public because go-bzlmod (the resolver
  layer) also needs them. No buildtools-shape knowledge beyond
  "extract a string / int / list kwarg by name."
- **`third_party/buildtools/`** — vendored bazelbuild buildtools
  parser (Apache-2.0); the underlying parser this library wraps

## Why it exists

The library was internal to `go-bzlmod` until 2026-05-30 when it was
carved out as part of the three-lib refactor documented at
[`assay/docs/lib-carveout-plan.md`](https://github.com/albertocavalcante/assay/blob/main/docs/lib-carveout-plan.md):

- `go-bzlmod-ast` (this lib) — MODULE.bazel parse + AST + Handler
- `go-bzlmod` — MVS resolver + lockfile + registry; depends on this lib
- `assay/modulefile` — consumes Handler directly; no longer carries
  its own MODULE.bazel surface walk

## Usage

```go
import "github.com/albertocavalcante/go-bzlmod-ast"

content, _ := os.ReadFile("MODULE.bazel")
result, err := ast.ParseContent("MODULE.bazel", content)
if err != nil { /* handle */ }

// Two paths from here:

// 1. Walk the file with a Handler. Embed BaseHandler and override
//    only the statement methods you care about.
type depCollector struct {
    ast.BaseHandler
    Deps []string
}

func (c *depCollector) BazelDep(name label.Module, version label.Version,
    _ int, _ label.ApparentRepo, _ bool) error {
    c.Deps = append(c.Deps, name.String())
    return nil
}

c := &depCollector{}
_ = ast.Walk(result.File, c)

// 2. Iterate result.File.Statements directly and type-switch.
//    Useful when you need access to the full typed struct (e.g.
//    UseExtension.Tags slice or any other field the Handler
//    callback doesn't expose).
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
interface — read `stmt.Span().Start` for the start, `.End` for the
end. The Span is also reachable as the typed struct's `Pos` field.

## Status

Pre-tag. Pseudo-versioned (`v0.0.0-YYYYMMDDhhmmss-<sha>`) until
go-bzlmod + assay both consume it stably and the API surface
settles. Then tagged at `v0.1.0`.

## Layout decisions

- **Root package is `ast`** (unchanged from the original
  internal-package shape). Callers import as
  `import "github.com/albertocavalcante/go-bzlmod-ast"` and call
  `ast.Parse`, `ast.Walk`, etc. Go convention: package name
  ≠ import path is fine when the package name is contextually
  clear.
- **No vendor/ on this repo.** Single non-vendored runtime dep is
  go-starlark-syntaxutil (sibling); the bazelbuild buildtools
  parser is committed under `third_party/`.
- **third_party/ is excluded from lint + fmt + modernize.** It's
  upstream code we wrap, not modify.

## License

MIT for go-bzlmod-ast's own code. The vendored
`third_party/buildtools/` carries its own Apache-2.0 license
(`third_party/buildtools/LICENSE`).
