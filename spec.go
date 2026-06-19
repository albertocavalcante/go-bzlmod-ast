package ast

// AttrSpec describes a single attribute of a MODULE.bazel directive
// and the Bazel-version lifecycle metadata we know about it.
//
// All version strings are Bazel semver (e.g. "7.0.0"). Sentinel values:
//
//   - "" in AddedIn means "available since bzlmod was introduced".
//   - "" in DeprecatedIn means "not documented as deprecated".
//   - "" in NoopSince means "still functional in Bazel HEAD".
//   - "HEAD" in DeprecatedIn / NoopSince means "the field IS deprecated /
//     a no-op as of Bazel HEAD, but the exact version when this changed
//     hasn't been backfilled yet." Callers should treat HEAD as "true at
//     all observed versions".
//
// Source of truth: src/main/java/com/google/devtools/build/lib/bazel/bzlmod/
// ModuleFileGlobals.java in bazelbuild/bazel. Refresh this table when a
// new Bazel release changes a directive surface.
type AttrSpec struct {
	Name         string
	Doc          string
	AddedIn      string
	DeprecatedIn string
	NoopSince    string
}

// ModuleAttrs returns the canonical attribute spec for module().
// Reference: ModuleFileGlobals.java @StarlarkMethod(name = "module").
func ModuleAttrs() []AttrSpec {
	return []AttrSpec{
		{Name: "name", Doc: "Module name. Required for non-root modules."},
		{Name: "version", Doc: "Module version. Required for non-root modules."},
		{
			Name:         "compatibility_level",
			Doc:          "Originally meant for breaking-change tracking. Deprecated no-op at HEAD.",
			DeprecatedIn: "HEAD",
			NoopSince:    "HEAD",
		},
		{Name: "repo_name", Doc: "Override the repo name representing this module."},
		{Name: "bazel_compatibility", Doc: "Allowed Bazel versions, e.g. '>=7.0.0'. Informational, does not affect resolution."},
	}
}

// BazelDepAttrs returns the canonical attribute spec for bazel_dep().
// Reference: ModuleFileGlobals.java @StarlarkMethod(name = "bazel_dep").
func BazelDepAttrs() []AttrSpec {
	return []AttrSpec{
		{Name: "name", Doc: "Module name to depend on. Required."},
		{Name: "version", Doc: "Minimum required version."},
		{
			Name:         "max_compatibility_level",
			Doc:          "Used to cap compatibility_level. Deprecated no-op at HEAD now that compatibility_level itself is a no-op.",
			DeprecatedIn: "HEAD",
			NoopSince:    "HEAD",
		},
		{Name: "repo_name", Doc: "Apparent repo name to expose this dep under. Pass repo_name=None to mark as a nodep dependency."},
		{Name: "dev_dependency", Doc: "True if only needed in dev/test contexts."},
	}
}

// SingleVersionOverrideAttrs returns the canonical attribute spec for
// single_version_override(). Reference: ModuleFileGlobals.java
// @StarlarkMethod(name = "single_version_override").
func SingleVersionOverrideAttrs() []AttrSpec {
	return []AttrSpec{
		{Name: "module_name", Doc: "Module being overridden. Required."},
		{Name: "version", Doc: "Pinned version (optional; omit if only overriding registry or patches)."},
		{Name: "registry", Doc: "Override the registry URL for this module."},
		{Name: "patches", Doc: "List of label-string patches to apply."},
		{Name: "patch_cmds", Doc: "Shell commands run after patching (Linux/macOS)."},
		{Name: "patch_strip", Doc: "patch tool -p argument."},
	}
}

// MultipleVersionOverrideAttrs returns the canonical attribute spec for
// multiple_version_override(). Reference: ModuleFileGlobals.java
// @StarlarkMethod(name = "multiple_version_override").
func MultipleVersionOverrideAttrs() []AttrSpec {
	return []AttrSpec{
		{Name: "module_name", Doc: "Module being overridden. Required."},
		{Name: "versions", Doc: "Sorted list of allowed versions."},
		{Name: "registry", Doc: "Override the registry URL for this module."},
	}
}

