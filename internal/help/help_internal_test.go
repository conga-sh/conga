package help

import (
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

func TestFormatRangeHint(t *testing.T) {
	t.Parallel()

	t.Run("nil range returns empty string", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "", formatRangeHint(nil, ""))
	})

	t.Run("exact range without prefix", func(t *testing.T) {
		t.Parallel()

		r := &spec.Range{Exact: true, MinStr: "5"}
		assert.Equal(t, "[5]", formatRangeHint(r, ""))
	})

	t.Run("exact range with prefix", func(t *testing.T) {
		t.Parallel()

		r := &spec.Range{Exact: true, MinStr: "5"}
		assert.Equal(t, "[len: 5]", formatRangeHint(r, "len: "))
	})

	t.Run("bounded range with min and max", func(t *testing.T) {
		t.Parallel()

		r := &spec.Range{HasMin: true, HasMax: true, MinStr: "1", MaxStr: "10"}
		assert.Equal(t, "[1..10]", formatRangeHint(r, ""))
	})

	t.Run("unbounded max with min only", func(t *testing.T) {
		t.Parallel()

		r := &spec.Range{HasMin: true, MinStr: "1"}
		assert.Equal(t, "[>= 1]", formatRangeHint(r, ""))
	})

	t.Run("unbounded min with max only", func(t *testing.T) {
		t.Parallel()

		r := &spec.Range{HasMax: true, MaxStr: "10"}
		assert.Equal(t, "[<= 10]", formatRangeHint(r, ""))
	})
}

func TestFormatConstraintHints(t *testing.T) {
	t.Parallel()

	t.Run("all ranges populated", func(t *testing.T) {
		t.Parallel()

		valueRange := &spec.Range{HasMin: true, HasMax: true, MinStr: "1", MaxStr: "10"}
		lengthRange := &spec.Range{Exact: true, MinStr: "5"}
		itemsRange := &spec.Range{HasMin: true, MinStr: "2"}

		hint := formatConstraintHints(valueRange, lengthRange, itemsRange, hintSuppression{})
		assert.Equal(t, "[1..10] [len: 5] [items: >= 2]", hint)
	})

	t.Run("nil ranges return empty", func(t *testing.T) {
		t.Parallel()

		hint := formatConstraintHints(nil, nil, nil, hintSuppression{})
		assert.Equal(t, "", hint)
	})

	t.Run("referenced ranges are suppressed individually", func(t *testing.T) {
		t.Parallel()

		valueRange := &spec.Range{HasMin: true, HasMax: true, MinStr: "1", MaxStr: "10"}
		lengthRange := &spec.Range{Exact: true, MinStr: "5"}

		hint := formatConstraintHints(valueRange, lengthRange, nil, hintSuppression{rangeRef: true})
		assert.Equal(t, "[len: 5]", hint)
	})
}

func TestKindMetavar(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		kind     reflect.Kind
		expected string
	}{
		{kind: reflect.String, expected: "STRING"},
		{kind: reflect.Bool, expected: "BOOL"},
		{kind: reflect.Int, expected: "INT"},
		{kind: reflect.Int8, expected: "INT"},
		{kind: reflect.Int16, expected: "INT"},
		{kind: reflect.Int32, expected: "INT"},
		{kind: reflect.Int64, expected: "INT"},
		{kind: reflect.Uint, expected: "INT"},
		{kind: reflect.Uint8, expected: "INT"},
		{kind: reflect.Uint16, expected: "INT"},
		{kind: reflect.Uint32, expected: "INT"},
		{kind: reflect.Uint64, expected: "INT"},
		{kind: reflect.Uintptr, expected: "INT"},
		{kind: reflect.Float32, expected: "FLOAT"},
		{kind: reflect.Float64, expected: "FLOAT"},
		{kind: reflect.Struct, expected: "VALUE"},
		{kind: reflect.Chan, expected: "VALUE"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected+"_"+tc.kind.String(), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, kindMetavar(tc.kind))
		})
	}
}
