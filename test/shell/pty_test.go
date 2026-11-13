//go:build shelltest

package shell_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/test/shell"
)

// PTY completion tests drive real interactive shells inside tmux-managed
// pseudo-terminals: keys are sent as a user would type them and assertions
// run against the rendered screen and cursor position. Expectations encode
// each shell's genuine line-editor behavior, which differs from the simulated
// buffer semantics of the non-PTY harness (fish and elvish insert the first
// listed candidate on their second Tab; bash needs a third Tab to list).

const (
	ptyClusterCommandLine = "shell-completion deploy --env production --cluster "
	ptyClusterCandidateUS = "prod-us-east-1"
	ptyClusterCandidateEU = "prod-eu-west-1"
	ptyClusterCandidateAP = "prod-ap-southeast-1"
	ptyClusterPrefix      = ptyClusterCommandLine + "prod-"
	ptyClusterFirst       = ptyClusterCommandLine + ptyClusterCandidateAP
)

func startShellCompletionPty(t *testing.T, shellName string) *shell.PtySession {
	t.Helper()

	return setupShellCompletion(t).StartPtySession(t, "shell-completion", shellName)
}

// ptyHasToken reports whether the screen contains token as a
// whitespace-separated field, so candidate aliases can be distinguished from
// longer words containing the same characters.
func ptyHasToken(screen, token string) bool {
	return slices.Contains(strings.Fields(screen), token)
}

func TestPtyBash_TabCompletesSingleMatch(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "bash")

	session.Type("shell-completion de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "shell-completion deploy", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> shell-completion deploy "), result.CursorX)
}

// TestPtyPathQualifiedInvocation pins dispatch for a path-qualified command
// word in the shells that match on the command word: a full path invokes the
// completer registered under the binary's basename. Elvish keys its
// arg-completer map on the literal word and is documented as unsupported for
// path-qualified invocation (see the shell completion how-to).
func TestPtyPathQualifiedInvocation(t *testing.T) {
	t.Parallel()

	for _, shellName := range []string{"bash", "zsh", "fish", "powershell", "nushell"} {
		t.Run(shellName, func(t *testing.T) {
			t.Parallel()

			workspace := setupShellCompletion(t)
			session := workspace.StartPtySession(t, "shell-completion", shellName)

			session.Type(workspace.BinDir + "/shell-completion de")
			session.Tab()
			result := session.Capture()

			assert.Equal(t, workspace.BinDir+"/shell-completion deploy", result.BufferAfterPrompt())
		})
	}
}

func TestPtyBash_TabInsertsCommonPrefix(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "bash")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterPrefix, result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> "+ptyClusterPrefix), result.CursorX)
	assert.False(t, strings.Contains(result.Screen, ptyClusterCandidateUS))
}

func TestPtyBash_TabListsCandidates(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "bash")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	session.Capture()
	session.Tab()
	session.Capture()
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterPrefix, result.BufferAfterPrompt())
	assert.Contains(t, result.Screen, ptyClusterCandidateUS)
	assert.Contains(t, result.Screen, ptyClusterCandidateEU)
	assert.Contains(t, result.Screen, ptyClusterCandidateAP)
}

func TestPtyBash_TabCompletesColonCandidate(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "bash")

	session.Type("shell-completion deploy --image api-gateway:v3.2.1-")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "shell-completion deploy --image api-gateway:v3.2.1-rc1", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> shell-completion deploy --image api-gateway:v3.2.1-rc1"), result.CursorX)
}

func TestPtyZsh_TabCompletesSingleMatch(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type("shell-completion de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "shell-completion deploy", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> shell-completion deploy "), result.CursorX)
}

