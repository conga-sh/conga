//go:build shelltest

package shell_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

// TestWordHandlingSymmetry pins completion of a value containing a space or
// a history '!' in bash, zsh, fish, and PowerShell: an escaped space, an
// open quote, and a shared prefix that ends in a space. The bash harness
// buffer is simulated from a naive word split, so bash asserts the inserted
// text (COMPREPLY); the PTY tests check the bash and zsh buffers on screen.
// PowerShell's escape character is the backtick, so `My\ D` is not a
// PowerShell spelling and only the quoted forms run there.
func TestWordHandlingSymmetry(t *testing.T) {
	t.Parallel()

	workspace := setupWordtool(t)

	t.Run("bash escaped space completes the directory", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "wordtool", `wordtool --kubeconfig My\ D`)

		assert.Equal(t, []string{`My\ Dir/`}, result.Items)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("bash shared prefix inserts an escaped space", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "wordtool", `wordtool --kubeconfig My`)

		assert.Equal(t, []string{`My\ `}, result.Items)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("bash open double quote leaves a directory open", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "wordtool", `wordtool --kubeconfig "My D`)

		assert.Equal(t, []string{"My Dir/"}, result.Items)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("bash open double quote closes after a file", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "wordtool", `wordtool --kubeconfig "My F`)

		assert.Equal(t, []string{`My File.yaml"`}, result.Items)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("bash open single quote closes after a value", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "wordtool", `wordtool --env 'my`)

		assert.Equal(t, []string{`my env'`}, result.Items)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("bash history bang is escaped", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "wordtool", `wordtool --env a`)

		assert.Equal(t, []string{`a\!b`}, result.Items)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("bash multibyte text before the cursor in a UTF-8 locale", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletionWithSetup(t, "wordtool", `wordtool --kubeconfig héé --env p`, "export LC_ALL=C.UTF-8")

		assert.Equal(t, []string{"prod"}, result.Items)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("zsh escaped space completes the directory", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "wordtool", `wordtool --kubeconfig My\ D`)

		assert.Equal(t, `wordtool --kubeconfig My\ Dir/`, result.BufferTab1)
	})

	t.Run("zsh shared prefix inserts an escaped space", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "wordtool", `wordtool --kubeconfig My`)

		assert.Equal(t, `wordtool --kubeconfig My\ `, result.BufferTab1)
	})

	t.Run("zsh open double quote leaves a directory open", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "wordtool", `wordtool --kubeconfig "My D`)

		assert.Equal(t, `wordtool --kubeconfig "My Dir/`, result.BufferTab1)
	})

	t.Run("zsh open double quote closes after a file", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "wordtool", `wordtool --kubeconfig "My F`)

		assert.Equal(t, `wordtool --kubeconfig "My File.yaml" `, result.BufferTab1)
	})

	t.Run("zsh open single quote closes after a value", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "wordtool", `wordtool --env 'my`)

		assert.Equal(t, `wordtool --env 'my env' `, result.BufferTab1)
	})

	t.Run("zsh history bang is escaped", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "wordtool", `wordtool --env a`)

		assert.Equal(t, `wordtool --env a\!b `, result.BufferTab1)
	})

	t.Run("fish escaped space completes the directory", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "wordtool", `wordtool --kubeconfig My\ D`)

		assert.Equal(t, []string{"My Dir/"}, candidateValues(result.Items))
		assert.Equal(t, "", result.Stderr, "completion must not write to stderr")
	})

	t.Run("fish shared prefix lists both paths", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "wordtool", `wordtool --kubeconfig My`)

		assert.Equal(t, []string{"My Dir/", "My File.yaml"}, candidateValues(result.Items))
		assert.Equal(t, "", result.Stderr, "completion must not write to stderr")
	})

	t.Run("fish open double quote completes the directory", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "wordtool", `wordtool --kubeconfig "My D`)

		assert.Equal(t, []string{"My Dir/"}, candidateValues(result.Items))
		assert.Equal(t, "", result.Stderr, "completion must not write to stderr")
	})

	t.Run("fish trailing backslash keeps the word being completed", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "wordtool", `wordtool --kubeconfig back\`)

		assert.Equal(t, []string{`back\slash.yaml`}, candidateValues(result.Items))
		assert.Equal(t, "", result.Stderr, "completion must not write to stderr")
	})

	t.Run("fish open single quote completes the value", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "wordtool", `wordtool --env 'my`)

		assert.Equal(t, []string{"my env"}, candidateValues(result.Items))
		assert.Equal(t, "", result.Stderr, "completion must not write to stderr")
	})

	t.Run("powershell shared prefix lists both paths quoted", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "wordtool", `wordtool --kubeconfig My`)

		assert.Equal(t, []string{"'My Dir/'", "'My File.yaml'"}, powerShellValues(result.Items))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("powershell open double quote replaces the quoted word", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "wordtool", `wordtool --kubeconfig "My D`)

		assert.Equal(t, "wordtool --kubeconfig 'My Dir/'", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("powershell open single quote replaces the quoted word", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "wordtool", `wordtool --env 'my`)

		assert.Equal(t, "wordtool --env 'my env' ", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})
}
