package ast

import "testing"

func TestLookupAttr_Known(t *testing.T) {
	s, ok := LookupAttr("bazel_dep", "max_compatibility_level")
	if !ok {
		t.Fatal("LookupAttr(bazel_dep, max_compatibility_level) returned not-ok")
	}
	if s.DeprecatedIn == "" {
		t.Errorf("max_compatibility_level should be marked deprecated; got DeprecatedIn=%q", s.DeprecatedIn)
	}
	if s.NoopSince == "" {
		t.Errorf("max_compatibility_level should be marked noop; got NoopSince=%q", s.NoopSince)
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
