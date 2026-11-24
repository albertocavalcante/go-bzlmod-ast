package ast

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
)

// IncludeSuffix is the file name suffix Bazel requires of included files.
const IncludeSuffix = ".MODULE.bazel"

// Errors reported by [ExpandIncludes]. Apart from context errors and a
// root syntax error, every returned error is an *[IncludeError] that
// matches exactly one of these with [errors.Is].
var (
	ErrInvalidIncludeLabel       = errors.New("invalid include label")
	ErrIncludeNotAllowed         = errors.New("include() not allowed in this module")
	ErrIncludeNotFound           = errors.New("included file not found")
	ErrIncludeUnreadable         = errors.New("included file unreadable")
	ErrIncludeCycle              = errors.New("include() cycle")
	ErrIncludeForbiddenStatement = errors.New("statement not allowed in an included file")
	ErrIncludeLimit              = errors.New("include limit exceeded")
)

// ModuleRole tells [ExpandIncludes] which module the file belongs to;
// Bazel only lets some modules use include(). include(dev_dependency=)
// exists from Bazel 9.3.0; older releases reject it as a syntax error.
type ModuleRole int

const (
	RootModule     ModuleRole = iota // may include; dev includes honored unless IgnoreDevDependency
	OverrideModule                   // non-root with a non-registry override (Bazel 8.5+): may include; dev includes skipped
	RegistryModule                   // from a registry: non-dev include is ErrIncludeNotAllowed; dev includes skipped
)

// Default limits used when the corresponding [IncludeOptions] field is <= 0.
const (
	DefaultIncludeMaxDepth   = 16
	DefaultIncludeMaxFiles   = 256
	DefaultIncludeFileBytes  = 1 << 20
	DefaultIncludeTotalBytes = 16 << 20
	DefaultIncludeStatements = 100_000
)

// IncludeOptions configures [ExpandIncludes]. The zero value expands a
// root module with the default limits.
type IncludeOptions struct {
	Role                ModuleRole
	IgnoreDevDependency bool  // skip dev_dependency includes, like --ignore_dev_dependency
	MaxDepth            int   // longest include chain
	MaxFiles            int   // include() expansions; a file spliced twice counts twice
	MaxFileBytes        int64 // size of each file read
	MaxTotalBytes       int64 // bytes read across all files
	MaxStatements       int   // statements in the expanded list; bounds re-splicing, not parse cost
}

// IncludeSite is one include() statement on an include chain.
type IncludeSite struct {
	Label string // label as written
	Path  string // resolved slash-separated path relative to the module root
	Pos   Span   // the include() call in the including file
}

// ExpandedStatement is a statement with its provenance. File owns the
// statement (edit it there); its Path, also the statement's Span
// filename, is the root's name or the resolved include path. Chain
// lists the include() sites that brought it in, outermost first; it is
// empty for root statements and shared, so do not modify it.
type ExpandedStatement struct {
	Statement
	File  *ModuleFile
	Chain []IncludeSite
}

// ExpandedFile is a module file with every include() spliced in place.
type ExpandedFile struct {
	// Statements is in Bazel evaluation order. Each *Include is kept
	// as a marker immediately followed by the included statements; a
	// skipped dev_dependency include has none. A file included twice
	// is spliced twice and its statements appear twice.
	Statements []ExpandedStatement
	// Files lists each distinct file once, root first, in first-visit order.
	Files []*ModuleFile
	// Errors and Warnings collect parse diagnostics of all Files.
	Errors   []*ParseError
	Warnings []*ParseError
}

// IncludeError describes a failed expansion. Pos is the offending
// include() call, or the forbidden statement.
type IncludeError struct {
	Kind   error // one of the Err* sentinels
	Pos    Position
	Label  string
	Path   string        // resolved path, when known
	Chain  []IncludeSite // active include chain, outermost first
	Detail string
	Err    error // underlying cause, if any
}

func (e *IncludeError) Error() string {
	var b strings.Builder
	if e.Pos.Line > 0 {
		fmt.Fprintf(&b, "%s:%d:%d: %v", e.Pos.Filename, e.Pos.Line, e.Pos.Column, e.Kind)
	} else {
		fmt.Fprintf(&b, "%s: %v", e.Pos.Filename, e.Kind)
	}
	if e.Label != "" {
		fmt.Fprintf(&b, ": %q", e.Label)
	}
	if e.Path != "" {
		fmt.Fprintf(&b, " (%s)", e.Path)
	}
	if e.Detail != "" {
		b.WriteString(": " + e.Detail)
	}
	if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	return b.String()
}

// Is matches the error's Kind sentinel.
func (e *IncludeError) Is(target error) bool { return target == e.Kind }

// Unwrap returns the underlying cause, such as an fs or *ParseError.
func (e *IncludeError) Unwrap() error { return e.Err }

