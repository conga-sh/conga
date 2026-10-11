package bash //nolint:testpackage // exercises the unexported session presentation state

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/sentinel"
)

func sessionWithEnvironment(environment map[string]string) *Session {
	return NewSessionWithEnv(func(key string) (string, bool) {
		value, ok := environment[key]

		return value, ok
	})
}

func TestPatch_StashesWordbreakPrefixFromRawLine(t *testing.T) {
	t.Parallel()

	line := `cloudctl deploy --env="pro`
	session := sessionWithEnvironment(map[string]string{
		"COMP_LINE":  line,
		"COMP_POINT": strconv.Itoa(len(line)),
	})

	patched, err := session.Patch([]string{"deploy", "--env=pro"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--env=pro"}, patched)
	assert.True(t, session.active)
	assert.Equal(t, `--env="`, session.wordbreakPrefix)
	assert.Equal(t, "", session.compType, "unset COMP_TYPE must stash the empty raw value")
}

func TestPatch_StashesCompType(t *testing.T) {
	t.Parallel()

	line := "cloudctl deploy --labels team="
	session := sessionWithEnvironment(map[string]string{
		"COMP_LINE":  line,
		"COMP_POINT": strconv.Itoa(len(line)),
		"COMP_TYPE":  "63",
	})

	patched, err := session.Patch([]string{"deploy", "--labels", "team="})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--labels", "team="}, patched)
	assert.Equal(t, "63", session.compType)
	assert.Equal(t, "team=", session.wordbreakPrefix)
}

func TestPatch_UnparseableCompPointFallsBackToLineEnd(t *testing.T) {
	t.Parallel()

	session := sessionWithEnvironment(map[string]string{
		"COMP_LINE":  "cloudctl deploy --env ",
		"COMP_POINT": "not-a-number",
	})

	patched, err := session.Patch([]string{"deploy", "--env", ""})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--env", ""}, patched)
}

func TestPatch_EmptyLinePassesThrough(t *testing.T) {
	t.Parallel()

	session := sessionWithEnvironment(map[string]string{
		"COMP_LINE":  "",
		"COMP_POINT": "0",
	})

	args := []string{"deploy"}

	patched, err := session.Patch(args)

	assert.NoError(t, err)
	assert.Equal(t, args, patched)
	assert.False(t, session.active)
}

func TestPatch_MissingPointPassesThrough(t *testing.T) {
	t.Parallel()

	session := sessionWithEnvironment(map[string]string{
		"COMP_LINE": "cloudctl deploy ",
	})

	args := []string{"deploy", ""}

	patched, err := session.Patch(args)

	assert.NoError(t, err)
	assert.Equal(t, args, patched)
	assert.False(t, session.active)
}

func TestPatch_RedirectPositionLeavesSessionInactive(t *testing.T) {
	t.Parallel()

	line := "cloudctl deploy > "
	session := sessionWithEnvironment(map[string]string{
		"COMP_LINE":  line,
		"COMP_POINT": strconv.Itoa(len(line)),
	})

	patched, err := session.Patch([]string{"deploy", ">", ""})

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrRedirectTarget)
	assert.Zero(t, patched)
	assert.False(t, session.active)
}

func TestPatch_RedirectPositionWithoutTrailingSpace(t *testing.T) {
	t.Parallel()

	line := "cloudctl deploy >"
	session := sessionWithEnvironment(map[string]string{
		"COMP_LINE":  line,
		"COMP_POINT": strconv.Itoa(len(line)),
	})

	patched, err := session.Patch([]string{"deploy", ">"})

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrRedirectTarget)
	assert.Zero(t, patched)
	assert.False(t, session.active)
}

func TestPatch_RedirectPositionResetsSessionState(t *testing.T) {
	t.Parallel()

	line := "cloudctl deploy > "
	session := sessionWithEnvironment(map[string]string{
		"COMP_LINE":  line,
		"COMP_POINT": strconv.Itoa(len(line)),
	})

	patched, err := session.Patch([]string{"deploy", ">", ""})

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrRedirectTarget)
	assert.Zero(t, patched)
	assert.False(t, session.active, "session.active must be false after redirect error")
	assert.Equal(t, "", session.compType, "session.compType must be empty after redirect error")
	assert.Equal(t, "", session.wordbreakPrefix, "session.wordbreakPrefix must be empty after redirect error")
}

