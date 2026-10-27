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
	ptyClusterCommandLine = "cloudctl deploy --env production --cluster "
	ptyClusterCandidateUS = "prod-us-east-1"
	ptyClusterCandidateEU = "prod-eu-west-1"
	ptyClusterCandidateAP = "prod-ap-southeast-1"
	ptyClusterPrefix      = ptyClusterCommandLine + "prod-"
	ptyClusterFirst       = ptyClusterCommandLine + ptyClusterCandidateAP
)

func startCloudctlPty(t *testing.T, shellName string) *shell.PtySession {
	t.Helper()

	return setupCloudctl(t).StartPtySession(t, "cloudctl", shellName)
}

// ptyHasToken reports whether the screen contains token as a
// whitespace-separated field, so candidate aliases can be distinguished from
// longer words containing the same characters.
func ptyHasToken(screen, token string) bool {
	return slices.Contains(strings.Fields(screen), token)
}

func TestPtyBash_TabCompletesSingleMatch(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "bash")

	session.Type("cloudctl de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "cloudctl deploy", result.BufferAfterPrompt())
	assert.Equal(t, 23, result.CursorX)
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

			workspace := setupCloudctl(t)
			session := workspace.StartPtySession(t, "cloudctl", shellName)

			session.Type(workspace.BinDir + "/cloudctl de")
			session.Tab()
			result := session.Capture()

			assert.Equal(t, workspace.BinDir+"/cloudctl deploy", result.BufferAfterPrompt())
		})
	}
}

func TestPtyBash_TabInsertsCommonPrefix(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "bash")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterPrefix, result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> "+ptyClusterPrefix), result.CursorX)
	assert.False(t, strings.Contains(result.Screen, ptyClusterCandidateUS))
}

func TestPtyBash_TabListsCandidates(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "bash")

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

	session := startCloudctlPty(t, "bash")

	session.Type("cloudctl deploy --image api-gateway:v3.2.1-")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "cloudctl deploy --image api-gateway:v3.2.1-rc1", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> cloudctl deploy --image api-gateway:v3.2.1-rc1"), result.CursorX)
}

func TestPtyZsh_TabCompletesSingleMatch(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "zsh")

	session.Type("cloudctl de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "cloudctl deploy", result.BufferAfterPrompt())
	assert.Equal(t, 23, result.CursorX)
}

func TestPtyZsh_TabListsCandidatesWithoutInserting(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "zsh")

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

	session := startCloudctlPty(t, "zsh")

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

	session := startCloudctlPty(t, "zsh")

	session.Type("cloudctl deploy --image api-gateway:v3.2.1-")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "cloudctl deploy --image api-gateway:v3.2.1-rc1", result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> cloudctl deploy --image api-gateway:v3.2.1-rc1"), result.CursorX)
}

func TestPtyZsh_UserZstylesSurvive(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "zsh")

	session.Type(`zstyle ':completion:*' group-name '%d'`)
	session.PressEnter()
	session.Type(`zstyle ':completion:*' list-colors 'ln=target'`)
	session.PressEnter()

	session.Type("cloudctl deploy --")
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

	session := startCloudctlPty(t, "zsh")

	session.Type("cloudctl ")
	session.Tab()

	assert.Contains(t, session.Capture().Screen, "Deploy service to a cluster")
}

func TestPtyZsh_VerboseZstyleHidesDescriptions(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "zsh")

	session.Type(`zstyle ':completion:*' verbose false`)
	session.PressEnter()

	session.Type("cloudctl ")
	session.Tab()
	result := session.Capture()

	assert.NotContains(t, result.Screen, "Deploy service to a cluster")
	assert.True(t, ptyHasToken(result.Screen, "deploy"))
}

func TestPtyZsh_AliasShownByDefault(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "zsh")

	session.Type("cloudctl ")
	session.Tab()

	assert.True(t, ptyHasToken(session.Capture().Screen, "d"))
}

