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
		assert.False(t, zero.Has(predict.DirectiveNoFileComp))
		assert.False(t, zero.Has(predict.DirectiveKeepOrder))
	})

	t.Run("bitmask combination and checks", func(t *testing.T) {
		t.Parallel()

		combined := predict.DirectiveNoFileComp | predict.DirectiveNoSpace | predict.DirectiveKeepOrder

		assert.True(t, combined.Has(predict.DirectiveNoFileComp))
		assert.True(t, combined.Has(predict.DirectiveNoSpace))
		assert.True(t, combined.Has(predict.DirectiveKeepOrder))
		assert.False(t, combined.Has(predict.DirectiveError))
	})

	t.Run("individual directive flags", func(t *testing.T) {
		t.Parallel()

		assert.True(t, predict.DirectiveError.Has(predict.DirectiveError))
		assert.True(t, predict.DirectiveNoSpace.Has(predict.DirectiveNoSpace))
		assert.True(t, predict.DirectiveNoFileComp.Has(predict.DirectiveNoFileComp))
		assert.True(t, predict.DirectiveKeepOrder.Has(predict.DirectiveKeepOrder))
		assert.False(t, predict.DirectiveNoSpace.Has(predict.DirectiveNoFileComp))
	})
}