func TestPtyZsh_TabListsCandidatesWithoutInserting(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	result := session.Capture()

	assert.Equal(t, strings.TrimRight(ptyClusterCommandLine, " "), result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> "+ptyClusterCommandLine), result.CursorX)
	assert.Contains(t, result.Screen, ptyClusterCandidateUS)
	assert.Contains(t, result.Screen, ptyClusterCandidateEU)
	assert.Contains(t, result.Screen, ptyClusterCandidateAP)
}

func TestPtyZsh_SecondTabKeepsListAndBuffer(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	session.Capture()
	session.Tab()
	result := session.Capture()

	assert.Equal(t, strings.TrimRight(ptyClusterCommandLine, " "), result.BufferAfterPrompt())
	assert.Contains(t, result.Screen, ptyClusterCandidateUS)
	assert.Contains(t, result.Screen, ptyClusterCandidateEU)
	assert.Contains(t, result.Screen, ptyClusterCandidateAP)
}

func TestPtyZsh_TabCompletesColonCandidate(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type("shell-completion deploy --image api-gateway:v3.2.1-")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "shell-completion deploy --image api-gateway:v3.2.1-rc1", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> shell-completion deploy --image api-gateway:v3.2.1-rc1"), result.CursorX)
}

func TestPtyZsh_UserZstylesSurvive(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type(`zstyle ':completion:*' group-name '%d'`)
	session.PressEnter()
	session.Type(`zstyle ':completion:*' list-colors 'ln=target'`)
	session.PressEnter()

	session.Type("shell-completion deploy --")
	session.Tab()
	session.Capture()
	session.CtrlC()
	session.PressEnter()

	session.Type(`zstyle -L ':completion:*' group-name`)
	session.PressEnter()
	session.Type(`zstyle -L ':completion:*' list-colors`)
	session.PressEnter()
	result := session.Capture()

	assert.Contains(t, result.Screen, "group-name %d")
	assert.Contains(t, result.Screen, "list-colors 'ln=target'")
	assert.NotContains(t, result.Screen, "group-name ''")
	assert.NotContains(t, result.Screen, "=(#b)")
}

func TestPtyZsh_DescriptionsShownByDefault(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type("shell-completion ")
	session.Tab()

	assert.Contains(t, session.Capture().Screen, "Deploy service to a cluster")
}

func TestPtyZsh_VerboseZstyleHidesDescriptions(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type(`zstyle ':completion:*' verbose false`)
	session.PressEnter()

	session.Type("shell-completion ")
	session.Tab()
	result := session.Capture()

	assert.NotContains(t, result.Screen, "Deploy service to a cluster")
	assert.True(t, ptyHasToken(result.Screen, "deploy"))
}

func TestPtyZsh_AliasShownByDefault(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type("shell-completion ")
	session.Tab()

	assert.True(t, ptyHasToken(session.Capture().Screen, "d"))
}

func TestPtyZsh_ShowAliasesZstyleHidesAlias(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type(`zstyle ':completion:*' show-aliases false`)
	session.PressEnter()

	session.Type("shell-completion ")
	session.Tab()
	result := session.Capture()

	assert.True(t, ptyHasToken(result.Screen, "deploy"))
	assert.False(t, ptyHasToken(result.Screen, "d"))
}

func TestPtyZsh_DefaultSeparatorColorsBothColumns(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type(`zstyle ':completion:*' show-aliases false`)
	session.PressEnter()

	session.Type("shell-completion --")
	session.Tab()
	session.Tab()
	result := session.Capture()

	assert.Contains(t, result.Screen, "-- Path to a file")
	assert.True(t, ptyStyledBefore(result.ScreenEscaped, "--kubeconfig"), "two-tone entry must color the display column")
	assert.True(t, ptyDimWithPadding(result.ScreenEscaped, "-- Path to a file"), "the description column must carry the description color")
}

func TestPtyZsh_ListSeparatorColorsBothColumns(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type(`zstyle ':completion:*' show-aliases false`)
	session.PressEnter()
	session.Type(`zstyle ':completion:*' list-separator '<->'`)
	session.PressEnter()

	session.Type("shell-completion --")
	session.Tab()
	session.Tab()
	result := session.Capture()

	assert.Contains(t, result.Screen, "<-> Path to a file")
	assert.True(t, ptyStyledBefore(result.ScreenEscaped, "--kubeconfig"), "two-tone entry must color the display column")
	assert.True(t, ptyDimWithPadding(result.ScreenEscaped, "<-> Path to a file"), "the description column must carry the description color")
}

func TestPtyZsh_StyleLimitCatchAllColorsDescriptions(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "zsh")

	session.Type(`zstyle ':completion:*' show-aliases false`)
	session.PressEnter()
	session.Type("export CONGA_ZSH_STYLE_LIMIT=0")
	session.PressEnter()

	session.Type("shell-completion ")
	session.Tab()
	result := session.Capture()

	assert.Contains(t, result.Screen, "-- Stream and export service logs")
	assert.True(t, ptyDimWithPadding(result.ScreenEscaped, "-- Stream and export service logs"), "the catch-all must color the description column")
	assert.False(t, ptyStyledBefore(result.ScreenEscaped, "logs"), "no per-candidate entries are emitted when the style limit is zero")
}

