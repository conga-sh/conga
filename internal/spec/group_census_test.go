package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

func TestGroupCensus_MultiMemberGroupsAreLegal(t *testing.T) {
	t.Parallel()

	census := spec.NewGroupCensus[string]()
	census.Add("oneof", "format", "json", false, "JSON")
	census.Add("oneof", "format", "yaml", false, "YAML")
	census.Add("xor", "transport", "tcp", false, "TCP")
	census.Add("xor", "transport", "udp", false, "UDP")
	census.Add("xor", "transport", "quic", false, "QUIC")
	census.Add("and", "auth", "user", false, "User")
	census.Add("and", "auth", "password", false, "Password")

	assert.Zero(t, census.SingleMemberGroups())
}

func TestGroupCensus_SingleMemberKindsReported(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"oneof", "xor", "and"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			census := spec.NewGroupCensus[string]()
			census.Add(kind, "solo", "format", false, "Format")

			violations := census.SingleMemberGroups()

			assert.Equal(t, 1, len(violations))
			assert.Equal(t, kind, violations[0].Kind)
			assert.Equal(t, "solo", violations[0].Group)
			assert.Equal(t, "format", violations[0].FlagName)
			assert.Equal(t, "Format", violations[0].Attribution)
			assert.Equal(t, "", violations[0].Suggestion)
		})
	}
}

func TestGroupCensus_SingleOneOfCounterIsLegal(t *testing.T) {
	t.Parallel()

	census := spec.NewGroupCensus[string]()
	census.Add("oneof", "mode", "verbose", true, "Verbose")

	assert.Zero(t, census.SingleMemberGroups())
}

func TestGroupCensus_SingleXorCounterIsReported(t *testing.T) {
	t.Parallel()

	census := spec.NewGroupCensus[string]()
	census.Add("xor", "mode", "verbose", true, "Verbose")

	violations := census.SingleMemberGroups()

	assert.Equal(t, 1, len(violations))
	assert.Equal(t, "xor", violations[0].Kind)
}

func TestGroupCensus_ViolationOrderFollowsFirstDeclaration(t *testing.T) {
	t.Parallel()

	census := spec.NewGroupCensus[string]()
	census.Add("xor", "second", "second-a", false, "SecondA")
	census.Add("and", "legal", "legal-a", false, "LegalA")
	census.Add("and", "legal", "legal-b", false, "LegalB")
	census.Add("xor", "first", "first-a", false, "FirstA")

	violations := census.SingleMemberGroups()

	groups := make([]string, 0, len(violations))
	for _, violation := range violations {
		groups = append(groups, violation.Group)
	}

	assert.Equal(t, []string{"second", "first"}, groups)
}

func TestGroupCensus_TypoGroupSuggestsPopulatedGroup(t *testing.T) {
	t.Parallel()

	census := spec.NewGroupCensus[string]()
	census.Add("oneof", "format", "json", false, "JSON")
	census.Add("oneof", "format", "yaml", false, "YAML")
	census.Add("oneof", "fromat", "text", false, "Text")

	violations := census.SingleMemberGroups()

	assert.Equal(t, 1, len(violations))
	assert.Equal(t, "fromat", violations[0].Group)
	assert.Equal(t, "format", violations[0].Suggestion)
}

func TestGroupCensus_SplitPairHasNoSuggestion(t *testing.T) {
	t.Parallel()

	census := spec.NewGroupCensus[string]()
	census.Add("oneof", "format", "json", false, "JSON")
	census.Add("oneof", "fromat", "yaml", false, "YAML")

	violations := census.SingleMemberGroups()

	assert.Equal(t, 2, len(violations))

	for _, violation := range violations {
		assert.Equal(t, "", violation.Suggestion)
	}
}

func TestGroupCensus_DifferentKindGroupIsNotSuggested(t *testing.T) {
	t.Parallel()

	census := spec.NewGroupCensus[string]()
	census.Add("xor", "format", "json", false, "JSON")
	census.Add("xor", "format", "yaml", false, "YAML")
	census.Add("oneof", "fromat", "text", false, "Text")

	violations := census.SingleMemberGroups()

	assert.Equal(t, 1, len(violations))
	assert.Equal(t, "oneof", violations[0].Kind)
	assert.Equal(t, "", violations[0].Suggestion)
}

func TestGroupCensus_SameSpellingAcrossKindsIsIndependent(t *testing.T) {
	t.Parallel()

	census := spec.NewGroupCensus[string]()
	census.Add("oneof", "format", "json", false, "JSON")
	census.Add("xor", "format", "yaml", false, "YAML")
	census.Add("xor", "format", "text", false, "Text")

	violations := census.SingleMemberGroups()

	assert.Equal(t, 1, len(violations))
	assert.Equal(t, "oneof", violations[0].Kind)
	assert.Equal(t, "format", violations[0].Group)
	assert.Equal(t, "", violations[0].Suggestion)
}

func TestGroupCensus_EmptyGroupIgnored(t *testing.T) {
	t.Parallel()

	census := spec.NewGroupCensus[string]()
	census.Add("oneof", "", "json", false, "JSON")
	census.Add("xor", "", "yaml", false, "YAML")

	assert.Zero(t, census.SingleMemberGroups())
}
