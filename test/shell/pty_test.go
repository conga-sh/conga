//go:build shelltest

package shell_test

import (
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

	workspace := setupCloudctl(t)

	return workspace.StartPtySession(t, "cloudctl", shellName)
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
