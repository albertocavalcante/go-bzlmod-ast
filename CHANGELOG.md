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
- Import paths rewritten:
  - `github.com/albertocavalcante/go-bzlmod/label` → `github.com/albertocavalcante/go-bzlmod-ast/label`
  - `github.com/albertocavalcante/go-bzlmod/internal/buildutil` → `github.com/albertocavalcante/go-bzlmod-ast/buildutil` (promoted out of internal/ so go-bzlmod can import it externally)
  - `github.com/albertocavalcante/go-bzlmod/third_party/buildtools/build` → `github.com/albertocavalcante/go-bzlmod-ast/third_party/buildtools/build`
