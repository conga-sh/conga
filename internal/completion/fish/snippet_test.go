package fish_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/fish"
)

func TestSnippet_SingleName(t *testing.T) {
	t.Parallel()

	output := fish.Snippet("myapp", nil)

	assert.Equal(t,
		`function _myapp_completion`+"\n"+
			`  set --local words (commandline -opc)`+"\n"+
			`  if test (count $words) -gt 0`+"\n"+
			`    set --local current (commandline -ct)`+"\n"+
			`    $words[1] __complete fish $words[2..-1] $current 2>/dev/null`+"\n"+
			`  end`+"\n"+
			`end`+"\n"+
			"\n"+
			`complete -e 'myapp'`+"\n"+
			`complete -c 'myapp' -f -a '(_myapp_completion)' -r`+"\n",
		output)
}

func TestSnippet_AliasesShareFunction(t *testing.T) {
	t.Parallel()

	output := fish.Snippet("myapp", []string{"ma", "app"})

	assert.Equal(t,
		`function _myapp_completion`+"\n"+
			`  set --local words (commandline -opc)`+"\n"+
			`  if test (count $words) -gt 0`+"\n"+
			`    set --local current (commandline -ct)`+"\n"+
			`    $words[1] __complete fish $words[2..-1] $current 2>/dev/null`+"\n"+
			`  end`+"\n"+
			`end`+"\n"+
			"\n"+
			`complete -e 'myapp'`+"\n"+
			`complete -c 'myapp' -f -a '(_myapp_completion)' -r`+"\n"+
			"\n"+
			`complete -e 'ma'`+"\n"+
			`complete -c 'ma' -f -a '(_myapp_completion)' -r`+"\n"+
			"\n"+
			`complete -e 'app'`+"\n"+
			`complete -c 'app' -f -a '(_myapp_completion)' -r`+"\n",
		output)
}

func TestSnippet_NoLegacyProtocolRemnants(t *testing.T) {
	t.Parallel()

	output := fish.Snippet("myapp", []string{"ma"})

	for _, remnant := range []string{"sed ", "xargs", "commandline -cp"} {
		assert.False(t, strings.Contains(output, remnant), "snippet must not contain %q", remnant)
	}
}

func TestSnippet_QuotesNonIdentifierNames(t *testing.T) {
	t.Parallel()

	output := fish.Snippet("my-app", nil)

	assert.True(t, strings.Contains(output, "complete -c 'my-app' -f -a '(_my_app_completion)' -r"))
}

// TestSnippet_IdentifierHygiene pins the quoting contract for hostile command
// names: the completer function name is always a sanitized identifier, the
// function body invokes whatever command name fish reports in $words[1]
// (never the raw generation-time name), registration lines keep the raw name
// quoted, and the raw name never appears as executable text outside quotes.
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

			output := fish.Snippet(testCase.name, nil)

			assert.Contains(t, output, "function _"+testCase.identifier+"_completion\n")
			assert.Contains(t, output, `$words[1] __complete fish $words[2..-1] $current 2>/dev/null`)
			assert.Contains(t, output, "complete -c '"+testCase.name+"' -f -a '(_"+testCase.identifier+"_completion)' -r")

			// Injection invariant: when sanitization rewrote the identifier,
			// no line carries the raw name outside the single-quoted
			// target/registration words (identifier positions hold the
			// sanitized form, which no longer contains the raw substring).
			if testCase.identifier != testCase.name {
				for line := range strings.SplitSeq(output, "\n") {
					withoutQuoted := strings.ReplaceAll(line, "'"+testCase.name+"'", "")

					assert.False(t, strings.Contains(withoutQuoted, testCase.name),
						"raw name must only appear inside quoted words: %q", line)
				}
			}
		})
	}
}