func TestPatch_SequentialInvocationsDoNotBleedState(t *testing.T) {
	t.Parallel()

	// Cycle 1: flag with equals sign wordbreak
	line1 := "cloudctl deploy --env=pro"
	equalsSession := sessionWithEnvironment(map[string]string{
		"COMP_LINE":  line1,
		"COMP_POINT": strconv.Itoa(len(line1)),
	})

	patched1, err := equalsSession.Patch([]string{"deploy", "--env=pro"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--env=pro"}, patched1)
	assert.Equal(t, "--env=", equalsSession.wordbreakPrefix)

	out1 := equalsSession.Format("--env=pro", wire.Directives{}, []wire.Candidate{{Value: "--env=production"}})

	assert.Contains(t, out1, "C\tproduction")

	// Cycle 2: colon wordbreak without equals
	line2 := "cloudctl deploy user:al"
	colonSession := sessionWithEnvironment(map[string]string{
		"COMP_LINE":  line2,
		"COMP_POINT": strconv.Itoa(len(line2)),
	})

	patched2, err := colonSession.Patch([]string{"deploy", "user:al"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "user:al"}, patched2)
	assert.Equal(t, "user:", colonSession.wordbreakPrefix)

	out2 := colonSession.Format("user:al", wire.Directives{}, []wire.Candidate{{Value: "user:alice"}})

	assert.Contains(t, out2, "C\talice")
	assert.False(t, strings.Contains(out2, "--env="), "cycle 2 must not bleed cycle 1 prefix")

	// Cycle 3: a fresh session with no COMP_LINE must stay inactive
	plainSession := sessionWithEnvironment(map[string]string{})

	patched3, err := plainSession.Patch([]string{"deploy", "plain"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "plain"}, patched3)
	assert.False(t, plainSession.active, "un-instrumented cycle 3 must leave the session inactive")
	assert.Equal(t, "", plainSession.wordbreakPrefix)
}

func TestPatch_CustomCompWordbreaksWithoutEquals(t *testing.T) {
	t.Parallel()

	line := "cloudctl deploy --env=pro"
	session := sessionWithEnvironment(map[string]string{
		"COMP_LINE":       line,
		"COMP_POINT":      strconv.Itoa(len(line)),
		"COMP_WORDBREAKS": " \t\n\"'><;|&(:",
	})

	patched, err := session.Patch([]string{"deploy", "--env=pro"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--env=pro"}, patched)
	assert.Equal(t, "", session.wordbreakPrefix, "prefix should be empty when '=' is not in COMP_WORDBREAKS")

	out := session.Format("--env=pro", wire.Directives{}, []wire.Candidate{{Value: "--env=production"}})

	assert.Contains(t, out, "C\t--env=production", "candidate should retain full flag when '=' is not a wordbreak")
}

func TestSession_ConcurrentInvocationsDoNotShareState(t *testing.T) {
	t.Parallel()

	type testScenario struct {
		environment map[string]string
		args        []string
		toComplete  string
		candidates  []wire.Candidate
		expected    string
		expectError error
	}

	scenarios := []testScenario{
		{
			environment: map[string]string{
				"COMP_LINE":  "cloudctl deploy --env=pro",
				"COMP_POINT": strconv.Itoa(len("cloudctl deploy --env=pro")),
			},
			args:       []string{"deploy", "--env=pro"},
			toComplete: "--env=pro",
			candidates: []wire.Candidate{{Value: "--env=production", Display: "production"}},
			expected:   "D\tnoprefix\nC\tproduction",
		},
		{
			environment: map[string]string{
				"COMP_LINE":  "cloudctl deploy user:al",
				"COMP_POINT": strconv.Itoa(len("cloudctl deploy user:al")),
			},
			args:       []string{"deploy", "user:al"},
			toComplete: "user:al",
			candidates: []wire.Candidate{{Value: "user:alice", Display: "alice"}},
			expected:   "D\tnoprefix\nC\talice",
		},
		{
			environment: map[string]string{
				"COMP_LINE":  "cloudctl deploy > ",
				"COMP_POINT": strconv.Itoa(len("cloudctl deploy > ")),
			},
			args:        []string{"deploy", ">", ""},
			expectError: sentinel.ErrRedirectTarget,
		},
		{
			environment: map[string]string{
				"COMP_LINE":  "cloudctl deploy team=",
				"COMP_POINT": strconv.Itoa(len("cloudctl deploy team=")),
				"COMP_TYPE":  "63",
			},
			args:       []string{"deploy", "team="},
			toComplete: "team=",
			candidates: []wire.Candidate{
				{Value: "team=core", Display: "core", Description: "Core team"},
				{Value: "team=ops", Display: "ops"},
			},
			expected: "D\tnoprefix nospace\nC\tcore\tteam=core\tCore team\nC\tops\tteam=ops\t",
		},
	}

	results := make([]string, len(scenarios)*10)
	patchErrors := make([]error, len(scenarios)*10)

	var waitGroup sync.WaitGroup

	for i := range results {
		scenario := scenarios[i%len(scenarios)]

		waitGroup.Go(func() {
			session := sessionWithEnvironment(scenario.environment)

			_, err := session.Patch(scenario.args)

			patchErrors[i] = err

			if err == nil {
				results[i] = session.Format(scenario.toComplete, wire.Directives{}, scenario.candidates)
			}
		})
	}

	waitGroup.Wait()

	for i, result := range results {
		scenario := scenarios[i%len(scenarios)]
		if scenario.expectError != nil {
			assert.IsError(t, patchErrors[i], scenario.expectError)
		} else {
			assert.NoError(t, patchErrors[i])
			assert.Equal(t, scenario.expected, result)
		}
	}
}
