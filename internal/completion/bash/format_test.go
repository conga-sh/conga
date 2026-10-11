package bash //nolint:testpackage // exercises the unexported session presentation state

import (
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
)

func TestFormat_DirectiveHeaderAlwaysFirst(t *testing.T) {
	t.Parallel()

	formatted := NewSession().Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy"}})

	assert.Equal(t, "D\t\nC\tdeploy", formatted)
}

func TestFormat_MessageLinesFollowHeader(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{Messages: []string{"usage: app <command>"}}
	formatted := NewSession().Format("", directives, nil)

	assert.Equal(t, "D\t\nM\tusage: app <command>", formatted)
}

func TestFormat_KeepOrderToken(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{KeepOrder: true}
	formatted := NewSession().Format("", directives, []wire.Candidate{{Value: "deploy"}})

	assert.Equal(t, "D\tkeeporder\nC\tdeploy", formatted)
}

func TestFormat_NospaceTokenForValueSuffixes(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "owner="}, {Value: "team="}}
	formatted := NewSession().Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnospace\nC\towner=\nC\tteam=", formatted)
}

func TestFormat_NospaceTokenForDirectorySuffix(t *testing.T) {
	t.Parallel()

	formatted := NewSession().Format("", wire.Directives{}, []wire.Candidate{{Value: "build/"}})

	assert.Equal(t, "D\tnospace\nC\tbuild/", formatted)
}

func TestFormat_NospaceTokenFromEngineSet(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{NoSpace: wire.SuffixSet{}}
	directives.NoSpace.Add('*')

	formatted := NewSession().Format("", directives, []wire.Candidate{{Value: "deploy"}})

	assert.Equal(t, "D\tnospace\nC\tdeploy", formatted)
}

func TestFormat_NoprefixTokenForAttachedValues(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{
		{Value: "--env=staging", Display: "staging"},
		{Value: "--env=production", Display: "production"},
	}

	formatted := NewSession().Format("--env=", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnoprefix\nC\tstaging\nC\tproduction", formatted)
}

func TestFormat_QuotesValuesWithMetacharacters(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "two words"}, {Value: `a"b$c\`}}
	formatted := NewSession().Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\t\"two words\"\nC\t\"a\\\"b\\$c\\\\\"", formatted)
}

func TestFormat_QuotesMidWordMetacharacters(t *testing.T) {
	t.Parallel()

	formatted := NewSession().Format("", wire.Directives{}, []wire.Candidate{{Value: "a;b|c&d"}})

	assert.Equal(t, "D\t\nC\t\"a;b|c&d\"", formatted)
}

func TestFormat_BackslashEscapesTildeLeadingValues(t *testing.T) {
	t.Parallel()

	formatted := NewSession().Format("", wire.Directives{}, []wire.Candidate{{Value: "~/my dir/config"}})

	assert.Equal(t, "D\t\nC\t~/my\\ dir/config", formatted)
}

func TestFormat_SanitizesFramingCharactersInValues(t *testing.T) {
	t.Parallel()

	formatted := NewSession().Format("", wire.Directives{}, []wire.Candidate{{Value: "with\ttab\nnewline"}})

	assert.Equal(t, "D\t\nC\t\"with tab newline\"", formatted)
}

func TestFormat_StripsWordbreakPrefixDerivedFromToComplete(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "team=core", Display: "core"}}

	formatted := NewSession().Format("team=", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnoprefix\nC\tcore", formatted)
}

func TestFormat_DisplayFallbackWhenPrefixDoesNotMatchValue(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "--env=production", Display: "production"}}

	formatted := NewSession().Format(`--env="pro`, wire.Directives{}, candidates)

	assert.Equal(t, "D\tnoprefix\nC\tproduction", formatted)
}

func TestFormat_CollapsesSharedNonFlagPrefix(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{
		{Value: "prod-us-east-1", Display: "prod-us-east-1"},
		{Value: "prod-eu-west-1", Display: "prod-eu-west-1"},
		{Value: "prod-ap-southeast-1", Display: "prod-ap-southeast-1"},
	}

	formatted := NewSession().Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnospace\nC\tprod-", formatted)
}

// Note: common-prefix collapsing is bash-only today (zsh manages its own insertion),
// so no cross-shell duplication is required for this behavior.
func TestFormat_CollapsesMultibyteRunePrefix(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "café-1"}, {Value: "cafè-2"}}

	formatted := NewSession().Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnospace\nC\tcaf", formatted)
}

