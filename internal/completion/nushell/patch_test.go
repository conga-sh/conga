package nushell_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/nushell"
)

func TestPatch_DequotesRawSpans(t *testing.T) {
	t.Parallel()

	args := []string{`"double quoted"`, `'single quoted'`, "`backtick word`", "plain", "", `"partial"word`}

	assert.Equal(t,
		[]string{"double quoted", "single quoted", "backtick word", "plain", "", "partialword"},
		nushell.Patch(args))
}

func TestPatch_PreservesUnterminatedQuotes(t *testing.T) {
	t.Parallel()

	// A span without a closing quote is the word being completed and passes
	// through raw; concatenated quoted segments re-lex to one word.
	args := []string{`"unclosed`, `'unclosed2`, `"mid"close"quote`}

	assert.Equal(t, []string{`"unclosed`, `'unclosed2`, "midclosequote"}, nushell.Patch(args))
}

func TestPatch_MetacharacterQuotedSpans(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"a b", "x;y"}, nushell.Patch([]string{`"a b"`, `'x;y'`}))
}

func TestPatch_EmptyArgsPassThrough(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"", ""}, nushell.Patch([]string{"", ""}))
}

func TestPatch_DoesNotMutateCallerSlice(t *testing.T) {
	t.Parallel()

	original := []string{`"hello"`, "`world`"}
	copied := []string{`"hello"`, "`world`"}

	assert.Equal(t, []string{"hello", "world"}, nushell.Patch(original))
	assert.Equal(t, copied, original, "caller's slice must not be modified in-place")
}
