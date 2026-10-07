package bash //nolint:testpackage // exercises the unexported patch-to-format presentation state channel

import (
	"os"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
)

// resetPresentationState restores the patch-to-format state channel between
// tests: while Patch zeroes the process-global presentation state at entry in
// production, tests invoking Format directly or configuring synthetic state
// use this helper to guarantee clean test isolation.
func resetPresentationState(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		presentationState.active = false
		presentationState.compType = ""
		presentationState.wordbreakPrefix = ""
	})
}

func TestFormat_DirectiveHeaderAlwaysFirst(t *testing.T) {
	t.Parallel()

	formatted := Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy"}})

	assert.Equal(t, "D\t\nC\tdeploy", formatted)
}

func TestFormat_MessageLinesFollowHeader(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{Messages: []string{"usage: app <command>"}}
	formatted := Format("", directives, nil)

	assert.Equal(t, "D\t\nM\tusage: app <command>", formatted)
}

func TestFormat_KeepOrderToken(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{KeepOrder: true}
	formatted := Format("", directives, []wire.Candidate{{Value: "deploy"}})

	assert.Equal(t, "D\tkeeporder\nC\tdeploy", formatted)
}

func TestFormat_NospaceTokenForValueSuffixes(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "owner="}, {Value: "team="}}
	formatted := Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnospace\nC\towner=\nC\tteam=", formatted)
}

func TestFormat_NospaceTokenForDirectorySuffix(t *testing.T) {
	t.Parallel()

	formatted := Format("", wire.Directives{}, []wire.Candidate{{Value: "build/"}})

	assert.Equal(t, "D\tnospace\nC\tbuild/", formatted)
}

func TestFormat_NospaceTokenFromEngineSet(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{NoSpace: wire.SuffixSet{}}
	directives.NoSpace.Add('*')

	formatted := Format("", directives, []wire.Candidate{{Value: "deploy"}})

	assert.Equal(t, "D\tnospace\nC\tdeploy", formatted)
}

