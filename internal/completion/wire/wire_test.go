package wire_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/predict"
)

func TestFromResultDefaultsDisplayToValue(t *testing.T) {
	t.Parallel()

	result := predict.Values("deploy", "logs").Invoke(predict.Context{})

	candidates, _ := wire.FromResult(result)

	assert.Equal(t, []wire.Candidate{
		{Value: "deploy", Display: "deploy"},
		{Value: "logs", Display: "logs"},
	}, candidates)
}

func TestFromResultPreservesDisplayOverride(t *testing.T) {
	t.Parallel()

	result := predict.Items(predict.NewItem("region", "us-east-1").WithDisplay("us-east-1 (N. Virginia)")).Invoke(predict.Context{})

	candidates, _ := wire.FromResult(result)

	assert.Equal(t, []wire.Candidate{
		{Value: "region", Display: "us-east-1 (N. Virginia)", Description: "us-east-1"},
	}, candidates)
}

func TestFromResultCopiesFieldsVerbatimInEngineOrder(t *testing.T) {
	t.Parallel()

	result := predict.Result{
		Items: []predict.Item{
			{Value: "deploy", Display: "deploy", Description: "Deploy service", Style: "blue", Tag: "commands"},
			{Value: "--verbose", Display: "--verbose", Tag: "longhand flags"},
		},
	}

	candidates, _ := wire.FromResult(result)

	assert.Equal(t, []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service", Style: "blue", Tag: "commands"},
		{Value: "--verbose", Display: "--verbose", Tag: "longhand flags"},
	}, candidates)
}

func TestFromResultSuppressesAliasCandidates(t *testing.T) {
	t.Setenv(wire.EnvAliases, "0")

	result := predict.Result{Items: []predict.Item{
		{Value: "deploy", Display: "deploy", Description: "Deploy service"},
		{Value: "d", Display: "d", Description: "Deploy service", Alias: true},
	}}

	candidates, _ := wire.FromResult(result)

	assert.Equal(t, []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service"},
	}, candidates)
}

func TestFromResultKeepsAliasCandidatesMarked(t *testing.T) {
	t.Setenv(wire.EnvAliases, "1")

	result := predict.Result{Items: []predict.Item{
		{Value: "deploy", Display: "deploy"},
		{Value: "d", Display: "d", Alias: true},
	}}

	candidates, _ := wire.FromResult(result)

	assert.Equal(t, []wire.Candidate{
		{Value: "deploy", Display: "deploy"},
		{Value: "d", Display: "d", Alias: true},
	}, candidates)
}

func TestFromResultSuppressesDescriptions(t *testing.T) {
	t.Setenv(wire.EnvDescriptions, "0")

	result := predict.Result{Items: []predict.Item{
		{Value: "deploy", Display: "deploy", Description: "Deploy service"},
	}}

	candidates, _ := wire.FromResult(result)

	assert.Equal(t, []wire.Candidate{{Value: "deploy", Display: "deploy"}}, candidates)
}

func TestFromResultNoSpaceDirectiveSetsWildcard(t *testing.T) {
	t.Parallel()

	result := predict.Values("deploy=").NoSpace().Invoke(predict.Context{})

	_, directives := wire.FromResult(result)

	assert.True(t, directives.NoSpace.Matches("anything"))
	assert.Equal(t, []string{"nospace"}, directives.Tokens())
}

func TestFromResultWithoutNoSpaceDirectiveLeavesSetEmpty(t *testing.T) {
	t.Parallel()

	result := predict.Values("deploy").Invoke(predict.Context{})

	_, directives := wire.FromResult(result)

	assert.True(t, directives.NoSpace.IsEmpty())
	assert.Zero(t, directives.Tokens())
}

