package lex_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/lex"
)

// defaultWordbreaks mirrors bash's documented COMP_WORDBREAKS default.
const defaultWordbreaks = " \t\n\"'><=;|&(:"

func TestParse_WordsSplitsOnWhitespace(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy staging")

	assert.Equal(t, []string{"cloudctl", "deploy", "staging"}, line.Words())
}

func TestParse_TrailingWhitespaceYieldsEmptyWord(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy --env ")

	assert.Equal(t, []string{"cloudctl", "deploy", "--env", ""}, line.Words())
}

func TestParse_EmptyLineYieldsSingleEmptyWord(t *testing.T) {
	t.Parallel()

	line := lex.Parse("")

	assert.Equal(t, []string{""}, line.Words())
}

func TestParse_NoTrailingWhitespaceOmitsEmptyWord(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy --env=")

	assert.Equal(t, []string{"cloudctl", "deploy", "--env="}, line.Words())
}

func TestParse_SingleQuotesResolveAndPreserveRaw(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy --env 'qa test'")

	assert.Equal(t, []string{"cloudctl", "deploy", "--env", "qa test"}, line.Words())
	assert.Equal(t, "'qa test'", line.Tokens[3].Raw)
}

func TestParse_DoubleQuotesResolveWithEscapes(t *testing.T) {
	t.Parallel()

	line := lex.Parse(`cloudctl "a\"b\$c"`)

	assert.Equal(t, []string{"cloudctl", `a"b$c`}, line.Words())
	assert.Equal(t, `"a\"b\$c"`, line.Tokens[1].Raw)
}

func TestParse_DoubleQuotesEscapedQuotePreservesRawAndValue(t *testing.T) {
	t.Parallel()

	line := lex.Parse(`cloudctl "a\"b"`)

	assert.Equal(t, `a"b`, line.Tokens[1].Value)
	assert.Equal(t, `"a\"b"`, line.Tokens[1].Raw)
}

func TestParse_DoubleQuotesEscapedBackslashPreservesRawAndValue(t *testing.T) {
	t.Parallel()

	line := lex.Parse(`cloudctl "a\\b"`)

	assert.Equal(t, `a\b`, line.Tokens[1].Value)
	assert.Equal(t, `"a\\b"`, line.Tokens[1].Raw)
}

func TestParse_DoubleQuotesBackslashNewlineContinuation(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl \"a\\\nb\"")

	assert.Equal(t, "ab", line.Tokens[1].Value)
	assert.Equal(t, "\"a\\\nb\"", line.Tokens[1].Raw)
}

func TestParse_UnquotedBackslashEscapesAnyCharacter(t *testing.T) {
	t.Parallel()

	line := lex.Parse(`cloudctl a\ b\"c`)

	assert.Equal(t, []string{"cloudctl", `a b"c`}, line.Words())
	assert.Equal(t, `a\ b\"c`, line.Tokens[1].Raw)
}

func TestParse_UnquotedBackslashSpacePreservesRawAndValue(t *testing.T) {
	t.Parallel()

	line := lex.Parse(`cloudctl a\ b`)

	assert.Equal(t, "a b", line.Tokens[1].Value)
	assert.Equal(t, `a\ b`, line.Tokens[1].Raw)
}

func TestParse_UnquotedBackslashNewlineContinuation(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl a\\\nb")

	assert.Equal(t, "ab", line.Tokens[1].Value)
	assert.Equal(t, "a\\\nb", line.Tokens[1].Raw)
}

func TestParse_UnclosedDoubleQuoteToleratesToEnd(t *testing.T) {
	t.Parallel()

	line := lex.Parse(`cloudctl deploy --env="pro`)

	assert.Equal(t, []string{"cloudctl", "deploy", "--env=pro"}, line.Words())
	assert.Equal(t, `--env="pro`, line.Tokens[2].Raw)
}

func TestParse_UnclosedSingleQuoteToleratesToEnd(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy --env='pro")

	assert.Equal(t, []string{"cloudctl", "deploy", "--env=pro"}, line.Words())
	assert.Equal(t, "--env='pro", line.Tokens[2].Raw)
}