// IncludePath resolves an include() label to a slash-separated path
// relative to the module root, valid for [fs.FS]. It accepts only
// repo-relative labels ("//pkg:name" or "//pkg") whose file name ends
// in [IncludeSuffix] and does not start with ".", and rejects paths
// that leave the root lexically. Symlinks are the [fs.FS]'s concern:
// use os.OpenRoot(dir).FS() rather than os.DirFS for untrusted trees.
// Unlike Bazel, it does not require a BUILD file in the package.
func IncludePath(label string) (string, error) {
	rest, ok := strings.CutPrefix(label, "//")
	if !ok {
		return "", errors.New(`label must start with "//"`)
	}
	if strings.IndexFunc(rest, func(r rune) bool { return r < 0x20 || r == 0x7f || r == '\\' }) >= 0 {
		return "", errors.New("label must not contain backslashes or control characters")
	}
	pkg, name, hasName := strings.Cut(rest, ":")
	if !hasName {
		name = pkg[strings.LastIndex(pkg, "/")+1:]
	}
	if strings.Contains(name, ":") {
		return "", errors.New(`label has more than one ":"`)
	}
	if pkg != "" && (!cleanSegments(pkg) || strings.IndexFunc(pkg, func(r rune) bool { return r > 0x7e }) >= 0) {
		return "", fmt.Errorf("invalid package %q", pkg)
	}
	if !cleanSegments(name) {
		return "", fmt.Errorf("invalid target name %q", name)
	}
	base := name[strings.LastIndex(name, "/")+1:]
	if !strings.HasSuffix(base, IncludeSuffix) || strings.HasPrefix(base, ".") {
		return "", fmt.Errorf("file name %q must end in %q and not start with \".\"", base, IncludeSuffix)
	}
	p := name
	if pkg != "" {
		p = pkg + "/" + name
	}
	if !fs.ValidPath(p) {
		return "", fmt.Errorf("invalid path %q", p)
	}
	return p, nil
}

