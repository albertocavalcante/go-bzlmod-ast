package ast

import "testing"

func TestLookupAttr_Known(t *testing.T) {
	s, ok := LookupAttr("bazel_dep", "max_compatibility_level")
	if !ok {
		t.Fatal("LookupAttr(bazel_dep, max_compatibility_level) returned not-ok")
	}
	if len(s.DeprecatedIn) == 0 {
		t.Errorf("max_compatibility_level should be marked deprecated; got empty DeprecatedIn")
	}
	if len(s.NoopSince) == 0 {
		t.Errorf("max_compatibility_level should be marked noop; got empty NoopSince")
	}
}

func TestLookupAttr_Unknown(t *testing.T) {
	if _, ok := LookupAttr("unknown_directive", "name"); ok {
		t.Error("expected unknown directive to return not-ok")
	}
	if _, ok := LookupAttr("bazel_dep", "unknown_attr"); ok {
		t.Error("expected unknown attr to return not-ok")
	}
}

func TestIsDeprecatedAtHead(t *testing.T) {
	if !IsDeprecatedAtHead("module", "compatibility_level") {
		t.Error("module.compatibility_level should be deprecated at HEAD")
	}
	if !IsDeprecatedAtHead("bazel_dep", "max_compatibility_level") {
		t.Error("bazel_dep.max_compatibility_level should be deprecated at HEAD")
	}
	if IsDeprecatedAtHead("bazel_dep", "name") {
		t.Error("bazel_dep.name should not be deprecated")
	}
	if IsDeprecatedAtHead("unknown", "anything") {
		t.Error("unknown directive should return false")
	}
}

func TestIsNoopAtHead(t *testing.T) {
	if !IsNoopAtHead("module", "compatibility_level") {
		t.Error("module.compatibility_level should be noop at HEAD")
	}
	if IsNoopAtHead("bazel_dep", "version") {
		t.Error("bazel_dep.version should not be noop")
	}
}

// TestIsDeprecatedAt_VersionBoundaries pins the per-version semantics:
// compatibility_level / max_compatibility_level deprecation FIRST
// shipped in 8.6.0 (2026-02-26) and was forward-ported to 9.1.0
// (2026-04-20). Bazel 7.x never saw the deprecation. 8.5.x and 9.0.x
// were still pre-deprecation.
func TestIsDeprecatedAt_VersionBoundaries(t *testing.T) {
	cases := []struct {
		bazel string
		want  bool
	}{
		{"7.0.0", false},
		{"7.7.1", false}, // last 7.x release; never deprecated
		{"8.0.0", false},
		{"8.5.1", false}, // pre-deprecation 8.x
		{"8.6.0", true},  // first version where it ships
		{"8.7.0", true},
		{"9.0.0", false}, // 9.0.x predates the forward-port
		{"9.0.2", false},
		{"9.1.0", true}, // forward-port lands here
		{"9.1.1", true},
	}
	for _, tc := range cases {
		if got := IsDeprecatedAt("module", "compatibility_level", tc.bazel); got != tc.want {
			t.Errorf("IsDeprecatedAt(module.compatibility_level, %q) = %v, want %v", tc.bazel, got, tc.want)
		}
		if got := IsDeprecatedAt("bazel_dep", "max_compatibility_level", tc.bazel); got != tc.want {
			t.Errorf("IsDeprecatedAt(bazel_dep.max_compatibility_level, %q) = %v, want %v", tc.bazel, got, tc.want)
		}
		if got := IsNoopAt("module", "compatibility_level", tc.bazel); got != tc.want {
			t.Errorf("IsNoopAt(module.compatibility_level, %q) = %v, want %v", tc.bazel, got, tc.want)
		}
	}
}