func TestParse_MidWordQuoteOpensQuotedRegion(t *testing.T) {
	t.Parallel()

	line := lex.Parse(`cloudctl deploy --env="pro"duction`)

	assert.Equal(t, []string{"cloudctl", "deploy", `--env=production`}, line.Words())
	assert.Equal(t, `--env="pro"duction`, line.Tokens[2].Raw)
}

func TestParse_RedirectAndTargetAreClassified(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy > /tmp/out.log")

	redirect := line.Tokens[2]
	assert.Equal(t, lex.Redirect, redirect.Kind)
	assert.Equal(t, ">", redirect.Value)
	assert.Equal(t, []string{"cloudctl", "deploy", "/tmp/out.log"}, line.Words())
}

func TestParse_RedirectFormsLongestMatch(t *testing.T) {
	t.Parallel()

	for command, expected := range map[string]string{
		"cloudctl a > /tmp/f":   ">",
		"cloudctl a >> /tmp/f":  ">>",
		"cloudctl a <> /tmp/f":  "<>",
		"cloudctl a <<< w":      "<<<",
		"cloudctl a <& 2":       "<&",
		"cloudctl a &> /tmp/f":  "&>",
		"cloudctl a >& /tmp/f":  ">&",
		"cloudctl a &>> /tmp/f": "&>>",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			parsed := lex.Parse(command)

			redirects := []lex.Token{}

			for _, token := range parsed.Tokens {
				if token.Kind == lex.Redirect {
					redirects = append(redirects, token)
				}
			}

			assert.Equal(t, 1, len(redirects), "expected exactly one redirect in %q", command)
			assert.Equal(t, expected, redirects[0].Value)
		})
	}
}

func TestParse_FdNumberFoldsIntoRedirect(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy 2> /tmp/err.log")

	redirect := line.Tokens[2]
	assert.Equal(t, lex.Redirect, redirect.Kind)
	assert.Equal(t, "2>", redirect.Raw)
	assert.Equal(t, []string{"cloudctl", "deploy", "/tmp/err.log"}, line.Words())
}

func TestParse_MidWordRedirectSplitsWord(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy>log")

	assert.Equal(t, []string{"cloudctl", "deploy", "log"}, line.Words())
	assert.Equal(t, lex.Redirect, line.Tokens[2].Kind)
}

func TestParse_NonDigitPrefixFlushesWordBeforeRedirect(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy>x")

	assert.Equal(t, lex.Redirect, line.Tokens[2].Kind)
	assert.Equal(t, []string{"cloudctl", "deploy", "x"}, line.Words())
}

func TestParse_PipesAndSeparatorsClassify(t *testing.T) {
	t.Parallel()

	line := lex.Parse("echo foo | cloudctl dep && echo done")

	assert.Equal(t, lex.Pipe, line.Tokens[2].Kind)
	assert.Equal(t, lex.Separator, line.Tokens[5].Kind)
	assert.Equal(t, "|&", lex.Parse("a |& b").Tokens[1].Value)
	assert.Equal(t, "&&", lex.Parse("a && b").Tokens[1].Value)
	assert.Equal(t, "||", lex.Parse("a || b").Tokens[1].Value)
}

func TestParse_CommentConsumesRestOfLine(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy # trailing comment > redirect")

	assert.Equal(t, []string{"cloudctl", "deploy"}, line.Words())
	assert.False(t, line.RedirectAtEnd())
}

func TestParse_HashMidWordIsLiteral(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy --tag a#b")

	assert.Equal(t, []string{"cloudctl", "deploy", "--tag", "a#b"}, line.Words())
}

func TestCurrentPipeline_IsolatesLastSegment(t *testing.T) {
	t.Parallel()

	line := lex.Parse("echo foo | cloudctl dep")

	pipeline := line.CurrentPipeline()

	assert.Equal(t, []string{"cloudctl", "dep"}, pipeline.Words())
}

