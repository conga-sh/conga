package elvish_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/elvish"
)

func TestSnippet_RegistersCompleterPerNameAndAlias(t *testing.T) {
	t.Parallel()

	script := elvish.Snippet("myapp", []string{"ma", "app"}, "")

	assert.Equal(t, 3, strings.Count(script, "set edit:completion:arg-completer["), "one registration per name and alias")
	assert.Contains(t, script, "set edit:completion:arg-completer[myapp] = $completer~")
	assert.Contains(t, script, "set edit:completion:arg-completer[ma] = $completer~")
	assert.Contains(t, script, "set edit:completion:arg-completer[app] = $completer~")
}

func TestSnippet_InvokesCompleteEntryPointWithSlicedArgs(t *testing.T) {
	t.Parallel()

	script := elvish.Snippet("myapp", nil, "")

	assert.Contains(t, script, "set cmd = (search-external $args[0])")
	assert.Contains(t, script, "var completion = ($cmd __complete elvish (all $args[1..]) 2>/dev/null | from-json)")
	assert.NotContains(t, script, "to-lines", "argv-only invocation: no stdin piping")
}

func TestSnippet_SetsSidebandShellVariable(t *testing.T) {
	t.Parallel()

	assert.Contains(t, elvish.Snippet("myapp", nil, ""), "set-env CONGA_SHELL elvish")
}

func TestSnippet_RendersCandidates(t *testing.T) {
	t.Parallel()

	script := elvish.Snippet("myapp", nil, "")

	assert.Contains(t, script, "edit:complex-candidate $c[value] &display=(styled $c[display] $styling)$description &code-suffix=$c[codeSuffix]")
	assert.NotContains(t, script, "echo", "must not echo output during completion")
	assert.NotContains(t, script, "notify", "must not notify unmanaged messages during completion")
	assert.Contains(t, script, "from-json")
}

func TestSnippet_InvokesTypedCommandName(t *testing.T) {
	t.Parallel()

	script := elvish.Snippet("my app", nil, "")

	assert.Contains(t, script, "set cmd = (search-external $args[0])")
	assert.Contains(t, script, "var completion = ($cmd __complete elvish (all $args[1..]) 2>/dev/null | from-json)")
}

func TestSnippet_QuotesNameWithMetacharacters(t *testing.T) {
	t.Parallel()

	script := elvish.Snippet("my app", []string{"app[x]"}, "")

	assert.Contains(t, script, `set edit:completion:arg-completer['my app'] = $completer~`)
	assert.Contains(t, script, `set edit:completion:arg-completer['app[x]'] = $completer~`)
}

// TestSnippet_IdentifierHygiene pins the quoting contract for hostile command
// names: elvish registration keys must stay the literal command name (elvish
// dispatches arg-completers by key), quoted with wire.QuoteElvish whenever
// the name carries any elvish metacharacter — including the separators,
// redirections, globbing, and comment markers `; | & < > * ? #` — and the
// raw name never appears as executable text outside quotes.
func TestSnippet_IdentifierHygiene(t *testing.T) {
	t.Parallel()

	for label, testCase := range map[string]struct {
		name   string
		target string
	}{
		"plain word":   {name: "myapp", target: "myapp"},
		"space":        {name: "my app", target: "'my app'"},
		"exe suffix":   {name: "app.exe", target: "app.exe"},
		"path":         {name: "./app", target: "./app"},
		"dash":         {name: "app-1", target: "app-1"},
		"semicolon":    {name: "app;payload", target: "'app;payload'"},
		"pipe":         {name: "app|payload", target: "'app|payload'"},
		"ampersand":    {name: "app&payload", target: "'app&payload'"},
		"less than":    {name: "app<payload", target: "'app<payload'"},
		"greater than": {name: "app>payload", target: "'app>payload'"},
		"glob star":    {name: "app*payload", target: "'app*payload'"},
		"question":     {name: "app?payload", target: "'app?payload'"},
		"hash":         {name: "app#payload", target: "'app#payload'"},
		"double quote": {name: `app"quote`, target: `'app"quote'`},
		"unicode":      {name: "café", target: "café"},
	} {
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			script := elvish.Snippet(testCase.name, nil, "")

			assert.Contains(t, script, "set cmd = (search-external $args[0])")
			assert.Contains(t, script, "var completion = ($cmd __complete elvish (all $args[1..]) 2>/dev/null | from-json)")
			assert.Contains(t, script, "set edit:completion:arg-completer["+testCase.target+"] = $completer~")

			// Injection invariant: no line carries the raw name outside the
			// quoted registration key / invocation target.
			if testCase.target != testCase.name {
				for line := range strings.SplitSeq(script, "\n") {
					withoutQuoted := strings.ReplaceAll(line, testCase.target, "")

					assert.False(t, strings.Contains(withoutQuoted, testCase.name),
						"raw name must only appear inside quoted words: %q", line)
				}
			}
		})
	}
}