func TestIsDeprecatedAt_UnknownReturnsFalse(t *testing.T) {
	if IsDeprecatedAt("bazel_dep", "name", "9.1.1") {
		t.Error("bazel_dep.name should not be deprecated at any version")
	}
	if IsDeprecatedAt("unknown_directive", "anything", "9.0.0") {
		t.Error("unknown directive should return false")
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"7.0.0", "7.0.0", 0},
		{"7.0.0", "8.0.0", -1},
		{"9.1.0", "9.0.99", 1},
		{"8.6.0", "8.5.9", 1},
		{"8.6.0-rc1", "8.6.0", 0}, // pre-release suffix stripped
		{"9.1.0+build.42", "9.1.0", 0},
		{"garbage", "1.0.0", -1}, // unparsable -> zero -> smallest
	}
	for _, tc := range cases {
		if got := compareSemver(tc.a, tc.b); got != tc.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestParse_DropsTypedNilFromParseHelpers pins the fix for the typed-nil
// trap: when a per-directive parse helper (e.g. parseBazelDep) returns
// a typed-nil pointer after recording an error, the result must not
// leak into file.Statements (where the interface wrapping a nil
// pointer would survive an `s != nil` check and crash downstream
// handlers).
func TestParse_DropsTypedNilFromParseHelpers(t *testing.T) {
	// bazel_dep without name -> parseBazelDep returns (*BazelDep)(nil).
	const src = `bazel_dep(version = "1.0.0")`
	result, err := ParseContent("MODULE.bazel", []byte(src))
	if err != nil {
		t.Fatalf("ParseContent: %v", err)
	}
	if !result.HasErrors() {
		t.Fatal("expected an error in result.Errors for missing-name bazel_dep")
	}
	for _, stmt := range result.File.Statements {
		if stmt == nil {
			t.Fatal("nil statement leaked into File.Statements")
		}
		// Walk must not panic on whatever survived the filter.
		_ = Walk(result.File, &BaseHandler{})
	}
}

func TestParse_BazelDepRepoNameNone(t *testing.T) {
	const src = `
module(name = "root", version = "1.0.0")
bazel_dep(name = "nodep_target", version = "1.0.0", repo_name = None)
bazel_dep(name = "regular_dep", version = "2.0.0")
`
	result, err := ParseContent("MODULE.bazel", []byte(src))
	if err != nil {
		t.Fatalf("ParseContent: %v", err)
	}
	collector := &DependencyCollector{}
	if err := Walk(result.File, collector); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(collector.Dependencies) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(collector.Dependencies))
	}
	if !collector.Dependencies[0].IsNodepDep {
		t.Error("first dep should be marked IsNodepDep (repo_name=None)")
	}
	if collector.Dependencies[1].IsNodepDep {
		t.Error("second dep should NOT be marked IsNodepDep (repo_name omitted)")
	}
}

func TestParse_GitOverrideVerboseAndExtraKwargs(t *testing.T) {
	const src = `
module(name = "root", version = "1.0.0")
bazel_dep(name = "gazelle", version = "0.32.0")
git_override(
    module_name = "gazelle",
    remote = "https://github.com/bazelbuild/bazel-gazelle.git",
    commit = "abc123",
    verbose = True,
    shallow_since = "2026-01-01",
    recursive_init_submodules = True,
)
`
	result, err := ParseContent("MODULE.bazel", []byte(src))
	if err != nil {
		t.Fatalf("ParseContent: %v", err)
	}
	collector := &OverrideCollector{}
	if err := Walk(result.File, collector); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(collector.GitOverrides) != 1 {
		t.Fatalf("expected 1 git_override, got %d", len(collector.GitOverrides))
	}
	got := collector.GitOverrides[0]
	if !got.Verbose {
		t.Error("Verbose should be true")
	}
	if len(got.ExtraKwargs) != 2 {
		t.Fatalf("expected 2 ExtraKwargs (shallow_since, recursive_init_submodules), got %d", len(got.ExtraKwargs))
	}
	names := []string{got.ExtraKwargs[0].Name, got.ExtraKwargs[1].Name}
	if names[0] != "shallow_since" || names[1] != "recursive_init_submodules" {
		t.Errorf("ExtraKwargs names = %v, want [shallow_since recursive_init_submodules]", names)
	}
	if got.ExtraKwargs[0].Value != "2026-01-01" {
		t.Errorf("shallow_since value = %v, want \"2026-01-01\"", got.ExtraKwargs[0].Value)
	}
	if got.ExtraKwargs[1].Value != true {
		t.Errorf("recursive_init_submodules value = %v (%T), want true (bool)", got.ExtraKwargs[1].Value, got.ExtraKwargs[1].Value)
	}
}

func TestParse_ArchiveOverrideExtraKwargs(t *testing.T) {
	const src = `
module(name = "root", version = "1.0.0")
bazel_dep(name = "rules_foo", version = "1.0.0")
archive_override(
    module_name = "rules_foo",
    urls = ["https://example.com/rules_foo.tar.gz"],
    integrity = "sha256-abc=",
    type = "tar.gz",
    rename_files = {"old": "new"},
)
`
	result, err := ParseContent("MODULE.bazel", []byte(src))
	if err != nil {
		t.Fatalf("ParseContent: %v", err)
	}
	collector := &OverrideCollector{}
	if err := Walk(result.File, collector); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(collector.ArchiveOverrides) != 1 {
		t.Fatalf("expected 1 archive_override, got %d", len(collector.ArchiveOverrides))
	}
	got := collector.ArchiveOverrides[0]
	if len(got.ExtraKwargs) != 2 {
		t.Fatalf("expected 2 ExtraKwargs (type, rename_files), got %d (%+v)", len(got.ExtraKwargs), got.ExtraKwargs)
	}
	if got.ExtraKwargs[0].Name != "type" || got.ExtraKwargs[0].Value != "tar.gz" {
		t.Errorf("first extra = (%s, %v), want (type, tar.gz)", got.ExtraKwargs[0].Name, got.ExtraKwargs[0].Value)
	}
	if got.ExtraKwargs[1].Name != "rename_files" {
		t.Errorf("second extra name = %s, want rename_files", got.ExtraKwargs[1].Name)
	}
}