// ptyStyledBefore reports whether text is immediately preceded by an SGR
// sequence in the escape form of the screen.
func ptyStyledBefore(screenEscaped, text string) bool {
	return regexp.MustCompile(`\x1b\[[0-9;]*m` + regexp.QuoteMeta(text)).MatchString(screenEscaped)
}

// ptyDimWithPadding reports whether text is preceded by the dim SGR sequence
// and at least one padding space (the description column starts after the
// aligned display column).
func ptyDimWithPadding(screenEscaped, text string) bool {
	return regexp.MustCompile(`\x1b\[2m(?:\x1b\[[0-9;]*m)* +` + regexp.QuoteMeta(text)).MatchString(screenEscaped)
}

func TestPtyFish_TabCompletesSingleMatch(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "fish")

	session.Type("shell-completion de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "shell-completion deploy", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> shell-completion deploy "), result.CursorX)
}

func TestPtyFish_TabInsertsCommonPrefixAndLists(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "fish")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterPrefix, result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> "+ptyClusterPrefix), result.CursorX)
	assert.Contains(t, result.Screen, ptyClusterCandidateUS)
	assert.Contains(t, result.Screen, ptyClusterCandidateEU)
	assert.Contains(t, result.Screen, ptyClusterCandidateAP)
}

func TestPtyFish_SecondTabInsertsFirstCandidate(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "fish")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	session.Capture()
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterFirst, result.BufferAfterPrompt())
	assert.Contains(t, result.Screen, ptyClusterCandidateUS)
	assert.Contains(t, result.Screen, ptyClusterCandidateEU)
	assert.Contains(t, result.Screen, ptyClusterCandidateAP)
}

func TestPtyPowerShell_TabCompletesSingleMatch(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "powershell")

	session.Type("shell-completion de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "shell-completion deploy", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> shell-completion deploy "), result.CursorX)
}

func TestPtyPowerShell_TabInsertsCommonPrefix(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "powershell")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterPrefix, result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> "+ptyClusterPrefix), result.CursorX)
	assert.False(t, strings.Contains(result.Screen, ptyClusterCandidateUS))
}

func TestPtyPowerShell_SecondTabListsCandidates(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "powershell")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	session.Capture()
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterPrefix, result.BufferAfterPrompt())
	assert.Contains(t, result.Screen, ptyClusterCandidateUS)
	assert.Contains(t, result.Screen, ptyClusterCandidateEU)
	assert.Contains(t, result.Screen, ptyClusterCandidateAP)
}

func TestPtyNushell_TabCompletesSingleMatch(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "nushell")

	session.Type("shell-completion de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "shell-completion deploy", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> shell-completion deploy "), result.CursorX)
}