func TestCurrentPipeline_SeparatorSegments(t *testing.T) {
	t.Parallel()

	line := lex.Parse("echo foo; cloudctl dep && cd /tmp")

	assert.Equal(t, []string{"cd", "/tmp"}, line.CurrentPipeline().Words())
}

func TestCurrentPipeline_WithoutDelimiterReturnsAll(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy --env production")

	assert.Equal(t, []string{"cloudctl", "deploy", "--env", "production"}, line.CurrentPipeline().Words())
}

func TestWithoutRedirects_DropsOperatorAndTarget(t *testing.T) {
	t.Parallel()

	line := lex.Parse("corpusctl deploy > /tmp/output.log --env ")

	filtered := line.WithoutRedirects()

	assert.Equal(t, []string{"corpusctl", "deploy", "--env", ""}, filtered.Words())
}

func TestWithoutRedirects_DropsAttachedTarget(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy >/tmp/out.log --env st")

	assert.Equal(t, []string{"cloudctl", "deploy", "--env", "st"}, filteredWords(line))
}

func TestWithoutRedirects_KeepsLaterRedirectTargetAfterEarlierRedirect(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl > /tmp/a.log deploy > /tmp/b.log --env ")

	assert.Equal(t, []string{"cloudctl", "deploy", "--env", ""}, filteredWords(line))
}

func TestRedirectAtEnd_TrueWhenCursorFollowsRedirect(t *testing.T) {
	t.Parallel()

	assert.True(t, lex.Parse("cloudctl deploy > ").RedirectAtEnd())
	assert.True(t, lex.Parse("cloudctl deploy >").RedirectAtEnd())
}

func TestRedirectAtEnd_FalseForWordCompletion(t *testing.T) {
	t.Parallel()

	assert.False(t, lex.Parse("cloudctl deploy --env ").RedirectAtEnd())
	assert.False(t, lex.Parse("cloudctl deploy > /tmp/out --env ").RedirectAtEnd())
	assert.False(t, lex.Parse("echo > /tmp/a | cloudctl dep").RedirectAtEnd())
}

func TestWordbreakPrefix_AttachedSeparator(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy --env=")

	assert.Equal(t, "--env=", line.WordbreakPrefix(defaultWordbreaks))
}

func TestWordbreakPrefix_UnclosedQuoteIncluded(t *testing.T) {
	t.Parallel()

	line := lex.Parse(`cloudctl deploy --env="pro`)

	assert.Equal(t, `--env="`, line.WordbreakPrefix(defaultWordbreaks))
}

func TestWordbreakPrefix_EscapedQuote(t *testing.T) {
	t.Parallel()

	line := lex.Parse(`cloudctl deploy --env=\"pro`)

	assert.Equal(t, `--env=\"pro`, line.Tokens[2].Raw)
	assert.Equal(t, `--env=\"`, line.WordbreakPrefix(defaultWordbreaks))
}

func TestWordbreakPrefix_MapKeySeparator(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl deploy --labels team=")

	assert.Equal(t, "team=", line.WordbreakPrefix(defaultWordbreaks))
}

func TestWordbreakPrefix_NoneWithoutWordbreakRune(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", lex.Parse("cloudctl deploy prod-").WordbreakPrefix(defaultWordbreaks))
	assert.Equal(t, "", lex.Parse("cloudctl ").WordbreakPrefix(defaultWordbreaks))
}

func TestWordbreakPrefix_EmptyAfterOperator(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", lex.Parse("cloudctl deploy >").WordbreakPrefix(defaultWordbreaks))
}

func TestWords_PreservesEngineOrderForAliasAndFlagRuns(t *testing.T) {
	t.Parallel()

	line := lex.Parse("cloudctl d --env production --cluster prod-")

	assert.Equal(t, []string{"cloudctl", "d", "--env", "production", "--cluster", "prod-"}, line.Words())
}

// filteredWords restates the patch pipeline: isolate, filter, extract.
func filteredWords(line lex.Line) []string {
	return line.CurrentPipeline().WithoutRedirects().Words()
}
