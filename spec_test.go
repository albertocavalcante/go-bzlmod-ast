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

// TestIsDeprecatedAt_VersionBoundaries pins the per-version semantics
// for the real compatibility_level / max_compatibility_level data:
// deprecation FIRST shipped in 8.6.0 (2026-02-26) and was forward-ported
// to 9.1.0 (2026-04-20). Bazel 7.x never saw the deprecation. 8.5.x and
// 9.0.x were pre-deprecation.
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

// ============================================================================
// Cherry-pick / backport scenarios -- the model is per-major lifecycle slices.
// Each test below uses synthetic stages so we can pin the contract independent
// of whatever real Bazel does this week.
// ============================================================================

// TestReachedStageAt_ForwardOnlyChange: a transition that lands in 9.2.0 and
// is never backported. Users on 9.0.x or 9.1.x see the OLD behavior; users
// from 9.2.0 onward see the NEW behavior; users on 7.x / 8.x never see it.
func TestReachedStageAt_ForwardOnlyChange(t *testing.T) {
	stage := []string{"9.2.0"}
	cases := []struct {
		v    string
		want bool
	}{
		{"7.7.1", false},  // 7.x branch never touched
		{"8.7.0", false},  // 8.x branch never touched
		{"9.0.0", false},  // 9.0.x predates the change
		{"9.0.99", false}, // hypothetical late 9.0.x patch -- still no backport
		{"9.1.99", false}, // hypothetical late 9.1.x patch -- still no backport
		{"9.2.0", true},   // exact landing
		{"9.2.5", true},
		{"9.10.0", true}, // future patch in the 9.x line
	}
	for _, tc := range cases {
		if got := reachedStageAt(stage, tc.v); got != tc.want {
			t.Errorf("forward-only 9.2.0 @ %q = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// TestReachedStageAt_BackportTo8x: the same logical 9.2.0 change cherry-picked
// into 8.7.0 the same week. Users on either branch see the new behavior from
// their branch-specific landing version onward; older versions on each branch
// still see the old behavior.
func TestReachedStageAt_BackportTo8x(t *testing.T) {
	stage := []string{"8.7.0", "9.2.0"}
	cases := []struct {
		v    string
		want bool
	}{
		// 7.x: never touched.
		{"7.7.1", false},
		// 8.x boundary.
		{"8.0.0", false},
		{"8.6.99", false}, // last patch before the backport
		{"8.7.0", true},   // backport landing
		{"8.7.1", true},
		{"8.99.0", true},
		// 9.x boundary.
		{"9.0.0", false},
		{"9.1.99", false}, // last 9.1.x patch still pre-forward-port
		{"9.2.0", true},   // forward-port landing
		{"9.2.5", true},
	}
	for _, tc := range cases {
		if got := reachedStageAt(stage, tc.v); got != tc.want {
			t.Errorf("backport [8.7.0, 9.2.0] @ %q = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// TestReachedStageAt_CherryPickToFinal7x: a change cherry-picked only to the
// final patch release of an EOL-bound branch (7.7.1 was the last 7.x). Earlier
// 7.x users do not get it; the new 8.x / 9.x lines may or may not get it
// depending on the spec.
func TestReachedStageAt_CherryPickToFinal7x(t *testing.T) {
	stage := []string{"7.7.1"}
	cases := []struct {
		v    string
		want bool
	}{
		{"7.7.0", false}, // pre-cherry-pick
		{"7.7.1", true},  // exact landing
		{"8.0.0", false}, // different major, no entry -> false
		{"9.0.0", false},
	}
	for _, tc := range cases {
		if got := reachedStageAt(stage, tc.v); got != tc.want {
			t.Errorf("7.7.1-only @ %q = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// TestReachedStageAt_EmptyMeansNeverLanded: empty stage slice means the
// transition has not landed on any in-scope branch.
func TestReachedStageAt_EmptyMeansNeverLanded(t *testing.T) {
	if reachedStageAt(nil, "9.99.99") {
		t.Error("nil stage should return false for any version")
	}
	if reachedStageAt([]string{}, "9.99.99") {
		t.Error("empty stage should return false for any version")
	}
}

// TestReachedStageAt_PreRelease: pre-release / build-metadata suffixes are
// stripped before comparison, so 9.2.0-rc1 compares equal to 9.2.0.
func TestReachedStageAt_PreRelease(t *testing.T) {
	stage := []string{"9.2.0"}
	if !reachedStageAt(stage, "9.2.0-rc1") {
		t.Error("9.2.0-rc1 should reach the 9.2.0 stage")
	}
	if !reachedStageAt(stage, "9.2.0+build.5") {
		t.Error("9.2.0+build.5 should reach the 9.2.0 stage")
	}
}

// TestIsAvailableAt_BackportedNewKwarg: a brand-new kwarg added in 9.2.0 and
// cherry-picked back into 8.7.0 so 8.x users can migrate at their own pace.
// IsAvailableAt should reflect per-branch availability.
func TestIsAvailableAt_BackportedNewKwarg(t *testing.T) {
	const directive = "test_directive"
	const attr = "shiny_kwarg"
	directiveAttrs[directive] = func() []AttrSpec {
		return []AttrSpec{
			{
				Name:    attr,
				AddedIn: []string{"8.7.0", "9.2.0"},
			},
		}
	}
	t.Cleanup(func() { delete(directiveAttrs, directive) })

	cases := []struct {
		v    string
		want bool
	}{
		{"7.7.1", false}, // never on 7.x
		{"8.6.0", false}, // pre-backport on 8.x
		{"8.7.0", true},
		{"8.99.0", true},
		{"9.0.0", false}, // pre-forward-port on 9.x
		{"9.1.99", false},
		{"9.2.0", true},
		{"9.99.0", true},
	}
	for _, tc := range cases {
		if got := IsAvailableAt(directive, attr, tc.v); got != tc.want {
			t.Errorf("IsAvailableAt(%q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// TestIsRemovedAt_BranchSpecific: a kwarg deleted on 9.x in 9.5.0 stays
// available on 8.x indefinitely. Mirrors a deprecation-then-removal flow.
func TestIsRemovedAt_BranchSpecific(t *testing.T) {
	const directive = "test_directive_removal"
	const attr = "doomed_kwarg"
	directiveAttrs[directive] = func() []AttrSpec {
		return []AttrSpec{
			{
				Name:      attr,
				RemovedIn: []string{"9.5.0"}, // gone on 9.x only
			},
		}
	}
	t.Cleanup(func() { delete(directiveAttrs, directive) })

	cases := []struct {
		v         string
		removed   bool
		available bool
	}{
		{"7.7.1", false, true},  // 7.x: not removed
		{"8.7.0", false, true},  // 8.x: not removed
		{"9.4.99", false, true}, // pre-removal on 9.x
		{"9.5.0", true, false},  // 9.5.0+ on 9.x: removed -> unavailable
		{"9.5.1", true, false},
		{"9.99.0", true, false},
	}
	for _, tc := range cases {
		if got := IsRemovedAt(directive, attr, tc.v); got != tc.removed {
			t.Errorf("IsRemovedAt(%q) = %v, want %v", tc.v, got, tc.removed)
		}
		if got := IsAvailableAt(directive, attr, tc.v); got != tc.available {
			t.Errorf("IsAvailableAt(%q) = %v, want %v", tc.v, got, tc.available)
		}
	}
}

// TestIsAvailableAt_AddedThenRemoved: a kwarg that was BOTH backported in
// (AddedIn=[8.7.0, 9.2.0]) and later REMOVED on the 9.x branch
// (RemovedIn=[9.5.0]). IsAvailableAt must compose both gates.
func TestIsAvailableAt_AddedThenRemoved(t *testing.T) {
	const directive = "test_directive_both"
	const attr = "shortlived_kwarg"
	directiveAttrs[directive] = func() []AttrSpec {
		return []AttrSpec{
			{
				Name:      attr,
				AddedIn:   []string{"8.7.0", "9.2.0"},
				RemovedIn: []string{"9.5.0"},
			},
		}
	}
	t.Cleanup(func() { delete(directiveAttrs, directive) })

	cases := []struct {
		v    string
		want bool
	}{
		{"7.7.1", false}, // never added on 7.x
		{"8.6.0", false}, // pre-backport on 8.x
		{"8.7.0", true},  // backport on 8.x, no removal here -> available
		{"8.99.0", true}, // 8.x lives on
		{"9.0.0", false}, // 9.x: not yet added
		{"9.2.0", true},  // 9.x: added
		{"9.4.99", true}, // 9.x: still there
		{"9.5.0", false}, // 9.x: removed
		{"9.5.1", false},
	}
	for _, tc := range cases {
		if got := IsAvailableAt(directive, attr, tc.v); got != tc.want {
			t.Errorf("IsAvailableAt(%q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// TestIsAvailableAt_IncludeDirective pins the AddedIn data for include().
// Real Bazel emits "unknown function 'include'" at 7.1.x and earlier;
// at 7.2.0 onward it parses. This test mirrors that semantic against
// the spec's per-major AddedIn list.
func TestIsAvailableAt_IncludeDirective(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		// 7.x branch: include() arrived in 7.2.0.
		{"7.0.0", false},
		{"7.1.0", false},
		{"7.1.2", false},
		{"7.2.0", true},
		{"7.2.1", true},
		{"7.7.1", true},
		// 8.x branch: included from 8.0.0 onward.
		{"8.0.0", true},
		{"8.7.0", true},
		// 9.x branch: included from 9.0.0 onward.
		{"9.0.0", true},
		{"9.1.1", true},
	}
	for _, tc := range cases {
		if got := IsAvailableAt("include", "label", tc.v); got != tc.want {
			t.Errorf("IsAvailableAt(include.label, %q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// TestIsAvailableAt_FlagAliasNonMonotone pins the cherry-pick story for
// flag_alias(): added in 7.7.0 on 7.x, REMOVED in 8.0.0 (8.0.x-8.4.x do
// not have it), RE-ADDED in 8.5.0, present in 9.0.0 onward. The per-major
// AddedIn slice models this naturally because the 8.x entry is 8.5.0,
// so 8.0.0-8.4.x query as "not available".
func TestIsAvailableAt_FlagAliasNonMonotone(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		// 7.x branch.
		{"7.6.0", false}, // pre-add
		{"7.7.0", true},  // added
		{"7.7.1", true},
		// 8.x branch: gap from 8.0.0 to 8.4.x.
		{"8.0.0", false},
		{"8.4.2", false}, // last 8.x version without flag_alias
		{"8.5.0", true},  // re-added
		{"8.5.1", true},
		{"8.6.0", true},
		{"8.7.0", true},
		// 9.x branch.
		{"9.0.0", true},
		{"9.1.1", true},
	}
	for _, tc := range cases {
		if got := IsAvailableAt("flag_alias", "name", tc.v); got != tc.want {
			t.Errorf("IsAvailableAt(flag_alias.name, %q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// TestIsAvailableAt_OverrideRepoInjectRepo pins the 7.4.0 / 8.0.0 / 9.0.0
// arrival boundaries for override_repo and inject_repo.
func TestIsAvailableAt_OverrideRepoInjectRepo(t *testing.T) {
	for _, directive := range []string{"override_repo", "inject_repo"} {
		t.Run(directive, func(t *testing.T) {
			cases := []struct {
				v    string
				want bool
			}{
				{"7.3.0", false},
				{"7.3.2", false},
				{"7.4.0", true},
				{"7.7.1", true},
				{"8.0.0", true},
				{"9.0.0", true},
			}
			for _, tc := range cases {
				if got := IsAvailableAt(directive, "extension_proxy", tc.v); got != tc.want {
					t.Errorf("IsAvailableAt(%s.extension_proxy, %q) = %v, want %v",
						directive, tc.v, got, tc.want)
				}
			}
		})
	}
}

// TestIsAvailableAt_AlwaysAvailable spot-checks attrs whose AddedIn is
// empty (always present in scope): they should report available for
// every version we query.
func TestIsAvailableAt_AlwaysAvailable(t *testing.T) {
	checks := []struct {
		directive string
		attr      string
	}{
		{"module", "name"},
		{"bazel_dep", "version"},
		{"use_extension", "isolate"}, // present since 7.0.0
	}
	for _, c := range checks {
		for _, v := range []string{"7.0.0", "7.7.1", "8.5.1", "9.1.1"} {
			if !IsAvailableAt(c.directive, c.attr, v) {
				t.Errorf("IsAvailableAt(%s.%s, %q) = false, want true (always-available attr)",
					c.directive, c.attr, v)
			}
		}
	}
}

// TestAttrSpec_AtMostOneEntryPerMajor enforces the per-major-uniqueness
// invariant across every registered directive in the spec. Two entries
// sharing a major would silently cause the first-wins resolver to return
// stale answers on the second cherry-pick window.
func TestAttrSpec_AtMostOneEntryPerMajor(t *testing.T) {
	check := func(t *testing.T, directive, attr, field string, stage []string) {
		t.Helper()
		seen := map[int]string{}
		for _, v := range stage {
			parts := parseSemver3(v)
			major := parts[0]
			if prev, ok := seen[major]; ok {
				t.Errorf("%s.%s: %s has two entries for major %d (%q and %q); only the earliest may be recorded",
					directive, attr, field, major, prev, v)
			}
			seen[major] = v
		}
	}
	for directive, fn := range directiveAttrs {
		for _, s := range fn() {
			check(t, directive, s.Name, "AddedIn", s.AddedIn)
			check(t, directive, s.Name, "DeprecatedIn", s.DeprecatedIn)
			check(t, directive, s.Name, "NoopSince", s.NoopSince)
			check(t, directive, s.Name, "RemovedIn", s.RemovedIn)
		}
	}
}
