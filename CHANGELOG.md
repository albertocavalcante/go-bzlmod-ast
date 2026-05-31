# Changelog

All notable changes to go-bzlmod-ast are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Sections used per release (Keep-A-Changelog v1.1.0): Added, Changed,
Deprecated, Removed, Fixed, Security. Empty sections are omitted.

## [Unreleased]

This section accumulates changes until the first `v0.1.0` tag,
which lands when both go-bzlmod and assay consume it stably and the
API surface settles. Pseudo-versions
(`v0.0.0-YYYYMMDDhhmmss-<sha>`) carry development through then.

### Added

Initial carve-out from `go-bzlmod/ast/` plus the supporting
packages it depends on:

- `parser.go` — `Parse(filename, data)` returning a `*ModuleFile`
  with statements + comments + back-pointer to the buildtools `*build.File`
- `types.go` — `Statement` interface + concrete types for every
  recognized MODULE.bazel directive (Module, BazelDep, UseExtension,
  UseRepo, all 5 *Override variants, RegisterToolchains,
  RegisterExecutionPlatforms)
- `handler.go` — `Handler` interface + `Walk` dispatcher
- `bridge.go` — connects the buildtools AST to the typed statements
- `label/` — typed Module / Version / ApparentRepo / ApparentLabel /
  StarlarkIdentifier
- `buildutil/` — public buildtools-attribute extraction helpers
  shared between this lib and go-bzlmod's resolver layer
- `third_party/buildtools/` — vendored bazelbuild buildtools parser
  (Apache-2.0)

Origin: [`assay/docs/lib-carveout-plan.md`](https://github.com/albertocavalcante/assay/blob/main/docs/lib-carveout-plan.md)
covering the three-lib carve-out
(go-starlark-syntaxutil + go-bzlmod-ast + downstream refactor).

### Changed (against previous internal-package shape)

- Module path is now `github.com/albertocavalcante/go-bzlmod-ast`
  (was `github.com/albertocavalcante/go-bzlmod/ast`). Package name
  stays `ast`. Callers import the module path and call
  `ast.Parse`, `ast.Walk`, etc.
- **BREAKING — Span replaces flat Position on every typed statement.**
  Statement.Position() method renamed to Statement.Span() and
  returns a new `Span` struct (`{Start, End Position}`) instead
  of a single Position. The Pos field on every typed statement
  changes type from `Position` to `Span` (field name preserved
  — read `.Pos.Start` / `.Pos.End`). Handler.UnknownStatement's
  second param changes from `Position` to `Span` for consistency.
  See 0B-rev1.
- **BREAKING — Handler.UseExtension absorbs tags; ExtensionProxy
  removed.** Signature changes from
  `(file, name, devDep, isolate) (ExtensionProxy, error)` to
  `(variable, file, name, devDep, isolate, tags) error`. The
  variable field of the LHS assignment (`python` in
  `python = use_extension(...)`) is exposed as the first
  parameter so consumers can link subsequent UseRepo / tag
  calls without a parallel map. Tags arrive as a `[]ExtensionTag`
  slice in source order. The `ExtensionProxy` interface and its
  `Tag(name, attrs)` method are deleted. See 0B-rev2.
- **BREAKING — Handler.UseRepo gains extension link + renames.**
  Signature changes from `(repos, devDependency)` to
  `(extensionVariable, repos, renames, devDependency)`. The
  `UseRepo` struct's `Extension *UseExtension` field (which was
  never populated by the parser) is replaced by
  `ExtensionVariable string` (populated from the first
  positional ident in use_repo). Named-kwarg aliases
  (`use_repo(ext, my_alias = "remote_repo")`) are now captured
  into `UseRepo.Renames map[string]string`; previously dropped.
  See 0B-rev3.
- **BREAKING — Handler.Include added; Walk now dispatches
  *Include statements.** Pre-0B-rev4, include() statements
  parsed into typed `*Include` but `walkStatement` had no case
  for them — they were silently skipped. The new method on
  Handler is `Include(labelStr string, pos Span) error`.
  BaseHandler provides a no-op default; any Handler implementor
  written before this change won't compile against the new
  interface (one new method to add). See 0B-rev4.
- Import paths rewritten:
  - `github.com/albertocavalcante/go-bzlmod/label` → `github.com/albertocavalcante/go-bzlmod-ast/label`
  - `github.com/albertocavalcante/go-bzlmod/internal/buildutil` → `github.com/albertocavalcante/go-bzlmod-ast/buildutil` (promoted out of internal/ so go-bzlmod can import it externally)
  - `github.com/albertocavalcante/go-bzlmod/third_party/buildtools/build` → `github.com/albertocavalcante/go-bzlmod-ast/third_party/buildtools/build`