func TestPtyNushell_TabInsertsCommonPrefixAndOpensMenu(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "nushell")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	result := session.Capture()

	// Reedline renders a "|" insertion marker inside the buffer line while
	// the menu is open, so assert containment rather than equality.
	assert.Contains(t, result.BufferAfterPrompt(), ptyClusterPrefix)
	assert.Contains(t, result.Screen, ptyClusterCandidateUS)
	assert.Contains(t, result.Screen, ptyClusterCandidateEU)
	assert.Contains(t, result.Screen, ptyClusterCandidateAP)
}

func TestPtyNushell_SecondTabKeepsMenuAndBuffer(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "nushell")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	session.Capture()
	session.Tab()
	result := session.Capture()

	assert.Contains(t, result.BufferAfterPrompt(), ptyClusterPrefix)
	assert.False(t, strings.Contains(result.BufferAfterPrompt(), ptyClusterCandidateAP))
	assert.Contains(t, result.Screen, ptyClusterCandidateUS)
	assert.Contains(t, result.Screen, ptyClusterCandidateEU)
	assert.Contains(t, result.Screen, ptyClusterCandidateAP)
}

func TestPtyElvish_TabCompletesSingleMatch(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "elvish")

	session.Type("shell-completion de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "shell-completion deploy", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> shell-completion deploy "), result.CursorX)
}

func TestPtyElvish_TabInsertsCommonPrefix(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "elvish")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterPrefix, result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> "+ptyClusterPrefix), result.CursorX)
	assert.False(t, strings.Contains(result.Screen, ptyClusterCandidateUS))
}

func TestPtyElvish_SecondTabOpensMenuAndInsertsFirstCandidate(t *testing.T) {
	t.Parallel()

	session := startShellCompletionPty(t, "elvish")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	session.Capture()
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterFirst, result.BufferAfterPrompt())
	assert.Contains(t, result.Screen, ptyClusterCandidateUS)
	assert.Contains(t, result.Screen, ptyClusterCandidateEU)
	assert.Contains(t, result.Screen, ptyClusterCandidateAP)
}

// ptyTokenOrder returns the given tokens in the order they appear on screen
// as whitespace-separated fields.
func ptyTokenOrder(screen string, tokens ...string) []string {
	return slices.DeleteFunc(strings.Fields(screen), func(field string) bool {
		return !slices.Contains(tokens, field)
	})
}

// TestPtyKeepOrder pins the candidate order bash and zsh show on screen: a
// KeepOrder predictor (logs --severity) keeps its declared order, and a plain
// enum (logs --env, declared staging, production, qa) is sorted by the shell.
func TestPtyKeepOrder(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		shell    string
		line     string
		tabs     int
		tokens   []string
		expected []string
	}{
		"bash keeporder":  {"bash", "shell-completion logs --severity ", 2, []string{"debug", "info", "warn", "error"}, []string{"debug", "info", "warn", "error"}},
		"bash plain enum": {"bash", "shell-completion logs --env ", 2, []string{"staging", "production", "qa"}, []string{"production", "qa", "staging"}},
		"zsh keeporder":   {"zsh", "shell-completion logs --severity ", 1, []string{"debug", "info", "warn", "error"}, []string{"debug", "info", "warn", "error"}},
		"zsh plain enum":  {"zsh", "shell-completion logs --env ", 1, []string{"staging", "production", "qa"}, []string{"production", "qa", "staging"}},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			session := startShellCompletionPty(t, testCase.shell)

			session.Type(testCase.line)

			var result *shell.PtyResult

			for range testCase.tabs {
				session.Tab()
				result = session.Capture()
			}

			assert.Equal(t, testCase.expected, ptyTokenOrder(result.Screen, testCase.tokens...))
		})
	}
}

