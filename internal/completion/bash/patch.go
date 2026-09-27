// Package bash renders completion results in bash's line-oriented framing and
// generates the bash registration snippet for the wire protocol
// (docs/completion-wire-protocol.md). The Patch hook re-lexes
// the bash-native COMP_LINE environment through internal/completion/lex to
// reject redirect targets, isolate the current pipeline segment, and expose
// the COMP_WORDBREAKS prefix that the formatter strips from candidate values.
package bash

import (
	"fmt"
	"os"
	"strconv"

	"github.com/conga-sh/conga/internal/completion/lex"
	"github.com/conga-sh/conga/internal/sentinel"
)

// Bash-native completion environment variables.
const (
	envCompLine       = "COMP_LINE"
	envCompPoint      = "COMP_POINT"
	envCompType       = "COMP_TYPE"
	envCompWordbreaks = "COMP_WORDBREAKS"
)

// compTypeSuccessiveTabs is the readline COMP_TYPE of the successive-tabs
// listing mode, in which the wire carries display/value/description triplets.
const compTypeSuccessiveTabs = "63"

// defaultWordbreaks mirrors bash's documented COMP_WORDBREAKS default, used
// when the variable is unset.
const defaultWordbreaks = " \t\n\"'><=;|&(:"

// presentationState carries the formatting inputs the Patch hook derives from
// the completion environment to the Format hook. The completion pipeline runs
// synchronously on the main goroutine during process startup (AGENTS.md race
// policy), and the fork this driver replaces used the same documented
// single-goroutine channel; no mutex is required.
//
//nolint:gochecknoglobals // patch-to-format state channel within one synchronous completion invocation
var presentationState struct {
	// active reports whether Patch consumed a COMP_LINE this invocation;
	// inactive state makes Format derive its inputs from toComplete.
	active bool
	// compType is the raw COMP_TYPE value when known (empty otherwise).
	compType string
	// wordbreakPrefix is the raw prefix of the word being completed up to
	// and including its last COMP_WORDBREAKS rune (e.g. `--env="`).
	wordbreakPrefix string
}

// Patch rewrites the completion arguments for the engine: with COMP_LINE and
// COMP_POINT present it re-lexes the cursor-truncated line, rejects redirect
// positions, isolates the current pipeline segment, drops redirect operators
// with their targets, and preserves the trailing empty word. Without the
// bash-native environment (direct API invocations) the arguments pass through
// unchanged. The bash-native environment is always consumed — including on
// the redirect-rejection path — so a repeated Patch call in the same
// invocation never re-lexes a stale line.
func Patch(args []string) ([]string, error) {
	presentationState.active = false
	presentationState.compType = ""
	presentationState.wordbreakPrefix = ""

	line, ok := compLine()
	if !ok || line == "" {
		return args, nil
	}

	tokens := lex.Parse(line)

	// The environment is cleaned as soon as the line has been lexed so the
	// redirect rejection below cannot leave COMP_LINE behind: a stale line
	// would make a second Patch call re-lex the same redirect and fall back
	// to the raw arguments instead of honoring the sentinel.
	defer unsetCompEnv()

	if tokens.RedirectAtEnd() {
		return nil, fmt.Errorf("%w like `echo test >[TAB]`; no candidates are offered", sentinel.ErrRedirectTarget)
	}

	pipeline := tokens.CurrentPipeline().WithoutRedirects()

	presentationState.active = true
	presentationState.compType = compType()
	presentationState.wordbreakPrefix = pipeline.WordbreakPrefix(compWordbreaks())

	words := pipeline.Words()
	if len(words) > 0 {
		words = words[1:]
	}

	return words, nil
}

// compLine returns the cursor-truncated COMP_LINE: both COMP_LINE and a
// parseable COMP_POINT must be present, and the point is clamped to the line.
func compLine() (string, bool) {
	line, ok := os.LookupEnv(envCompLine)
	if !ok || line == "" {
		return "", false
	}

	rawPoint, ok := os.LookupEnv(envCompPoint)
	if !ok {
		return "", false
	}

	point, err := strconv.Atoi(rawPoint)
	if err != nil || point < 0 || point > len(line) {
		point = len(line)
	}

	return line[:point], true
}

// compType returns the raw COMP_TYPE value, or "" when absent or unparseable.
func compType() string {
	raw, ok := os.LookupEnv(envCompType)
	if !ok {
		return ""
	}

	if _, err := strconv.Atoi(raw); err != nil {
		return ""
	}

	return raw
}

// compWordbreaks returns the active COMP_WORDBREAKS, falling back to
// defaultWordbreaks when unset or empty.
func compWordbreaks() string {
	if value, ok := os.LookupEnv(envCompWordbreaks); ok && value != "" {
		return value
	}

	return defaultWordbreaks
}

// unsetCompEnv removes the bash-native completion variables so a second Patch
// call in the same invocation (completeForShell's direct-API path) passes
// through idempotently, and so a rejected redirect line is never re-lexed.
func unsetCompEnv() {
	for _, name := range []string{
		envCompLine,
		envCompPoint,
		envCompType,
		envCompWordbreaks,
		"COMP_KEY",
		"COMP_WORDS",
		"COMP_CWORD",
	} {
		_ = os.Unsetenv(name) //nolint:errcheck // Unsetting bash completion variables is best-effort.
	}
}
