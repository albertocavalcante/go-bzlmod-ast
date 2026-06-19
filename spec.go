package ast

import (
	"strconv"
	"strings"
)

// AttrSpec describes a single attribute of a MODULE.bazel directive and
// its Bazel-version lifecycle metadata.
//
// # Why per-major slices
//
// Bazel ships parallel LTS lines. A deprecation can land in one branch
// before another: compatibility_level became a no-op in 8.6.0
// (2026-02-26) BEFORE the forward-port to 9.1.0 (2026-04-20), even
// though 9.0.0 was already released (2026-01-20). A flat "deprecated
// since X.Y.Z" doesn't model this -- a user running 9.0.0 was on a
// pre-deprecation build despite 8.6.0 being out.
//
// DeprecatedIn and NoopSince therefore record one entry per major LTS
// branch: the FIRST version on that branch where the attribute reached
// that lifecycle stage. Missing major = the branch never saw the
// transition.
//
// All version strings are Bazel semver in major.minor.patch form.
// AddedIn is a single string (introductions don't bifurcate across
// branches in the same way; we use the earliest in-scope release).
//
// Sentinels:
//
//   - AddedIn=="" -> "available since the first Bazel release in scope".
//   - nil/empty DeprecatedIn -> "not deprecated in any in-scope branch".
//   - nil/empty NoopSince    -> "still functional in every in-scope branch".
//
// # Scope
//
// As of June 2026 the in-scope Bazel releases are 7.x, 8.x, and 9.x.
// Bazel 5.x and 6.x are explicitly EOL per the upstream release model
// (https://bazel.build/release) and are NOT covered. AddedIn=""
// therefore means "available in 7.0.0 onward". Prune 7.x entries once
// it leaves Maintenance (scheduled Dec 2026).
//
// # Source of truth
//
// Per-attribute data comes from reading
// src/main/java/com/google/devtools/build/lib/bazel/bzlmod/
// ModuleFileGlobals.java at each relevant Bazel release tag. Refresh
// when a new Bazel minor or LTS lands.
type AttrSpec struct {
	Name         string
	Doc          string
	AddedIn      string
	DeprecatedIn []string
	NoopSince    []string
}

// ModuleAttrs returns the canonical attribute spec for module().
//
// Reference: ModuleFileGlobals.java @StarlarkMethod(name = "module").
// Lifecycle anchors:
//
//   - compatibility_level was a functional field from 7.0.0 through
//     8.5.x and 9.0.x. It was deprecated and made a no-op in 8.6.0
//     (2026-02-26) and forward-ported to 9.1.0 (2026-04-20).
func ModuleAttrs() []AttrSpec {
	return []AttrSpec{
		{Name: "name", Doc: "Module name. Required for non-root modules."},
		{Name: "version", Doc: "Module version. Required for non-root modules."},
		{
			Name:         "compatibility_level",
			Doc:          "Originally tracked breaking changes. Deprecated no-op since 8.6.0 / 9.1.0.",
			DeprecatedIn: []string{"8.6.0", "9.1.0"},
			NoopSince:    []string{"8.6.0", "9.1.0"},
		},
		{Name: "repo_name", Doc: "Override the repo name representing this module."},
		{Name: "bazel_compatibility", Doc: "Allowed Bazel versions (e.g. \">=7.0.0\"). Informational; does not affect resolution."},
	}
}

// BazelDepAttrs returns the canonical attribute spec for bazel_dep().
//
// Reference: ModuleFileGlobals.java @StarlarkMethod(name = "bazel_dep").
// Lifecycle anchors:
//
//   - max_compatibility_level paired with compatibility_level and shared
//     its lifecycle: functional through 8.5.x / 9.0.x, deprecated no-op
//     starting at 8.6.0 (2026-02-26), forward-ported to 9.1.0
//     (2026-04-20).
func BazelDepAttrs() []AttrSpec {
	return []AttrSpec{
		{Name: "name", Doc: "Module name to depend on. Required."},
		{Name: "version", Doc: "Minimum required version."},
		{
			Name:         "max_compatibility_level",
			Doc:          "Capped compatibility_level for the resolved version. Deprecated no-op since 8.6.0 / 9.1.0.",
			DeprecatedIn: []string{"8.6.0", "9.1.0"},
			NoopSince:    []string{"8.6.0", "9.1.0"},
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
// git_override(). NOTE: Bazel uses extraKeywords for git_override --
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
// archive_override -- all kwargs except module_name are forwarded to
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

// IsDeprecatedAt reports whether the attribute of the given directive
// is documented as deprecated for the supplied Bazel version. Resolution
// is per-major-branch: a 9.0.0 query matches the entry on the 9.x branch
// (if any), not whichever entry has the smallest version string. Returns
// false for unknown directive/attr and for attrs that have never been
// deprecated on the queried major branch.
//
// bazelVersion is in major.minor.patch form (e.g. "8.5.0"). Malformed
// input parses as (0,0,0), which is older than any real Bazel and so
// returns false unless the attribute was deprecated on the (nonexistent)
// 0.x branch.
func IsDeprecatedAt(directive, attr, bazelVersion string) bool {
	s, ok := LookupAttr(directive, attr)
	if !ok {
		return false
	}
	return reachedStageAt(s.DeprecatedIn, bazelVersion)
}

// IsNoopAt reports whether the attribute of the given directive is
// documented as a no-op for the supplied Bazel version. Same per-major
// semantics as IsDeprecatedAt.
func IsNoopAt(directive, attr, bazelVersion string) bool {
	s, ok := LookupAttr(directive, attr)
	if !ok {
		return false
	}
	return reachedStageAt(s.NoopSince, bazelVersion)
}

// IsDeprecatedAtHead is the policy-free query: "is this attribute
// deprecated in any in-scope release branch?". Useful for tools that
// don't pin a target Bazel version.
func IsDeprecatedAtHead(directive, attr string) bool {
	s, ok := LookupAttr(directive, attr)
	if !ok {
		return false
	}
	return len(s.DeprecatedIn) > 0
}

// IsNoopAtHead is the policy-free counterpart for no-op status.
func IsNoopAtHead(directive, attr string) bool {
	s, ok := LookupAttr(directive, attr)
	if !ok {
		return false
	}
	return len(s.NoopSince) > 0
}

// reachedStageAt is the shared per-major-branch comparator. It picks
// the entry in stage whose major matches bazelVersion's major and
// reports whether bazelVersion >= that entry. Missing entry = the
// queried branch never reached this stage -> false.
func reachedStageAt(stage []string, bazelVersion string) bool {
	target := parseSemver3(bazelVersion)
	for _, v := range stage {
		entry := parseSemver3(v)
		if entry[0] != target[0] {
			continue
		}
		return compareSemverParts(target, entry) >= 0
	}
	return false
}

// compareSemver returns -1, 0, or +1 ordering a against b under simple
// major.minor.patch semantics. Pre-release suffixes ("-rc1", "+build")
// are ignored, matching how Bazel tags releases. An unparsable component
// is treated as 0, so malformed input compares as the smallest version.
func compareSemver(a, b string) int {
	return compareSemverParts(parseSemver3(a), parseSemver3(b))
}

func compareSemverParts(a, b [3]int) int {
	for i := range 3 {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func parseSemver3(v string) [3]int {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := range 3 {
		if i >= len(parts) {
			break
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		out[i] = n
	}
	return out
}
