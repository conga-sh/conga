package predict_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/predict"
)

func TestDirective(t *testing.T) {
	t.Parallel()

	t.Run("zero directive has no flags", func(t *testing.T) {
		t.Parallel()

		var zero predict.Directive

		assert.False(t, zero.Has(predict.DirectiveError))
		assert.False(t, zero.Has(predict.DirectiveNoSpace))
		assert.False(t, zero.Has(predict.DirectiveNoFileCompletion))
		assert.False(t, zero.Has(predict.DirectiveKeepOrder))
	})

	t.Run("bitmask combination and checks", func(t *testing.T) {
		t.Parallel()

		combined := predict.DirectiveNoFileCompletion | predict.DirectiveNoSpace | predict.DirectiveKeepOrder

		assert.True(t, combined.Has(predict.DirectiveNoFileCompletion))
		assert.True(t, combined.Has(predict.DirectiveNoSpace))
		assert.True(t, combined.Has(predict.DirectiveKeepOrder))
		assert.False(t, combined.Has(predict.DirectiveError))
	})
}
