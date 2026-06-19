package ast

// Handler processes MODULE.bazel statements.
//
// Each method receives a pointer to the typed AST node for the
// directive that fired. Implementations should treat the pointer
// as read-only; ast does not enforce immutability but downstream
// code may share the same node across multiple Walk passes.
//
// Returning a non-nil error from any method stops the Walk and
// surfaces the error verbatim to the caller.
//
// Adding fields to the AST node types is non-breaking for Handler
// implementations — only direct dependents of the new field need
// updating.
type Handler interface {
	Module(*ModuleDecl) error
	BazelDep(*BazelDep) error
	UseExtension(*UseExtension) error
	UseRepo(*UseRepo) error
	SingleVersionOverride(*SingleVersionOverride) error
	MultipleVersionOverride(*MultipleVersionOverride) error
	GitOverride(*GitOverride) error
	ArchiveOverride(*ArchiveOverride) error
	LocalPathOverride(*LocalPathOverride) error
	RegisterToolchains(*RegisterToolchains) error
	RegisterExecutionPlatforms(*RegisterExecutionPlatforms) error
	Include(*Include) error
	UnknownStatement(*UnknownStatement) error
}

// Walk traverses a ModuleFile and calls the handler for each statement.
func Walk(file *ModuleFile, handler Handler) error {
	for _, stmt := range file.Statements {
		if err := walkStatement(stmt, handler); err != nil {
			return err
		}
	}
	return nil
}

func walkStatement(stmt Statement, handler Handler) error {
	switch s := stmt.(type) {
	case *ModuleDecl:
		return handler.Module(s)
	case *BazelDep:
		return handler.BazelDep(s)
	case *UseExtension:
		return handler.UseExtension(s)
	case *UseRepo:
		return handler.UseRepo(s)
	case *SingleVersionOverride:
		return handler.SingleVersionOverride(s)
	case *MultipleVersionOverride:
		return handler.MultipleVersionOverride(s)
	case *GitOverride:
		return handler.GitOverride(s)
	case *ArchiveOverride:
		return handler.ArchiveOverride(s)
	case *LocalPathOverride:
		return handler.LocalPathOverride(s)
	case *RegisterToolchains:
		return handler.RegisterToolchains(s)
	case *RegisterExecutionPlatforms:
		return handler.RegisterExecutionPlatforms(s)
	case *Include:
		return handler.Include(s)
	case *UnknownStatement:
		return handler.UnknownStatement(s)
	}
	return nil
}

// BaseHandler provides default no-op implementations of all Handler methods.
// Embed this in your handler to only implement the methods you need.
//
// Example:
//
//	type MyHandler struct {
//	    ast.BaseHandler
//	    deps []string
//	}
//
//	func (h *MyHandler) BazelDep(d *ast.BazelDep) error {
//	    h.deps = append(h.deps, d.Name.String())
//	    return nil
//	}
type BaseHandler struct{}

func (h *BaseHandler) Module(*ModuleDecl) error                                 { return nil }
func (h *BaseHandler) BazelDep(*BazelDep) error                                 { return nil }
func (h *BaseHandler) UseExtension(*UseExtension) error                         { return nil }
func (h *BaseHandler) UseRepo(*UseRepo) error                                   { return nil }
func (h *BaseHandler) SingleVersionOverride(*SingleVersionOverride) error       { return nil }
func (h *BaseHandler) MultipleVersionOverride(*MultipleVersionOverride) error   { return nil }
func (h *BaseHandler) GitOverride(*GitOverride) error                           { return nil }
func (h *BaseHandler) ArchiveOverride(*ArchiveOverride) error                   { return nil }
func (h *BaseHandler) LocalPathOverride(*LocalPathOverride) error               { return nil }
func (h *BaseHandler) RegisterToolchains(*RegisterToolchains) error             { return nil }
func (h *BaseHandler) RegisterExecutionPlatforms(*RegisterExecutionPlatforms) error {
	return nil
}
func (h *BaseHandler) Include(*Include) error                   { return nil }
func (h *BaseHandler) UnknownStatement(*UnknownStatement) error { return nil }

// DependencyCollector is a handler that collects pointers to every
// bazel_dep declaration. The pointers reference nodes owned by the
// parsed ModuleFile; do not mutate them.
type DependencyCollector struct {
	BaseHandler
	Dependencies []*BazelDep
}

func (c *DependencyCollector) BazelDep(d *BazelDep) error {
	c.Dependencies = append(c.Dependencies, d)
	return nil
}

// OverrideCollector is a handler that collects pointers to every
// override declaration, sorted by override kind. The pointers
// reference nodes owned by the parsed ModuleFile; do not mutate them.
type OverrideCollector struct {
	BaseHandler
	SingleVersionOverrides   []*SingleVersionOverride
	MultipleVersionOverrides []*MultipleVersionOverride
	GitOverrides             []*GitOverride
	ArchiveOverrides         []*ArchiveOverride
	LocalPathOverrides       []*LocalPathOverride
}

func (c *OverrideCollector) SingleVersionOverride(o *SingleVersionOverride) error {
	c.SingleVersionOverrides = append(c.SingleVersionOverrides, o)
	return nil
}

func (c *OverrideCollector) MultipleVersionOverride(o *MultipleVersionOverride) error {
	c.MultipleVersionOverrides = append(c.MultipleVersionOverrides, o)
	return nil
}

func (c *OverrideCollector) GitOverride(o *GitOverride) error {
	c.GitOverrides = append(c.GitOverrides, o)
	return nil
}

func (c *OverrideCollector) ArchiveOverride(o *ArchiveOverride) error {
	c.ArchiveOverrides = append(c.ArchiveOverrides, o)
	return nil
}

func (c *OverrideCollector) LocalPathOverride(o *LocalPathOverride) error {
	c.LocalPathOverrides = append(c.LocalPathOverrides, o)
	return nil
}