// cleanSegments reports whether s is a non-empty "/"-separated path of
// non-empty segments other than "." and "..".
func cleanSegments(s string) bool {
	for seg := range strings.SplitSeq(s, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// ParseWithIncludes reads MODULE.bazel at the root of fsys, parses it
// and expands its includes. fsys is typically os.DirFS(workspaceRoot).
func ParseWithIncludes(ctx context.Context, fsys fs.FS, opts IncludeOptions) (*ExpandedFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	x := newExpander(ctx, fsys, opts)
	data, err := x.read("MODULE.bazel")
	if err != nil {
		return nil, err
	}
	root, err := ParseContent("MODULE.bazel", data)
	if err != nil {
		return nil, err
	}
	return x.run(root)
}

// ExpandIncludes splices every include() of root into a flat statement
// list, reading included files from fsys, which must be rooted at the
// module root. It performs no other IO, keeps no state, and returns the
// same result or error for the same inputs. root is not modified; ctx
// and root must be non-nil. Cycles are detected by resolved path, so two
// spellings of one file's label are the same file. A syntax error in an
// included file is ErrIncludeUnreadable wrapping the *[ParseError].
func ExpandIncludes(ctx context.Context, root *ParseResult, fsys fs.FS, opts IncludeOptions) (*ExpandedFile, error) {
	return newExpander(ctx, fsys, opts).run(root)
}

type expander struct {
	ctx    context.Context
	fsys   fs.FS
	opts   IncludeOptions
	out    *ExpandedFile
	parsed map[string]*ModuleFile
	files  int
	bytes  int64
}

func newExpander(ctx context.Context, fsys fs.FS, opts IncludeOptions) *expander {
	opts.MaxDepth = orDefault(opts.MaxDepth, DefaultIncludeMaxDepth)
	opts.MaxFiles = orDefault(opts.MaxFiles, DefaultIncludeMaxFiles)
	opts.MaxFileBytes = orDefault(opts.MaxFileBytes, DefaultIncludeFileBytes)
	opts.MaxTotalBytes = orDefault(opts.MaxTotalBytes, DefaultIncludeTotalBytes)
	opts.MaxStatements = orDefault(opts.MaxStatements, DefaultIncludeStatements)
	return &expander{ctx: ctx, fsys: fsys, opts: opts, out: &ExpandedFile{}, parsed: map[string]*ModuleFile{}}
}

func orDefault[T int | int64](v, def T) T {
	if v <= 0 {
		return def
	}
	return v
}

func (x *expander) run(root *ParseResult) (*ExpandedFile, error) {
	x.out.Files = append(x.out.Files, root.File)
	x.out.Errors = append(x.out.Errors, root.Errors...)
	x.out.Warnings = append(x.out.Warnings, root.Warnings...)
	if err := x.expand(root.File, nil); err != nil {
		return nil, err
	}
	return x.out, nil
}

func (x *expander) expand(f *ModuleFile, chain []IncludeSite) error {
	for _, s := range f.Statements {
		if len(x.out.Statements) >= x.opts.MaxStatements {
			return &IncludeError{Kind: ErrIncludeLimit, Pos: s.Span().Start, Chain: chain, Detail: fmt.Sprintf("more than %d expanded statements", x.opts.MaxStatements)}
		}
		x.out.Statements = append(x.out.Statements, ExpandedStatement{Statement: s, File: f, Chain: chain})
		inc, ok := s.(*Include)
		if !ok {
			continue
		}
		fail := func(kind error, path, detail string, err error) error {
			return &IncludeError{Kind: kind, Pos: inc.Pos.Start, Label: inc.Label, Path: path, Chain: chain, Detail: detail, Err: err}
		}
		if inc.malformed {
			return fail(ErrInvalidIncludeLabel, "", "include() takes one positional string literal and an optional literal dev_dependency", nil)
		}
		if inc.DevDependency && (x.opts.IgnoreDevDependency || x.opts.Role != RootModule) {
			continue
		}
		if x.opts.Role == RegistryModule {
			return fail(ErrIncludeNotAllowed, "", "only the root module and modules with non-registry overrides may use include()", nil)
		}
		path, err := IncludePath(inc.Label)
		if err != nil {
			return fail(ErrInvalidIncludeLabel, "", "", err)
		}
		site := IncludeSite{Label: inc.Label, Path: path, Pos: inc.Pos}
		if i := slices.IndexFunc(chain, func(c IncludeSite) bool { return c.Path == path }); i >= 0 {
			labels := make([]string, 0, len(chain)-i+1)
			for _, c := range chain[i:] {
				labels = append(labels, c.Label)
			}
			return fail(ErrIncludeCycle, path, strings.Join(append(labels, inc.Label), " -> "), nil)
		}
		if len(chain) >= x.opts.MaxDepth {
			return fail(ErrIncludeLimit, path, fmt.Sprintf("include depth exceeds %d", x.opts.MaxDepth), nil)
		}
		if x.files >= x.opts.MaxFiles {
			return fail(ErrIncludeLimit, path, fmt.Sprintf("more than %d included files", x.opts.MaxFiles), nil)
		}
		if cerr := x.ctx.Err(); cerr != nil {
			return cerr
		}
		x.files++
		child, err := x.load(path)
		if err != nil {
			var ie *IncludeError
			if errors.As(err, &ie) {
				ie.Pos, ie.Label, ie.Chain = inc.Pos.Start, inc.Label, chain
				return ie
			}
			return fail(ErrIncludeUnreadable, path, "", err)
		}
		next := append(slices.Clip(chain), site)
		for _, cs := range child.Statements {
			if m, ok := cs.(*ModuleDecl); ok {
				return &IncludeError{Kind: ErrIncludeForbiddenStatement, Pos: m.Pos.Start, Label: inc.Label, Path: path, Chain: next, Detail: "module() must precede every other call, include() included, so it cannot appear in an included file"}
			}
		}
		if err := x.expand(child, next); err != nil {
			return err
		}
	}
	return nil
}

// load parses path once; repeated includes reuse the parsed file.
func (x *expander) load(path string) (*ModuleFile, error) {
	if f, ok := x.parsed[path]; ok {
		return f, nil
	}
	data, err := x.read(path)
	if err != nil {
		return nil, err
	}
	res, err := ParseContent(path, data)
	if err != nil {
		return nil, err
	}
	x.parsed[path] = res.File
	x.out.Files = append(x.out.Files, res.File)
	x.out.Errors = append(x.out.Errors, res.Errors...)
	x.out.Warnings = append(x.out.Warnings, res.Warnings...)
	return res.File, nil
}

func (x *expander) read(path string) ([]byte, error) {
	fail := func(kind error, detail string, err error) error {
		return &IncludeError{Kind: kind, Pos: Position{Filename: path}, Path: path, Detail: detail, Err: err}
	}
	// Stat before Open so a FIFO or device is never opened (Open on one
	// can block). A file swapped between the two calls is not detected.
	st, err := fs.Stat(x.fsys, path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, fail(ErrIncludeNotFound, "", err)
	case err != nil:
		return nil, fail(ErrIncludeUnreadable, "", err)
	case !st.Mode().IsRegular():
		return nil, fail(ErrIncludeNotFound, "not a regular file", nil)
	}
	f, err := x.fsys.Open(path)
	if err != nil {
		return nil, fail(ErrIncludeUnreadable, "", err)
	}
	defer func() { _ = f.Close() }()
	limit := min(x.opts.MaxFileBytes, x.opts.MaxTotalBytes-x.bytes)
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fail(ErrIncludeUnreadable, "", err)
	}
	if int64(len(data)) > limit {
		return nil, fail(ErrIncludeLimit, fmt.Sprintf("file exceeds %d bytes or total exceeds %d bytes", x.opts.MaxFileBytes, x.opts.MaxTotalBytes), nil)
	}
	x.bytes += int64(len(data))
	return data, nil
}
