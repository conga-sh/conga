package wire_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
)

func TestQuotePOSIX(t *testing.T) {
	t.Parallel()

	t.Run("plain word", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "'deploy'", wire.QuotePOSIX("deploy"))
	})

	t.Run("embedded single quote", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `'it'\''s'`, wire.QuotePOSIX("it's"))
	})

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "''", wire.QuotePOSIX(""))
	})
}

func TestQuoteFish(t *testing.T) {
	t.Parallel()

	t.Run("plain word", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "'deploy'", wire.QuoteFish("deploy"))
	})

	t.Run("embedded single quote", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `'it\'s'`, wire.QuoteFish("it's"))
	})

	t.Run("trailing backslash", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `'foo\\'`, wire.QuoteFish(`foo\`))
	})

	t.Run("embedded and lone backslashes", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `'\\'`, wire.QuoteFish(`\`))
		assert.Equal(t, `'a\\b\\'`, wire.QuoteFish(`a\b\`))
	})

	t.Run("backslash before single quote", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `'foo\\\'bar'`, wire.QuoteFish(`foo\'bar`))
	})
}

func TestQuoteElvish(t *testing.T) {
	t.Parallel()

	t.Run("plain word", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "'deploy'", wire.QuoteElvish("deploy"))
	})

	t.Run("embedded single quote doubled", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `'it''s'`, wire.QuoteElvish("it's"))
	})

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "''", wire.QuoteElvish(""))
	})
}

func TestQuotePowerShell(t *testing.T) {
	t.Parallel()

	t.Run("plain word", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "'deploy'", wire.QuotePowerShell("deploy"))
	})

	t.Run("embedded single quote doubled", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "'it''s'", wire.QuotePowerShell("it's"))
	})
}

func TestExecTarget(t *testing.T) {
	t.Parallel()

	t.Run("explicit name passthrough", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "cloudctl", wire.ExecTarget("cloudctl"))
	})

	t.Run("empty name resolves executable", func(t *testing.T) {
		t.Parallel()

		target := wire.ExecTarget("")

		assert.True(t, target != "", "empty name must resolve to the running executable")
	})
}

func TestSanitizeIdentifier(t *testing.T) {
	t.Parallel()

	for label, testCase := range map[string]struct {
		input    string
		expected string
	}{
		"plain word":   {input: "myapp", expected: "myapp"},
		"dash":         {input: "my-app", expected: "my_app"},
		"exe suffix":   {input: "app.exe", expected: "app_exe"},
		"space":        {input: "my app", expected: "my_app"},
		"path":         {input: "./app", expected: "__app"},
		"numbered":     {input: "app-1", expected: "app_1"},
		"semicolon":    {input: "app;payload", expected: "app_payload"},
		"double quote": {input: `app"quote`, expected: "app_quote"},
		"unicode":      {input: "café", expected: "caf_"},
		"underscore":   {input: "my_app", expected: "my_app"},
		"only dashes":  {input: "---", expected: "___"},
		"empty string": {input: "", expected: "_"},
		"only unicode": {input: "éé", expected: "__"},
		"mixed alnum":  {input: "app2go", expected: "app2go"},
	} {
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, wire.SanitizeIdentifier(testCase.input))
		})
	}
}

func TestQuoteTargetPOSIX(t *testing.T) {
	t.Parallel()

	t.Run("safe name unquoted", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "cloudctl", wire.QuoteTargetPOSIX("cloudctl"))
	})

	t.Run("path with safe punctuation unquoted", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "/usr/local/bin/cloudctl", wire.QuoteTargetPOSIX("/usr/local/bin/cloudctl"))
	})

	t.Run("space quoted", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `'my app'`, wire.QuoteTargetPOSIX("my app"))
	})

	t.Run("metacharacter quoted", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `'app$'`, wire.QuoteTargetPOSIX("app$"))
	})

	t.Run("empty unquoted", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "", wire.QuoteTargetPOSIX(""))
	})
}

func TestQuoteIfUnsafe(t *testing.T) {
	t.Parallel()

	t.Run("safe word returned unchanged without invoking quote", func(t *testing.T) {
		t.Parallel()

		result := wire.QuoteIfUnsafe("deploy", " '$\"\\", func(string) string {
			return "SHOULD_NOT_BE_CALLED"
		})

		assert.Equal(t, "deploy", result)
	})

	t.Run("unsafe word quoted through callback", func(t *testing.T) {
		t.Parallel()

		result := wire.QuoteIfUnsafe("my app", " '$\"\\", func(s string) string {
			return "'" + s + "'"
		})

		assert.Equal(t, "'my app'", result)
	})

	t.Run("empty string is safe", func(t *testing.T) {
		t.Parallel()

		result := wire.QuoteIfUnsafe("", " '$\"\\", func(string) string {
			return "SHOULD_NOT_BE_CALLED"
		})

		assert.Equal(t, "", result)
	})
}
