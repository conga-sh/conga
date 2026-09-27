package powershell_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/powershell"
)

func TestSnippet_SingleCommand(t *testing.T) {
	t.Parallel()

	snippet := powershell.Snippet("myapp", nil)

	assert.Contains(t, snippet, "using namespace System.Management.Automation\nusing namespace System.Management.Automation.Language\n")
	assert.Contains(t, snippet, "Function _myapp_completer {")
	assert.Contains(t, snippet, "param($wordToComplete, $commandAst, $cursorPosition)")
	assert.Contains(t, snippet, `& $elems[0] __complete powershell $words ''`)
	assert.Contains(t, snippet, `& $elems[0] __complete powershell $words`)
	assert.Contains(t, snippet, "ConvertFrom-Json).candidates | ForEach-Object")
	assert.Contains(t, snippet, "[CompletionResult]::new($_.value, $_.display.replace('`e[', ($esc + '['))")
	assert.Contains(t, snippet, `Register-ArgumentCompleter -Native -ScriptBlock (Get-Item "Function:_myapp_completer").ScriptBlock -CommandName 'myapp' # 'myapp.exe'`)
}

func TestSnippet_LegacyProtocolRemoved(t *testing.T) {
	t.Parallel()

	snippet := powershell.Snippet("myapp", []string{"ma", "app"})

	assert.False(t, strings.Contains(snippet, "CONGA_"), "the shell is passed via argv, not the environment")
}

func TestSnippet_MultiCommand(t *testing.T) {
	t.Parallel()

	snippet := powershell.Snippet("myapp", []string{"ma", "app"})

	assert.Contains(t, snippet, "$_myapp_completer = {")
	assert.Contains(t, snippet, `& $elems[0] __complete powershell $words ''`)
	assert.Contains(t, snippet, "Register-ArgumentCompleter -Native -ScriptBlock $_myapp_completer -CommandName 'myapp' # 'myapp.exe'\n")
	assert.Contains(t, snippet, "Register-ArgumentCompleter -Native -ScriptBlock $_myapp_completer -CommandName 'ma' # 'ma.exe'\n")
	assert.Contains(t, snippet, "Register-ArgumentCompleter -Native -ScriptBlock $_myapp_completer -CommandName 'app' # 'app.exe'\n")
}

func TestSnippet_SuppressesDefaultFileCompletion(t *testing.T) {
	t.Parallel()

	snippet := powershell.Snippet("myapp", nil)

	assert.Contains(t, snippet, "if (!$payload) {")
	assert.Contains(t, snippet, `return ""`)
	assert.Contains(t, snippet, "if ($completions.Count -eq 0) {")
}

func TestSnippet_QuotesMetacharacterNames(t *testing.T) {
	t.Parallel()

	snippet := powershell.Snippet("my app", nil)

	assert.Contains(t, snippet, "Function _my_app_completer {")
	assert.Contains(t, snippet, `& $elems[0] __complete powershell $words ''`)
	assert.Contains(t, snippet, `-CommandName 'my app' # 'my app.exe'`)
}

func TestSnippet_PortableAnsiEscape(t *testing.T) {
	t.Parallel()

	snippet := powershell.Snippet("myapp", nil)

	assert.Contains(t, snippet, "$esc = [char]27")
	assert.Contains(t, snippet, ".replace('`e[', ($esc + '['))")
	assert.False(t, strings.Contains(snippet, "\"`e[\""), "double-quoted `e escape is PS 6.0+ only; PS 5.1 compatibility requires [char]27")
}
