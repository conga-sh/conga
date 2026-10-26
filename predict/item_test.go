package predict_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/predict"
)

func TestItem(t *testing.T) {
	t.Parallel()

	t.Run("construct with value and description", func(t *testing.T) {
		t.Parallel()

		item := predict.NewItem("json", "JavaScript Object Notation")

		assert.Equal(t, "json", item.Value)
		assert.Equal(t, "JavaScript Object Notation", item.Description)
		assert.Equal(t, "", item.Style)
	})

	t.Run("with description modifier", func(t *testing.T) {
		t.Parallel()

		item := predict.NewItem("yaml", "")

		modified := item.WithDescription("YAML Ain't Markup Language")

		assert.Equal(t, "YAML Ain't Markup Language", modified.Description)
		assert.Equal(t, "", item.Description)
	})

	t.Run("with style modifier", func(t *testing.T) {
		t.Parallel()

		item := predict.NewItem("toml", "TOML configuration")

		modified := item.WithStyle("blue")

		assert.Equal(t, "blue", modified.Style)
		assert.Equal(t, "", item.Style)
	})

	t.Run("with tag modifier", func(t *testing.T) {
		t.Parallel()

		item := predict.NewItem("xml", "XML format")

		modified := item.WithTag("data-formats")

		assert.Equal(t, "data-formats", modified.Tag)
		assert.Equal(t, "", item.Tag)
	})

	t.Run("with alias modifier", func(t *testing.T) {
		t.Parallel()

		item := predict.NewItem("build", "Build the project")

		modified := item.WithAlias()

		assert.True(t, modified.Alias)
		assert.False(t, item.Alias)
	})
}