func TestFromResultKeepOrderFlag(t *testing.T) {
	t.Parallel()

	t.Run("set", func(t *testing.T) {
		t.Parallel()

		result := predict.Values("b", "a").KeepOrder().Invoke(predict.Context{})

		_, directives := wire.FromResult(result)

		assert.True(t, directives.KeepOrder)
		assert.Equal(t, []string{"keeporder"}, directives.Tokens())
	})

	t.Run("unset", func(t *testing.T) {
		t.Parallel()

		result := predict.Values("a").Invoke(predict.Context{})

		_, directives := wire.FromResult(result)

		assert.False(t, directives.KeepOrder)
	})
}

func TestFromResultErrorWithUsageAddsUsageMessage(t *testing.T) {
	t.Parallel()

	result := predict.None().Invoke(predict.Context{})
	result.Usage = "usage: app <command>"
	result.Directive |= predict.DirectiveError

	_, directives := wire.FromResult(result)

	assert.Equal(t, []string{"usage: app <command>"}, directives.Messages)
	assert.Equal(t, "usage: app <command>", directives.Usage)
}

func TestFromResultErrorWithoutUsageAddsErrorMessage(t *testing.T) {
	t.Parallel()

	result := predict.None().Invoke(predict.Context{})
	result.Directive |= predict.DirectiveError

	_, directives := wire.FromResult(result)

	assert.Equal(t, []string{"ERROR"}, directives.Messages)
	assert.Equal(t, "", directives.Usage)
}

func TestFromResultUsagePassthroughWithoutError(t *testing.T) {
	t.Parallel()

	result := predict.None().Invoke(predict.Context{})
	result.Usage = "usage: app <command>"

	_, directives := wire.FromResult(result)

	assert.Equal(t, "usage: app <command>", directives.Usage)
	assert.Zero(t, directives.Messages)
}

func TestPrefixSetAddDeduplicatesAndSorts(t *testing.T) {
	t.Parallel()

	set := wire.PrefixSet{}

	set.Add('-')
	set.Add('_', '-', '+')

	assert.Equal(t, "+-_", set.String())
}

func TestPrefixSetAddWildcardCollapses(t *testing.T) {
	t.Parallel()

	t.Run("adding wildcard", func(t *testing.T) {
		t.Parallel()

		set := wire.PrefixSet{}

		set.Add('-', '*')

		assert.True(t, set.Matches("any-value"))
	})

	t.Run("adding to wildcard set", func(t *testing.T) {
		t.Parallel()

		set := wire.PrefixSet{}

		set.Add('*')
		set.Add('-')

		assert.True(t, set.Matches("any-value"))
	})
}

func TestPrefixSetMatchesFirstRune(t *testing.T) {
	t.Parallel()

	set := wire.PrefixSet{}

	set.Add('-')

	assert.True(t, set.Matches("--flag"))
	assert.False(t, set.Matches("flag"))
}

func TestPrefixSetMatchesMultibyteRune(t *testing.T) {
	t.Parallel()

	set := wire.PrefixSet{}

	set.Add('€')

	assert.True(t, set.Matches("€50"))
	assert.False(t, set.Matches("$50"))
}

func TestPrefixSetEmptySetNeverMatches(t *testing.T) {
	t.Parallel()

	set := wire.PrefixSet{}

	assert.False(t, set.Matches("--flag"))
	assert.True(t, set.IsEmpty())
}

func TestPrefixSetEmptyValueNeverMatches(t *testing.T) {
	t.Parallel()

	t.Run("concrete rune", func(t *testing.T) {
		t.Parallel()

		set := wire.PrefixSet{}

		set.Add('-')

		assert.False(t, set.Matches(""))
	})

	t.Run("wildcard", func(t *testing.T) {
		t.Parallel()

		set := wire.PrefixSet{}

		set.Add('*')

		assert.False(t, set.Matches(""))
	})
}

func TestSuffixSetMatchesLastRune(t *testing.T) {
	t.Parallel()

	set := wire.SuffixSet{}

	set.Add('/')

	assert.True(t, set.Matches("config/"))
	assert.False(t, set.Matches("config"))
}

func TestSuffixSetMatchesMultibyteRune(t *testing.T) {
	t.Parallel()

	set := wire.SuffixSet{}

	set.Add('→')

	assert.True(t, set.Matches("a →"))
	assert.False(t, set.Matches("a ←"))
}

