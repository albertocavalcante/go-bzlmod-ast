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
parsed, err := ast.Parse("MODULE.bazel", content)
if err != nil { /* handle */ }

// Walk with a custom handler:
type myHandler struct { /* per-project state */ }

func (h *myHandler) Module(name label.Module, version label.Version,
    compatibilityLevel int, repoName label.ApparentRepo) error { /* ... */ }
// ... implement the other Handler methods ...

if err := ast.Walk(parsed, &myHandler{}); err != nil { /* ... */ }
```

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
