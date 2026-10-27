package bash_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/bash"
)

func TestSnippet_SingleCommand(t *testing.T) {
	t.Parallel()

	snippet := bash.Snippet("myapp", nil, "")

	assert.Equal(t,
		`#!/bin/bash`+"\n"+
			`_myapp_completion() {`+"\n"+
			`  export COMP_LINE`+"\n"+
			`  export COMP_POINT`+"\n"+
			`  export COMP_TYPE`+"\n"+
			`  export COMP_WORDBREAKS`+"\n"+
			"\n"+
			`  local compline="${COMP_LINE:0:${COMP_POINT}}"`+"\n"+
			`  local words="${compline#"${compline%%[! ]*}"}"`+"\n"+
			`  local cmd="${COMP_WORDS[0]:-${words%% *}}"`+"\n"+
			`  cmd="${cmd/#\~/$HOME}"`+"\n"+
			`  if [[ "$words" == *" "* ]]; then`+"\n"+
			`    words="${words#* }"`+"\n"+
			`  else`+"\n"+
			`    words=""`+"\n"+
			`  fi`+"\n"+
			`  local data=""`+"\n"+
			`  if [[ -n "$cmd" ]]; then`+"\n"+
			`    data=$(printf '%s' "${words}''" | xargs -- "$cmd" __complete bash 2>/dev/null) ||`+"\n"+
			`    data=$(printf '%s' "${words}'" | xargs -- "$cmd" __complete bash 2>/dev/null) ||`+"\n"+
			`    data=$(printf '%s' "${words}\"" | xargs -- "$cmd" __complete bash 2>/dev/null) || true`+"\n"+
			`  fi`+"\n"+
			"\n"+
			`  local line payload display description`+"\n"+
			`  COMPREPLY=()`+"\n"+
			`  while IFS= read -r line; do`+"\n"+
			`    case "$line" in`+"\n"+
			`      'D'$'\t'*)`+"\n"+
			`        [[ "${line#D?}" == *nospace* ]] && compopt -o nospace`+"\n"+
			`        ;;`+"\n"+
			`      'M'$'\t'*)`+"\n"+
			`        ;;`+"\n"+
			`      'C'$'\t'*)`+"\n"+
			`        payload="${line#C?}"`+"\n"+
			`        if [[ "$COMP_TYPE" == "63" ]]; then`+"\n"+
			`          IFS=$'\t' read -r display _ description <<< "$payload"`+"\n"+
			`          COMPREPLY+=("${display}${description:+ (${description})}")`+"\n"+
			`        else`+"\n"+
			`          COMPREPLY+=("$payload")`+"\n"+
			`        fi`+"\n"+
			`        ;;`+"\n"+
			`    esac`+"\n"+
			`  done <<< "$data"`+"\n"+
			`}`+"\n"+
			`complete -o noquote -F _myapp_completion myapp`+"\n",
		snippet)
}

func TestSnippet_RegistersAliases(t *testing.T) {
	t.Parallel()

	snippet := bash.Snippet("myapp", []string{"ma", "app"}, "")

	assert.Contains(t, snippet, "complete -o noquote -F _myapp_completion myapp ma app")
}

// TestSnippet_IdentifierHygiene pins the quoting contract for hostile command
// names: the completer function name is always a sanitized identifier (a raw
// name with spaces or metacharacters produces unparseable bash), the xargs
// invocation target is "$cmd" — the command word the user typed, never the
// raw generation-time name — registration lines keep the raw name quoted, and
// the raw name never appears as executable text outside quotes.
func TestSnippet_IdentifierHygiene(t *testing.T) {
	t.Parallel()

	for label, testCase := range map[string]struct {
		name       string
		identifier string
		target     string
	}{
		"plain word":   {name: "myapp", identifier: "myapp", target: "myapp"},
		"space":        {name: "my app", identifier: "my_app", target: "'my app'"},
		"exe suffix":   {name: "app.exe", identifier: "app_exe", target: "app.exe"},
		"path":         {name: "./app", identifier: "__app", target: "./app"},
		"dash":         {name: "app-1", identifier: "app_1", target: "app-1"},
		"semicolon":    {name: "app;payload", identifier: "app_payload", target: "'app;payload'"},
		"double quote": {name: `app"quote`, identifier: "app_quote", target: `'app"quote'`},
		"unicode":      {name: "café", identifier: "caf_", target: "'café'"},
	} {
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			snippet := bash.Snippet(testCase.name, nil, "")

			assert.Contains(t, snippet, "_"+testCase.identifier+"_completion() {")
			assert.Contains(t, snippet, "complete -o noquote -F _"+testCase.identifier+"_completion "+testCase.target)
			assert.Contains(t, snippet, "| xargs -- \"$cmd\" __complete bash 2>/dev/null")
			assert.NotContains(t, snippet, "function _"+testCase.name)

			// Injection invariant: when sanitization rewrote the identifier,
			// no line carries the raw name outside the quoted target /
			// registration words.
			if testCase.target != testCase.name {
				for line := range strings.SplitSeq(snippet, "\n") {
					withoutQuoted := strings.ReplaceAll(line, testCase.target, "")

					assert.False(t, strings.Contains(withoutQuoted, testCase.name),
						"raw name must only appear inside quoted words: %q", line)
				}
			}
		})
	}
}