func TestSuffixSetAddDeduplicatesAndSorts(t *testing.T) {
	t.Parallel()

	set := wire.SuffixSet{}

	set.Add('/')
	set.Add('=', '/')

	assert.Equal(t, "/=", set.String())
	assert.True(t, set.Matches("config/"))
	assert.True(t, set.Matches("--env="))
}

func TestSuffixSetEmptyValueNeverMatches(t *testing.T) {
	t.Parallel()

	set := wire.SuffixSet{}

	set.Add('/')

	assert.False(t, set.Matches(""))
}

func TestDirectivesTokensFixedOrder(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{KeepOrder: true}
	directives.NoPrefix.Add('-')
	directives.NoSpace.Add('*')

	tokens := directives.Tokens()

	assert.Equal(t, []string{"keeporder", "noprefix", "nospace"}, tokens)
}

func TestDirectivesTokensEmpty(t *testing.T) {
	t.Parallel()

	tokens := wire.Directives{}.Tokens()

	assert.Zero(t, tokens)
}

func TestDirectivesNoSpaceFor(t *testing.T) {
	t.Parallel()

	t.Run("wildcard directive matches every value", func(t *testing.T) {
		t.Parallel()

		directives := wire.Directives{}

		directives.NoSpace.Add('*')

		assert.True(t, directives.NoSpaceFor("anything"))
	})

	t.Run("trailing equals sign", func(t *testing.T) {
		t.Parallel()

		directives := wire.Directives{}

		assert.True(t, directives.NoSpaceFor("--env="))
	})

	t.Run("trailing slash", func(t *testing.T) {
		t.Parallel()

		directives := wire.Directives{}

		assert.True(t, directives.NoSpaceFor("config/"))
	})

	t.Run("plain value", func(t *testing.T) {
		t.Parallel()

		directives := wire.Directives{}

		assert.False(t, directives.NoSpaceFor("deploy"))
	})
}

func TestCandidateTrimmedDescription(t *testing.T) {
	firstLine := wire.Candidate{Description: "Deploy service\nto a cluster"}

	assert.Equal(t, "Deploy service", firstLine.TrimmedDescription())

	trimmed := wire.Candidate{Description: "  Deploy service  "}

	assert.Equal(t, "Deploy service", trimmed.TrimmedDescription())

	short := wire.Candidate{Description: "Deploy service"}

	assert.Equal(t, "Deploy service", short.TrimmedDescription())

	t.Setenv(wire.EnvDescriptionLength, "10")

	clamped := wire.Candidate{Description: "Deploy the service now"}

	assert.Equal(t, "Deploy ...", clamped.TrimmedDescription())

	t.Setenv(wire.EnvDescriptionLength, "3")

	hardCut := wire.Candidate{Description: "Deploy service"}

	assert.Equal(t, "Dep", hardCut.TrimmedDescription())

	t.Setenv(wire.EnvDescriptionLength, "1")

	tinyLimit := wire.Candidate{Description: "Deploy service"}

	assert.Equal(t, "D", tinyLimit.TrimmedDescription())

	t.Setenv(wire.EnvDescriptionLength, "5")

	runes := wire.Candidate{Description: "héllo wörld"}

	assert.Equal(t, "hé...", runes.TrimmedDescription())
}

func TestStripControl(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "clean string", wire.StripControl("clean string", "\n\r\t"))
	assert.Equal(t, "foobar", wire.StripControl("foo\n\r\tbar", "\n\r\t"))
	assert.Equal(t, "", wire.StripControl("\n\n\r\t", "\n\r\t"))
}

func TestReplaceControlWithSpace(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "clean string", wire.ReplaceControlWithSpace("clean string", "\n\r\t"))
	assert.Equal(t, "foo   bar", wire.ReplaceControlWithSpace("foo\n\r\tbar", "\n\r\t"))
	assert.Equal(t, "    ", wire.ReplaceControlWithSpace("\n\n\r\t", "\n\r\t"))
}