// GitOverrideAttrs returns the commonly-used attribute spec for
// git_override(). NOTE: Bazel uses extraKeywords for git_override —
// all kwargs except module_name are forwarded to the underlying
// git_repository repo rule. The set listed here covers the typed
// fields on ast.GitOverride; anything else lands in ExtraKwargs.
// Reference: ModuleFileGlobals.java @StarlarkMethod(name = "git_override").
func GitOverrideAttrs() []AttrSpec {
	return []AttrSpec{
		{Name: "module_name", Doc: "Module being overridden. Required."},
		{Name: "remote", Doc: "Git repository URL."},
		{Name: "commit", Doc: "Commit hash to check out."},
		{Name: "tag", Doc: "Tag to check out."},
		{Name: "branch", Doc: "Branch to check out."},
		{Name: "patches", Doc: "List of label-string patches to apply."},
		{Name: "patch_cmds", Doc: "Shell commands run after patching (Linux/macOS)."},
		{Name: "patch_strip", Doc: "patch tool -p argument."},
		{Name: "init_submodules", Doc: "If true, initialize git submodules after checkout."},
		{Name: "strip_prefix", Doc: "Directory prefix to strip after checkout."},
		{Name: "verbose", Doc: "Verbose patch output."},
	}
}

// ArchiveOverrideAttrs returns the commonly-used attribute spec for
// archive_override(). NOTE: Bazel uses extraKeywords for
// archive_override — all kwargs except module_name are forwarded to
// the underlying http_archive repo rule. The set listed here covers
// the typed fields on ast.ArchiveOverride; anything else lands in
// ExtraKwargs. Reference: ModuleFileGlobals.java @StarlarkMethod(name
// = "archive_override").
func ArchiveOverrideAttrs() []AttrSpec {
	return []AttrSpec{
		{Name: "module_name", Doc: "Module being overridden. Required."},
		{Name: "urls", Doc: "Download URLs to try in order."},
		{Name: "integrity", Doc: "Subresource Integrity hash."},
		{Name: "strip_prefix", Doc: "Directory prefix to strip after extraction."},
		{Name: "patches", Doc: "List of label-string patches to apply."},
		{Name: "patch_cmds", Doc: "Shell commands run after patching (Linux/macOS)."},
		{Name: "patch_strip", Doc: "patch tool -p argument."},
	}
}

// LocalPathOverrideAttrs returns the canonical attribute spec for
// local_path_override(). Reference: ModuleFileGlobals.java
// @StarlarkMethod(name = "local_path_override").
func LocalPathOverrideAttrs() []AttrSpec {
	return []AttrSpec{
		{Name: "module_name", Doc: "Module being overridden. Required."},
		{Name: "path", Doc: "Local filesystem path to the module source. Required."},
	}
}

// directiveAttrs maps a directive name (as it appears in MODULE.bazel
// source) to its attribute spec function.
var directiveAttrs = map[string]func() []AttrSpec{
	"module":                    ModuleAttrs,
	"bazel_dep":                 BazelDepAttrs,
	"single_version_override":   SingleVersionOverrideAttrs,
	"multiple_version_override": MultipleVersionOverrideAttrs,
	"git_override":              GitOverrideAttrs,
	"archive_override":          ArchiveOverrideAttrs,
	"local_path_override":       LocalPathOverrideAttrs,
}

// LookupAttr returns the AttrSpec for an attribute of a directive,
// or (zero, false) when the directive or attribute is unknown.
func LookupAttr(directive, attr string) (AttrSpec, bool) {
	fn, ok := directiveAttrs[directive]
	if !ok {
		return AttrSpec{}, false
	}
	for _, s := range fn() {
		if s.Name == attr {
			return s, true
		}
	}
	return AttrSpec{}, false
}

// IsDeprecatedAtHead reports whether the attribute of the given
// directive is documented as deprecated in Bazel HEAD. This is the
// safest query when the caller does not know its target Bazel version
// or only cares about current-state advice.
func IsDeprecatedAtHead(directive, attr string) bool {
	s, ok := LookupAttr(directive, attr)
	if !ok {
		return false
	}
	return s.DeprecatedIn != ""
}

// IsNoopAtHead reports whether the attribute of the given directive
// is documented as a no-op in Bazel HEAD.
func IsNoopAtHead(directive, attr string) bool {
	s, ok := LookupAttr(directive, attr)
	if !ok {
		return false
	}
	return s.NoopSince != ""
}
