package zsh_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/zsh"
)

func TestSnippet_MultiForm(t *testing.T) {
	t.Parallel()

	snippet := zsh.Snippet("myapp", []string{"ma", "app"})

	assert.Contains(t, snippet, "#compdef myapp ma app\n")
	assert.Contains(t, snippet, "function _myapp_completer {")
	assert.Contains(t, snippet, `local target="${words[1]/#\~/$HOME}"`)
	assert.Contains(t, snippet, `target="${aliases[$target]:-$target}"`)
	assert.Contains(t, snippet, `lines="$(env "${conga_env[@]}" "$target" __complete zsh "${args[@]}" 2>/dev/null)" || lines=""`)
	assert.Contains(t, snippet, "compdef _myapp_completer myapp ma app\n")
}

func TestSnippet_SingleForm(t *testing.T) {
	t.Parallel()

	snippet := zsh.Snippet("foobar", nil)

	assert.Contains(t, snippet, "#compdef foobar\n")
	assert.Contains(t, snippet, "function _foobar_completion {")
	assert.Contains(t, snippet, `local target="${words[1]/#\~/$HOME}"`)
	assert.Contains(t, snippet, `target="${aliases[$target]:-$target}"`)
	assert.Contains(t, snippet, `lines="$(env "${conga_env[@]}" "$target" __complete zsh "${args[@]}" 2>/dev/null)" || lines=""`)
	assert.Contains(t, snippet, "compdef _foobar_completion foobar\n")
}

func TestSnippet_WordCollection(t *testing.T) {
	t.Parallel()

	snippet := zsh.Snippet("myapp", nil)

	// Words before the one being completed are ${(Q)}-unquoted; the last
	// word is passed raw.
	assert.Contains(t, snippet, `args=("${(@Q)words[2,CURRENT-1]}" "${words[CURRENT]}")`)
	assert.Contains(t, snippet, `args=("")`)
}

func TestSnippet_RecordParsing(t *testing.T) {
	t.Parallel()

	snippet := zsh.Snippet("myapp", nil)

	// Records are split on the unit separator field delimiter.
	assert.Contains(t, snippet, `us=$'\x1f'`)
	assert.Contains(t, snippet, `"${(@f)lines}"`)

	// noprefix drives compstate[insert]='' (no auto-insertion).
	assert.Contains(t, snippet, "compstate[insert]=''")

	// Probe-before-set: only the completer's own definitions are deleted, and
	// the engine defaults (group-name "" and the L-record list-colors) are
	// installed only when the user has not configured the style.
	assert.Contains(t, snippet, `zstyle -d "$style_pattern" group-name list-colors`)
	assert.Contains(t, snippet, `zstyle -a "$probe_context" group-name user_group_name`)
	assert.Contains(t, snippet, `zstyle "$style_pattern" group-name ''`)
	assert.Contains(t, snippet, `zstyle -a "$probe_context" list-colors user_list_colors`)
	assert.Contains(t, snippet, `(( conga_list_colors )) && zstyle "$style_pattern" list-colors "${line#*$us}"`)

	// zstyle-driven suppression sidebands: verbose hides descriptions,
	// show-aliases hides alias candidates.
	assert.Contains(t, snippet, `zstyle -T ":completion:${curcontext}:" verbose || conga_env+=(CONGA_COMPLETION_DESCRIPTIONS=0)`)
	assert.Contains(t, snippet, `zstyle -T ":completion:${curcontext}:" show-aliases || conga_env+=(CONGA_COMPLETION_ALIASES=0)`)

	// Messages forward to _message -r.
	assert.Contains(t, snippet, `_message -r "${line#*"$us"}"`)

	// Candidates compose display:description with escaped colons and group
	// into one _describe -t call per tag; untagged candidates use compadd.
	assert.Contains(t, snippet, `composed="${display//:/\\:}"`)
	assert.Contains(t, snippet, `_describe -t "$tag" "$tag" displays values -Q -S ''`)
	assert.Contains(t, snippet, `compadd -Q -S '' -a values`)
}

func TestSnippet_EmptyAliasListUsesSingleForm(t *testing.T) {
	t.Parallel()

	empty := zsh.Snippet("foobar", []string{})

	assert.Contains(t, empty, "function _foobar_completion {")
	assert.NotContains(t, empty, "_foobar_completer")
}

// TestSnippet_IdentifierHygiene pins the quoting contract for hostile command
// names: the completer function name is always a sanitized identifier, the
// completer invokes whatever command word the user typed via ${words[1]}
// (never the raw generation-time name), and registration lines keep the raw
// name quoted via wire.QuoteTargetPOSIX. The #compdef header is a compinit
// comment tag, not executable text, so its raw-name occurrences are excluded
// from the injection scan.
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

			snippet := zsh.Snippet(testCase.name, nil)

			assert.Contains(t, snippet, "function _"+testCase.identifier+"_completion {")
			assert.Contains(t, snippet, "compdef _"+testCase.identifier+"_completion "+testCase.target+"\n")
			assert.Contains(t, snippet, `local target="${words[1]/#\~/$HOME}"`)
			assert.Contains(t, snippet, `lines="$(env "${conga_env[@]}" "$target" __complete zsh "${args[@]}" 2>/dev/null)" || lines=""`)

			// Injection invariant: when sanitization rewrote the identifier,
			// no executable line carries the raw name outside the compdef
			// registration (the #compdef header is a compinit comment tag, not
			// executable text; the body invokes ${words[1]}, never the raw
			// name).
			if testCase.target != testCase.name {
				for line := range strings.SplitSeq(snippet, "\n") {
					if strings.HasPrefix(line, "#compdef ") || strings.HasPrefix(line, "  lines=") {
						continue
					}

					withoutTarget := strings.ReplaceAll(line, testCase.target, "")

					assert.False(t, strings.Contains(withoutTarget, testCase.name),
						"raw name must only appear inside the quoted target: %q", line)
				}
			}

			assert.NotContains(t, snippet, testCase.name+" __complete zsh",
				"body must never invoke the generation-time name")
		})
	}
}

func TestSnippet_MultiFormUnsafeNameSanitizesFunction(t *testing.T) {
	t.Parallel()

	snippet := zsh.Snippet("my app", []string{"ma"})

	assert.Contains(t, snippet, "function _my_app_completer {")
	assert.Contains(t, snippet, "compdef _my_app_completer 'my app' ma\n")
	assert.NotContains(t, snippet, "function _my app")
}
