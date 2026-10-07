// Package bash renders completion results in bash's line-oriented framing and
// generates the bash registration snippet for the wire protocol
// (docs/completion-wire-protocol.md). The Session Patch pass re-lexes
// the bash-native COMP_LINE environment through internal/completion/lex to
// reject redirect targets, isolate the current pipeline segment, and expose
// the COMP_WORDBREAKS prefix that the session's Format pass strips from
// candidate values.
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

// EnvLookup returns the value of an environment variable and whether it was set.
type EnvLookup func(key string) (string, bool)

// Session carries one bash completion invocation's presentation state: the
// inputs Patch derives from the bash-native environment and Format consumes
// when framing candidates. A fresh Session per invocation keeps concurrent
// completions independent — this package holds no per-invocation mutable state.
type Session struct {
	environmentLookup EnvLookup
	// active reports whether Patch consumed a COMP_LINE in this session;
	// inactive state makes Format derive its inputs from toComplete and the
	// environment (direct formatter invocations).
	active bool
	// compType is the raw COMP_TYPE value when known (empty otherwise).
	compType string
	// wordbreakPrefix is the raw prefix of the word being completed up to
	// and including its last COMP_WORDBREAKS rune (e.g. `--env="`).
	wordbreakPrefix string
}

// NewSession returns an inactive session for one completion invocation: Format
// derives its inputs from toComplete and the environment until Patch runs.
func NewSession() *Session {
	return NewSessionWithEnv(nil)
}

// NewSessionWithEnv returns an inactive session using lookup to resolve
// environment variables (falling back to os.LookupEnv when lookup is nil).
func NewSessionWithEnv(lookup EnvLookup) *Session {
	return &Session{environmentLookup: lookup}
}

// Patch rewrites the completion arguments for the engine: with COMP_LINE and
// COMP_POINT present it re-lexes the cursor-truncated line, rejects redirect
// positions, isolates the current pipeline segment, drops redirect operators
// with their targets, and preserves the trailing empty word. Without the
// bash-native environment (direct API invocations) the arguments pass through
// unchanged. Patch only reads the environment; it never mutates process state.
func (s *Session) Patch(args []string) ([]string, error) {
	s.active = false
	s.compType = ""
	s.wordbreakPrefix = ""

	line, ok := s.resolveCompLine()
	if !ok || line == "" {
		return args, nil
	}

	tokens := lex.Parse(line)

	if tokens.RedirectAtEnd() {
		return nil, fmt.Errorf("%w like `echo test >[TAB]`; no candidates are offered", sentinel.ErrRedirectTarget)
	}

	pipeline := tokens.CurrentPipeline().WithoutRedirects()

	s.active = true
	s.compType = s.resolveCompType()
	s.wordbreakPrefix = pipeline.WordbreakPrefix(s.resolveCompWordbreaks())

	words := pipeline.Words()
	if len(words) > 0 {
		words = words[1:]
	}

	return words, nil
}

func (s *Session) resolveCompLine() (string, bool) {
	line, ok := s.lookupEnvironment(envCompLine)
	if !ok || line == "" {
		return "", false
	}

	rawPoint, ok := s.lookupEnvironment(envCompPoint)
	if !ok {
		return "", false
	}

	point, err := strconv.Atoi(rawPoint)
	if err != nil || point < 0 || point > len(line) {
		point = len(line)
	}

	return line[:point], true
}

func (s *Session) resolveCompType() string {
	raw, ok := s.lookupEnvironment(envCompType)
	if !ok {
		return ""
	}

	if _, err := strconv.Atoi(raw); err != nil {
		return ""
	}

	return raw
}

func (s *Session) resolveCompWordbreaks() string {
	if value, ok := s.lookupEnvironment(envCompWordbreaks); ok && value != "" {
		return value
	}

	return defaultWordbreaks
}

func (s *Session) lookupEnvironment(key string) (string, bool) {
	if s.environmentLookup != nil {
		return s.environmentLookup(key)
	}

	return os.LookupEnv(key)
}

func (s *Session) getEnvironment(key string) string {
	value, _ := s.lookupEnvironment(key)

	return value
}
