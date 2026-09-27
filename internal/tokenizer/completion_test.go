package tokenizer_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/tokenizer"
)

func TestParseCompLine(t *testing.T) {
	t.Parallel()

	t.Run("empty line", func(t *testing.T) {
		t.Parallel()

		line := ""
		point := 0

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{""}, words)
	})

	t.Run("cursor at end of single word", func(t *testing.T) {
		t.Parallel()

		line := "myapp"
		point := len(line)

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{"myapp"}, words)
	})

	t.Run("cursor after space following word", func(t *testing.T) {
		t.Parallel()

		line := "myapp "
		point := len(line)

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{"myapp", ""}, words)
	})

	t.Run("multiple words with cursor in middle of last word", func(t *testing.T) {
		t.Parallel()

		line := "myapp deploy --env=prod"
		point := len("myapp deploy --en")

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{"myapp", "deploy", "--en"}, words)
	})

	t.Run("multiple consecutive spaces", func(t *testing.T) {
		t.Parallel()

		line := "myapp   deploy   "
		point := len(line)

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{"myapp", "deploy", ""}, words)
	})

	t.Run("single quoted string with spaces", func(t *testing.T) {
		t.Parallel()

		line := "myapp 'hello world' "
		point := len(line)

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{"myapp", "hello world", ""}, words)
	})

	t.Run("double quoted string with escaped quotes", func(t *testing.T) {
		t.Parallel()

		line := `myapp "say \"hi\"" `
		point := len(line)

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{"myapp", `say "hi"`, ""}, words)
	})

	t.Run("backslash escaped space outside quotes", func(t *testing.T) {
		t.Parallel()

		line := `myapp hello\ world `
		point := len(line)

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{"myapp", "hello world", ""}, words)
	})

	t.Run("cursor clamped when point is negative", func(t *testing.T) {
		t.Parallel()

		line := "myapp deploy"
		point := -5

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{""}, words)
	})

	t.Run("cursor clamped when point exceeds length", func(t *testing.T) {
		t.Parallel()

		line := "myapp deploy"
		point := 100

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{"myapp", "deploy"}, words)
	})

	t.Run("attached flag with equal sign", func(t *testing.T) {
		t.Parallel()

		line := "myapp --format="
		point := len(line)

		words := tokenizer.ParseCompLine(line, point)

		assert.Equal(t, []string{"myapp", "--format="}, words)
	})
}