func TestPtyZsh_ShowAliasesZstyleHidesAlias(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "zsh")

	session.Type(`zstyle ':completion:*' show-aliases false`)
	session.PressEnter()

	session.Type("cloudctl ")
	session.Tab()
	result := session.Capture()

	assert.True(t, ptyHasToken(result.Screen, "deploy"))
	assert.False(t, ptyHasToken(result.Screen, "d"))
}

func TestPtyZsh_DefaultSeparatorColorsBothColumns(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "zsh")

	session.Type(`zstyle ':completion:*' show-aliases false`)
	session.PressEnter()

	session.Type("cloudctl --")
	session.Tab()
	session.Tab()
	result := session.Capture()

	assert.Contains(t, result.Screen, "-- Path to a file")
	assert.True(t, ptyStyledBefore(result.ScreenEscaped, "--kubeconfig"), "two-tone entry must color the display column")
	assert.True(t, ptyDimWithPadding(result.ScreenEscaped, "-- Path to a file"), "the description column must carry the description color")
}

func TestPtyZsh_ListSeparatorColorsBothColumns(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "zsh")

	session.Type(`zstyle ':completion:*' show-aliases false`)
	session.PressEnter()
	session.Type(`zstyle ':completion:*' list-separator '<->'`)
	session.PressEnter()

	session.Type("cloudctl --")
	session.Tab()
	session.Tab()
	result := session.Capture()

	assert.Contains(t, result.Screen, "<-> Path to a file")
	assert.True(t, ptyStyledBefore(result.ScreenEscaped, "--kubeconfig"), "two-tone entry must color the display column")
	assert.True(t, ptyDimWithPadding(result.ScreenEscaped, "<-> Path to a file"), "the description column must carry the description color")
}

func TestPtyZsh_StyleLimitCatchAllColorsDescriptions(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "zsh")

	session.Type(`zstyle ':completion:*' show-aliases false`)
	session.PressEnter()
	session.Type("export CONGA_ZSH_STYLE_LIMIT=0")
	session.PressEnter()

	session.Type("cloudctl ")
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

	session := startCloudctlPty(t, "fish")

	session.Type("cloudctl de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "cloudctl deploy", result.BufferAfterPrompt())
	assert.Equal(t, 23, result.CursorX)
}

func TestPtyFish_TabInsertsCommonPrefixAndLists(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "fish")

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

	session := startCloudctlPty(t, "fish")

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

	session := startCloudctlPty(t, "powershell")

	session.Type("cloudctl de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "cloudctl deploy", result.BufferAfterPrompt())
	assert.Equal(t, 23, result.CursorX)
}

func TestPtyPowerShell_TabInsertsCommonPrefix(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "powershell")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterPrefix, result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> "+ptyClusterPrefix), result.CursorX)
	assert.False(t, strings.Contains(result.Screen, ptyClusterCandidateUS))
}

func TestPtyPowerShell_SecondTabListsCandidates(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "powershell")

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

	session := startCloudctlPty(t, "nushell")

	session.Type("cloudctl de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "cloudctl deploy", result.BufferAfterPrompt())
	assert.Equal(t, 23, result.CursorX)
}

func TestPtyNushell_TabInsertsCommonPrefixAndOpensMenu(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "nushell")

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

	session := startCloudctlPty(t, "nushell")

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

	session := startCloudctlPty(t, "elvish")

	session.Type("cloudctl de")
	session.Tab()
	result := session.Capture()

	assert.Equal(t, "cloudctl deploy", result.BufferAfterPrompt())
	assert.Equal(t, 23, result.CursorX)
}

func TestPtyElvish_TabInsertsCommonPrefix(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "elvish")

	session.Type(ptyClusterCommandLine)
	session.Tab()
	result := session.Capture()

	assert.Equal(t, ptyClusterPrefix, result.BufferAfterPrompt())
	assert.Equal(t, len("CONGA> "+ptyClusterPrefix), result.CursorX)
	assert.False(t, strings.Contains(result.Screen, ptyClusterCandidateUS))
}

func TestPtyElvish_SecondTabOpensMenuAndInsertsFirstCandidate(t *testing.T) {
	t.Parallel()

	session := startCloudctlPty(t, "elvish")

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
