module github.com/albertocavalcante/go-bzlmod-ast

go 1.26.0

// Sibling checkout; relative path so the workspace tree is portable.
// Drop once go-starlark-syntaxutil ships a tagged version.
replace github.com/albertocavalcante/go-starlark-syntaxutil => ../go-starlark-syntaxutil
