package configengine

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

func TestIsConfigCandidate(t *testing.T) {
	t.Parallel()

	t.Run("generic config stem matches case-insensitively", func(t *testing.T) {
		t.Parallel()

		assert.True(t, isConfigCandidate("config", "server-precedence"))
		assert.True(t, isConfigCandidate("CONFIG", "server-precedence"))
	})

	t.Run("app-name stem matches case-insensitively", func(t *testing.T) {
		t.Parallel()

		assert.True(t, isConfigCandidate("server-precedence", "server-precedence"))
		assert.True(t, isConfigCandidate("Server-Precedence", "server-precedence"))
		assert.True(t, isConfigCandidate("cascading-app", "Cascading-App"))
	})

	t.Run("dot-prefixed stems are rejected", func(t *testing.T) {
		t.Parallel()

		assert.False(t, isConfigCandidate(".config", "server-precedence"))
		assert.False(t, isConfigCandidate(".server-precedence", "server-precedence"))
	})

	t.Run("rc-style stems are rejected", func(t *testing.T) {
		t.Parallel()

		assert.False(t, isConfigCandidate("server-precedencerc", "server-precedence"))
		assert.False(t, isConfigCandidate(".server-precedencerc", "server-precedence"))
	})

	t.Run("unrelated stems are rejected", func(t *testing.T) {
		t.Parallel()

		assert.False(t, isConfigCandidate("notes", "server-precedence"))
		assert.False(t, isConfigCandidate("config-backup", "server-precedence"))
	})

	t.Run("empty app name only matches the generic config stem", func(t *testing.T) {
		t.Parallel()

		assert.True(t, isConfigCandidate("config", ""))
		assert.False(t, isConfigCandidate("server-precedence", ""))
	})
}

func TestSplitList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		value     string
		separator string
		want      []string
	}{
		{name: "default comma separator", value: "a.json,b.json", separator: "", want: []string{"a.json", "b.json"}},
		{name: "custom separator", value: "a.json;b.json", separator: ";", want: []string{"a.json", "b.json"}},
		{name: "whitespace trimmed and empties dropped", value: " a.json , ,b.json ", separator: ",", want: []string{"a.json", "b.json"}},
		{name: "none disables splitting", value: "  a.json,b.json  ", separator: "none", want: []string{"a.json,b.json"}},
		{name: "off disables splitting", value: "a.json", separator: "off", want: []string{"a.json"}},
		{name: "dash disables splitting", value: "a.json", separator: "-", want: []string{"a.json"}},
		{name: "blank value yields nil", value: "   ", separator: ",", want: nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, SplitList(testCase.value, testCase.separator))
		})
	}
}

func TestEnvCandidates_SingleValueKeepsSeparator(t *testing.T) {
	t.Setenv("CANDIDATE_SINGLE_CONFIG", "/tmp/x/a,b.json")

	cmd := &spec.Command{ConfigFileFlag: &spec.Flag{}}

	candidates := EnvCandidates("CANDIDATE_SINGLE_", cmd)

	assert.Equal(t, []string{"/tmp/x/a,b.json"}, candidates)
}

func TestEnvCandidates_MultiValueSplitsOnSeparator(t *testing.T) {
	t.Setenv("CANDIDATE_MULTI_CONFIG", "/tmp/x/a.json, /tmp/x/b.json")

	cmd := &spec.Command{ConfigFileFlag: &spec.Flag{IsSlice: true}}

	candidates := EnvCandidates("CANDIDATE_MULTI_", cmd)

	assert.Equal(t, []string{"/tmp/x/a.json", "/tmp/x/b.json"}, candidates)
}

func TestEnvCandidates_ExpandsPaths(t *testing.T) {
	t.Setenv("CANDIDATE_CLEAN_CONFIG", "/tmp/./x/../a.json")

	cmd := &spec.Command{ConfigFileFlag: &spec.Flag{}}

	candidates := EnvCandidates("CANDIDATE_CLEAN_", cmd)

	assert.Equal(t, []string{"/tmp/a.json"}, candidates)
}

func TestEnvCandidates_UnsetReturnsNil(t *testing.T) {
	t.Setenv("CANDIDATE_UNSET_CONFIG", "")

	cmd := &spec.Command{ConfigFileFlag: &spec.Flag{}}

	candidates := EnvCandidates("CANDIDATE_UNSET_", cmd)

	assert.Equal(t, []string(nil), candidates)
}

func TestSkippedTargets_FlagSkipsEnvAndExplicit(t *testing.T) {
	t.Setenv("SKIPPED_FLAG_CONFIG", "/work/team.json")

	resolver := Resolver{EnvPrefix: "SKIPPED_FLAG_", Settings: spec.ConfigSettings{Explicit: "/etc/app.json"}}
	cmd := &spec.Command{ConfigFileFlag: &spec.Flag{}}

	skipped := resolver.SkippedTargets([]string{"/work/ci.json"}, cmd)

	assert.Equal(t, []string{"/work/team.json", "/etc/app.json"}, skipped)
}

func TestSkippedTargets_EnvSkipsExplicit(t *testing.T) {
	t.Setenv("SKIPPED_ENV_CONFIG", "/work/team.json")

	resolver := Resolver{EnvPrefix: "SKIPPED_ENV_", Settings: spec.ConfigSettings{Explicit: "/etc/app.json"}}
	cmd := &spec.Command{ConfigFileFlag: &spec.Flag{}}

	skipped := resolver.SkippedTargets(nil, cmd)

	assert.Equal(t, []string{"/etc/app.json"}, skipped)
}

func TestSkippedTargets_MultiValueEnvSkippedPerPath(t *testing.T) {
	t.Setenv("SKIPPED_MULTI_CONFIG", "/work/a.json,/work/b.json")

	resolver := Resolver{EnvPrefix: "SKIPPED_MULTI_"}
	cmd := &spec.Command{ConfigFileFlag: &spec.Flag{IsSlice: true}}

	skipped := resolver.SkippedTargets([]string{"/work/ci.json"}, cmd)

	assert.Equal(t, []string{"/work/a.json", "/work/b.json"}, skipped)
}

func TestSkippedTargets_ExplicitAloneSkipsNothing(t *testing.T) {
	t.Setenv("SKIPPED_EXPLICIT_CONFIG", "")

	resolver := Resolver{EnvPrefix: "SKIPPED_EXPLICIT_", Settings: spec.ConfigSettings{Explicit: "/etc/app.json"}}
	cmd := &spec.Command{ConfigFileFlag: &spec.Flag{}}

	skipped := resolver.SkippedTargets(nil, cmd)

	assert.Equal(t, []string(nil), skipped)
}

func TestSkippedTargets_SearchTierNeverReported(t *testing.T) {
	t.Setenv("SKIPPED_TIER_CONFIG", "")

	resolver := Resolver{EnvPrefix: "SKIPPED_TIER_", Settings: spec.ConfigSettings{Tier: []string{"/etc/app"}}}
	cmd := &spec.Command{ConfigFileFlag: &spec.Flag{}}

	skipped := resolver.SkippedTargets([]string{"/work/ci.json"}, cmd)

	assert.Equal(t, []string(nil), skipped)
}