func TestFormat_CommonValuePrefixAlwaysValidUTF8(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		toComplete string
		candidates []wire.Candidate
		expected   string
	}{
		{
			name:       "latin multi-byte split rune",
			toComplete: "",
			candidates: []wire.Candidate{{Value: "café-1"}, {Value: "cafè-2"}},
			expected:   "D\tnospace\nC\tcaf",
		},
		{
			name:       "latin multi-byte shared rune",
			toComplete: "",
			candidates: []wire.Candidate{{Value: "café-alpha"}, {Value: "café-beta"}},
			expected:   "D\tnospace\nC\tcafé-",
		},
		{
			name:       "cjk shared rune stem",
			toComplete: "",
			candidates: []wire.Candidate{{Value: "東京-east"}, {Value: "東京-west"}},
			expected:   "D\tnospace\nC\t東京-",
		},
		{
			name:       "cjk split rune stem",
			toComplete: "",
			candidates: []wire.Candidate{{Value: "東京1"}, {Value: "東洋2"}},
			expected:   "D\tnospace\nC\t東",
		},
		{
			name:       "emoji shared stem",
			toComplete: "",
			candidates: []wire.Candidate{{Value: "🚀-launch"}, {Value: "🚀-orbit"}},
			expected:   "D\tnospace\nC\t🚀-",
		},
		{
			name:       "umlaut partial match",
			toComplete: "",
			candidates: []wire.Candidate{{Value: "über-1"}, {Value: "übung-2"}},
			expected:   "D\tnospace\nC\tüb",
		},
		{
			name:       "no common stem beyond typed",
			toComplete: "caf",
			candidates: []wire.Candidate{{Value: "café-1"}, {Value: "cafè-2"}},
			expected:   "D\t\nC\tcafé-1\nC\tcafè-2",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			formatted := NewSession().Format(testCase.toComplete, wire.Directives{}, testCase.candidates)

			assert.Equal(t, testCase.expected, formatted)
			assert.True(t, utf8.ValidString(formatted), "formatted output must be valid UTF-8")
		})
	}
}

func TestCommonValuePrefix_AlwaysValidUTF8(t *testing.T) {
	t.Parallel()

	candidatePairs := [][]wire.Candidate{
		{{Value: "café-1"}, {Value: "cafè-2"}},
		{{Value: "café-alpha"}, {Value: "café-beta"}},
		{{Value: "東京-east"}, {Value: "東京-west"}},
		{{Value: "東京1"}, {Value: "東洋2"}},
		{{Value: "🚀-launch"}, {Value: "🚀-land"}},
		{{Value: "über-1"}, {Value: "übel-2"}},
		{{Value: "naïve-1"}, {Value: "naïve-2"}},
		{{Value: "naïve"}, {Value: "naïf"}},
		{{Value: "résumé-draft"}, {Value: "résumé-final"}},
	}

	for _, pair := range candidatePairs {
		prefix := commonValuePrefix(pair, "")

		assert.True(t, utf8.ValidString(prefix), "commonValuePrefix must be valid UTF-8")
	}
}

func TestFormat_DoesNotCollapseFlags(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "--env"}, {Value: "--env-vars"}}

	formatted := NewSession().Format("--e", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\t--env\nC\t--env-vars", formatted)
}

func TestFormat_DoesNotCollapseWhenPrefixFullyTyped(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "prod-east"}, {Value: "prod-west"}}

	formatted := NewSession().Format("prod-", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\tprod-east\nC\tprod-west", formatted)
}

func TestFormat_DoesNotCollapseUnsharedValues(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "staging"}, {Value: "production"}, {Value: "qa"}}

	formatted := NewSession().Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\tstaging\nC\tproduction\nC\tqa", formatted)
}

func TestFormat_SuccessiveTabsRenderTriplets(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service to a cluster"},
		{Value: "logs", Display: "logs", Description: "Stream service logs"},
		{Value: "cp", Display: "cp"},
	}

	session := sessionWithEnvironment(map[string]string{"COMP_TYPE": "63"})
	formatted := session.Format("", wire.Directives{}, candidates)

	assert.Equal(t,
		"D\tnospace\nC\tdeploy\tdeploy\tDeploy service to a cluster\nC\tlogs\tlogs\tStream service logs\nC\tcp\tcp\t", //nolint:dupword // the bash candidate triplet legitimately repeats display and value fields
		formatted)
}

func TestFormat_SuccessiveTabsRequireMultipleCandidates(t *testing.T) {
	t.Parallel()

	session := sessionWithEnvironment(map[string]string{"COMP_TYPE": "63"})
	formatted := session.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Description: "Deploy"}})

	assert.Equal(t, "D\t\nC\tdeploy", formatted)
}

func TestFormat_SuccessiveTabsIgnoreNormalModeWithoutCompType(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "deploy"}, {Value: "logs"}}
	session := sessionWithEnvironment(map[string]string{"COMP_TYPE": "9"})
	formatted := session.Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\tdeploy\nC\tlogs", formatted)
}

func TestFormat_UsesSessionCompType(t *testing.T) {
	t.Parallel()

	line := "cloudctl deploy --labels "
	session := sessionWithEnvironment(map[string]string{
		"COMP_TYPE":  "63",
		"COMP_LINE":  line,
		"COMP_POINT": strconv.Itoa(len(line)),
	})

	_, err := session.Patch(nil)
	assert.NoError(t, err)
	assert.Equal(t, "63", session.compType)

	assert.True(t, session.isSuccessiveTabs(2), "a Patch pass must feed Format the readline COMP_TYPE")

	directSession := sessionWithEnvironment(map[string]string{"COMP_TYPE": "63"})
	assert.True(t, directSession.isSuccessiveTabs(2), "without a Patch pass Format reads COMP_TYPE from the environment")
}

func TestFormat_CollapseForcesNospaceBeyondSuffixes(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "prod-east"}, {Value: "prod-west"}}

	formatted := NewSession().Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnospace\nC\tprod-", formatted)
}