func TestFormat_NoprefixTokenForAttachedValues(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{
		{Value: "--env=staging", Display: "staging"},
		{Value: "--env=production", Display: "production"},
	}

	formatted := Format("--env=", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnoprefix\nC\tstaging\nC\tproduction", formatted)
}

func TestFormat_QuotesValuesWithMetacharacters(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{{Value: "two words"}, {Value: `a"b$c\`}}
	formatted := Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\t\"two words\"\nC\t\"a\\\"b\\$c\\\\\"", formatted)
}

func TestFormat_QuotesMidWordMetacharacters(t *testing.T) {
	t.Parallel()

	formatted := Format("", wire.Directives{}, []wire.Candidate{{Value: "a;b|c&d"}})

	assert.Equal(t, "D\t\nC\t\"a;b|c&d\"", formatted)
}

func TestFormat_BackslashEscapesTildeLeadingValues(t *testing.T) {
	t.Parallel()

	formatted := Format("", wire.Directives{}, []wire.Candidate{{Value: "~/my dir/config"}})

	assert.Equal(t, "D\t\nC\t~/my\\ dir/config", formatted)
}

func TestFormat_SanitizesFramingCharactersInValues(t *testing.T) {
	t.Parallel()

	formatted := Format("", wire.Directives{}, []wire.Candidate{{Value: "with\ttab\nnewline"}})

	assert.Equal(t, "D\t\nC\t\"with tab newline\"", formatted)
}

//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
func TestFormat_StripsWordbreakPrefixDerivedFromToComplete(t *testing.T) {
	resetPresentationState(t)

	candidates := []wire.Candidate{{Value: "team=core", Display: "core"}}

	formatted := Format("team=", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnoprefix\nC\tcore", formatted)
}

//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
func TestFormat_DisplayFallbackWhenPrefixDoesNotMatchValue(t *testing.T) {
	resetPresentationState(t)

	candidates := []wire.Candidate{{Value: "--env=production", Display: "production"}}

	formatted := Format(`--env="pro`, wire.Directives{}, candidates)

	assert.Equal(t, "D\tnoprefix\nC\tproduction", formatted)
}

//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
func TestFormat_CollapsesSharedNonFlagPrefix(t *testing.T) {
	resetPresentationState(t)

	candidates := []wire.Candidate{
		{Value: "prod-us-east-1", Display: "prod-us-east-1"},
		{Value: "prod-eu-west-1", Display: "prod-eu-west-1"},
		{Value: "prod-ap-southeast-1", Display: "prod-ap-southeast-1"},
	}

	formatted := Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnospace\nC\tprod-", formatted)
}

// Note: common-prefix collapsing is bash-only today (zsh manages its own insertion),
// so no cross-shell duplication is required for this behavior.
//
//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
func TestFormat_CollapsesMultibyteRunePrefix(t *testing.T) {
	resetPresentationState(t)

	candidates := []wire.Candidate{{Value: "café-1"}, {Value: "cafè-2"}}

	formatted := Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnospace\nC\tcaf", formatted)
}

//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
func TestFormat_CommonValuePrefixAlwaysValidUTF8(t *testing.T) {
	resetPresentationState(t)

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

	//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			formatted := Format(testCase.toComplete, wire.Directives{}, testCase.candidates)

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

//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
func TestFormat_DoesNotCollapseFlags(t *testing.T) {
	resetPresentationState(t)

	candidates := []wire.Candidate{{Value: "--env"}, {Value: "--env-vars"}}

	formatted := Format("--e", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\t--env\nC\t--env-vars", formatted)
}

//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
func TestFormat_DoesNotCollapseWhenPrefixFullyTyped(t *testing.T) {
	resetPresentationState(t)

	candidates := []wire.Candidate{{Value: "prod-east"}, {Value: "prod-west"}}

	formatted := Format("prod-", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\tprod-east\nC\tprod-west", formatted)
}

//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
func TestFormat_DoesNotCollapseUnsharedValues(t *testing.T) {
	resetPresentationState(t)

	candidates := []wire.Candidate{{Value: "staging"}, {Value: "production"}, {Value: "qa"}}

	formatted := Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\tstaging\nC\tproduction\nC\tqa", formatted)
}

func TestFormat_SuccessiveTabsRenderTriplets(t *testing.T) {
	resetPresentationState(t)
	t.Setenv("COMP_TYPE", "63")

	candidates := []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service to a cluster"},
		{Value: "logs", Display: "logs", Description: "Stream service logs"},
		{Value: "cp", Display: "cp"},
	}

	formatted := Format("", wire.Directives{}, candidates)

	assert.Equal(t,
		"D\tnospace\nC\tdeploy\tdeploy\tDeploy service to a cluster\nC\tlogs\tlogs\tStream service logs\nC\tcp\tcp\t", //nolint:dupword // the bash candidate triplet legitimately repeats display and value fields
		formatted)
}

func TestFormat_SuccessiveTabsRequireMultipleCandidates(t *testing.T) {
	resetPresentationState(t)
	t.Setenv("COMP_TYPE", "63")

	formatted := Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Description: "Deploy"}})

	assert.Equal(t, "D\t\nC\tdeploy", formatted)
}

func TestFormat_SuccessiveTabsIgnoreNormalModeWithoutCompType(t *testing.T) {
	resetPresentationState(t)
	t.Setenv("COMP_TYPE", "9")

	candidates := []wire.Candidate{{Value: "deploy"}, {Value: "logs"}}

	formatted := Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\t\nC\tdeploy\nC\tlogs", formatted)
}

func TestFormat_UsesPatchStateForCompType(t *testing.T) {
	resetPresentationState(t)
	t.Setenv("COMP_TYPE", "9")
	t.Setenv("COMP_LINE", "cloudctl deploy --labels ")
	t.Setenv("COMP_POINT", strconv.Itoa(len("cloudctl deploy --labels ")))

	_, err := Patch(nil)
	assert.NoError(t, err)

	candidates := []wire.Candidate{{Value: "owner="}, {Value: "team="}}

	formatted := Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnospace\nC\towner=\nC\tteam=", formatted)
	assert.False(t, isSuccessiveTabs(2), "COMP_TYPE consumed by Patch must not leak into the mode")
	assert.Equal(t, "", os.Getenv("COMP_LINE"), "Patch must unset the bash-native environment")
}

//nolint:paralleltest // restores the process-global presentation state that parallel Format tests read
func TestFormat_CollapseForcesNospaceBeyondSuffixes(t *testing.T) {
	resetPresentationState(t)

	candidates := []wire.Candidate{{Value: "prod-east"}, {Value: "prod-west"}}

	formatted := Format("", wire.Directives{}, candidates)

	assert.Equal(t, "D\tnospace\nC\tprod-", formatted)
}
