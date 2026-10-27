package nushell_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/nushell"
)

func TestSnippet_GlobalExternalCompleter(t *testing.T) {
	t.Parallel()

	script := nushell.Snippet("myapp", []string{"ma", "app"}, "")

	assert.Contains(t, script, "let myapp_completer = {|spans|")
	assert.Contains(t, script, "if ($spans | length) > 0 {")
	assert.Contains(t, script, "try {")
	assert.Contains(t, script, "^$spans.0 __complete nushell ...$words")
	assert.Contains(t, script, "from json")
	assert.Contains(t, script, "} catch { }")
	assert.NotContains(t, script, "arg-completer", "nushell registers one global completer, not per-name")
}

func TestSnippet_SetsSidebandShellVariable(t *testing.T) {
	t.Parallel()

	assert.Contains(t, nushell.Snippet("myapp", nil, ""), "load-env { CONGA_SHELL: 'nushell' }")
}

func TestSnippet_DefensiveConfigMerge(t *testing.T) {
	t.Parallel()

	script := nushell.Snippet("myapp", nil, "")

	assert.Contains(t, script, "mut current = (($env | default {} config).config | default {} completions)")
	assert.Contains(t, script, "$current.completions = ($current.completions | default {} external)")
	assert.Contains(t, script, "| default true enable")
	assert.Contains(t, script, "|# backwards compatible workaround for default, see nushell #15654")
	assert.Contains(t, script, "| upsert completer { if $in == null { $myapp_completer } else { $in } })")
	assert.Contains(t, script, "$env.config = $current")
}

func TestSnippet_ValidatesSpanCount(t *testing.T) {
	t.Parallel()

	// Defensive guard: check span count before indexing
	assert.Contains(t, nushell.Snippet("myapp", nil, ""), "if ($spans | length) > 0 {")
}

func TestSnippet_ErrorHandlingViaTryCatch(t *testing.T) {
	t.Parallel()

	script := nushell.Snippet("myapp", nil, "")

	// Wraps completion invocation in try-catch for robustness
	assert.Contains(t, script, "try {")
	assert.Contains(t, script, "} catch { }")
}

func TestSnippet_SanitizesIdentifierAndTarget(t *testing.T) {
	t.Parallel()

	script := nushell.Snippet("my-app", nil, "")

	assert.Contains(t, script, "let my_app_completer = {|spans|")
	assert.Contains(t, script, "$my_app_completer")

	typed := nushell.Snippet("my app", nil, "")

	assert.Contains(t, typed, "if ($spans | length) > 0 {")
	assert.Contains(t, typed, `^$spans.0 __complete nushell ...$words | from json`)
	assert.NotContains(t, typed, `"my app" `, "invocation target is the typed span, not the raw name")
}

// TestSnippet_IdentifierHygiene pins the quoting contract for hostile command
// names: the `let` identifier is always a sanitized identifier, the
// completer invokes the command name the user typed via $spans.0 (never the
// raw generation-time name), defensive error handling guards span count and
// wraps invocation in try-catch, and when sanitization rewrote the identifier
// the raw name never appears anywhere in the script.
func TestSnippet_IdentifierHygiene(t *testing.T) {
	t.Parallel()

	for label, testCase := range map[string]struct {
		name       string
		identifier string
	}{
		"plain word":   {name: "myapp", identifier: "myapp"},
		"space":        {name: "my app", identifier: "my_app"},
		"exe suffix":   {name: "app.exe", identifier: "app_exe"},
		"path":         {name: "./app", identifier: "__app"},
		"dash":         {name: "app-1", identifier: "app_1"},
		"semicolon":    {name: "app;payload", identifier: "app_payload"},
		"double quote": {name: `app"quote`, identifier: "app_quote"},
		"unicode":      {name: "café", identifier: "caf_"},
	} {
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			script := nushell.Snippet(testCase.name, nil, "")

			assert.Contains(t, script, "let "+testCase.identifier+"_completer = {|spans|")
			assert.Contains(t, script, "$"+testCase.identifier+"_completer")
			assert.Contains(t, script, "if ($spans | length) > 0 {")
			assert.Contains(t, script, "try {")
			assert.Contains(t, script, "^$spans.0 __complete nushell ...$words | from json")
			assert.Contains(t, script, "} catch { }")

			if testCase.identifier != testCase.name {
				for line := range strings.SplitSeq(script, "\n") {
					assert.False(t, strings.Contains(line, testCase.name),
						"raw name must never appear in the script: %q", line)
				}
			}
		})
	}
}