// TestPtyZshOrder pins the declared order on screen under the zsh
// styles that make _describe sort or regroup: verbose set to false, and the
// default list-grouped with candidates sharing a description.
func TestPtyZshOrder(t *testing.T) {
	t.Parallel()

	t.Run("verbose no", func(t *testing.T) {
		t.Parallel()

		session := startShellCompletionPty(t, "zsh")
		session.Type("zstyle ':completion:*' verbose no")
		session.PressEnter()
		session.Capture()
		session.Type("shell-completion logs --severity ")

		session.Tab()
		result := session.Capture()

		assert.Equal(t, []string{"debug", "info", "warn", "error"}, ptyTokenOrder(result.Screen, "debug", "info", "warn", "error"))
	})

	t.Run("shared descriptions", func(t *testing.T) {
		t.Parallel()

		session := setupOrdertool(t).StartPtySession(t, "ordertool", "zsh")
		session.Type("ordertool --level ")

		session.Tab()
		result := session.Capture()

		assert.Equal(t, []string{"debug", "info", "warn", "error"}, ptyTokenOrder(result.Screen, "debug", "info", "warn", "error"))
	})
}

// TestPtyWordHandling pins the bash and zsh buffers for values containing a
// space: a shared prefix inserts an escaped space and typing on reaches the
// directory, an open quote is continued and closed without doubling, a
// quoted directory keeps its path when completing inside it, and a non-ASCII
// word before the cursor in a UTF-8 locale does not shift the cursor offset.
// A backslash typed inside a double quote reaches zsh's PREFIX already
// backslash-quoted, which the zsh harness does not model, and a '!' inside
// double quotes keeps its backslash only while banghist is on.
func TestPtyWordHandling(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		shell    string
		utf8     bool
		options  string
		typed    []string
		expected string
	}{
		"bash shared prefix then directory":     {"bash", false, "", []string{"wordtool --kubeconfig My", "D"}, `wordtool --kubeconfig My\ Dir/`},
		"bash open quote value":                 {"bash", false, "", []string{`wordtool --env "m`}, `wordtool --env "my env"`},
		"bash inside quoted directory":          {"bash", false, "", []string{`wordtool --kubeconfig "My Dir/i`}, `wordtool --kubeconfig "My Dir/inner.yaml"`},
		"bash non-ascii word before cursor":     {"bash", true, "", []string{"wordtool --kubeconfig héé --env p"}, "wordtool --kubeconfig héé --env prod"},
		"zsh shared prefix then directory":      {"zsh", false, "", []string{"wordtool --kubeconfig My", "D"}, `wordtool --kubeconfig My\ Dir/`},
		"zsh open quote value":                  {"zsh", false, "", []string{`wordtool --env "m`}, `wordtool --env "my env"`},
		"zsh inside quoted directory":           {"zsh", false, "", []string{`wordtool --kubeconfig "My Dir/i`}, `wordtool --kubeconfig "My Dir/inner.yaml"`},
		"zsh non-ascii word before cursor":      {"zsh", true, "", []string{"wordtool --kubeconfig héé --env p"}, "wordtool --kubeconfig héé --env prod"},
		"zsh open quote keeps a backslash":      {"zsh", false, "", []string{`wordtool --kubeconfig "back\s`}, `wordtool --kubeconfig "back\\slash.yaml"`},
		"zsh open quote escapes a history bang": {"zsh", false, "", []string{`wordtool --env "a`}, `wordtool --env "a\!b"`},
		"zsh open quote bang under nobanghist":  {"zsh", false, "setopt nobanghist", []string{`wordtool --env "a`}, `wordtool --env "a!b"`},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			workspace := setupWordtool(t)
			session := workspace.StartPtySession(t, "wordtool", testCase.shell)

			setup := "cd '" + workspace.WorkDir + "'"
			if testCase.utf8 {
				setup += " && export LC_ALL=C.UTF-8"
			}

			if testCase.options != "" {
				setup += " && " + testCase.options
			}

			session.Type(setup + " && clear")
			session.PressEnter()
			session.Capture()

			for _, text := range testCase.typed {
				session.Type(text)
				session.Tab()
			}

			result := session.Capture()

			assert.Equal(t, testCase.expected, result.BufferAfterPrompt())
		})
	}
}
