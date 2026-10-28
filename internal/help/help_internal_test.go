package help

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/termwidth"
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

// TestWithBuiltinShort pins the rule that the configured built-in rune only
// adorns engine-built-in rows: the binder registers it on the injected --help
// and --version flags alone, so a user-declared shadow advertising the same
// rune would promise a short the parser rejects.
func TestWithBuiltinShort(t *testing.T) {
	t.Parallel()

	t.Run("shared built-in help row receives the rune", func(t *testing.T) {
		t.Parallel()

		applied := withBuiltinShort(spec.BuiltinHelpFlag, 'h')

		assert.Equal(t, 'h', applied.Short)
		assert.Equal(t, rune(0), spec.BuiltinHelpFlag.Short)
	})

	t.Run("shared built-in version row receives the rune", func(t *testing.T) {
		t.Parallel()

		applied := withBuiltinShort(spec.BuiltinVersionFlag, 'V')

		assert.Equal(t, 'V', applied.Short)
		assert.Equal(t, rune(0), spec.BuiltinVersionFlag.Short)
	})

	t.Run("user-declared shadow keeps no short", func(t *testing.T) {
		t.Parallel()

		shadow := &spec.Flag{Name: "help", IsBool: true}

		applied := withBuiltinShort(shadow, 'h')

		assert.True(t, applied == shadow)
		assert.Equal(t, rune(0), applied.Short)
	})

	t.Run("engine-owned non-builtin shadow keeps no short", func(t *testing.T) {
		t.Parallel()

		shadow := &spec.Flag{Name: "help", IsMap: true, EngineOwned: true}

		applied := withBuiltinShort(shadow, 'h')

		assert.True(t, applied == shadow)
		assert.Equal(t, rune(0), applied.Short)
	})

	t.Run("flag with its own short keeps it", func(t *testing.T) {
		t.Parallel()

		applied := withBuiltinShort(&spec.Flag{Name: "help", IsBool: true, Short: 'x'}, 'h')

		assert.Equal(t, 'x', applied.Short)
	})

	t.Run("zero configured rune is a no-op", func(t *testing.T) {
		t.Parallel()

		applied := withBuiltinShort(spec.BuiltinHelpFlag, 0)

		assert.True(t, applied == spec.BuiltinHelpFlag)
		assert.Equal(t, rune(0), applied.Short)
	})
}

// TestFormatFlags_BuiltinShortRows asserts the rendered rows: the completion
// command's shared --help row carries the configured rune, while a
// user-declared help shadow renders long-only so the screen matches the parser.
func TestFormatFlags_BuiltinShortRows(t *testing.T) {
	t.Parallel()

	t.Run("shared built-in help row renders the configured short", func(t *testing.T) {
		t.Parallel()

		rendered := formatFlags([]*spec.Flag{spec.BuiltinHelpFlag}, false, false, false, false, nil, 'h', 0, termwidth.Default)

		assert.Contains(t, rendered, "-h, --help")
	})

	t.Run("user-declared help shadow renders long-only", func(t *testing.T) {
		t.Parallel()

		rendered := formatFlags([]*spec.Flag{{Name: "help", IsBool: true, Help: "Custom help"}}, false, false, false, false, nil, 'h', 0, termwidth.Default)

		assert.Contains(t, rendered, "--help")
		assert.Contains(t, rendered, "Custom help")
		assert.NotContains(t, rendered, "-h, --help")
	})
}

func TestCellWidth(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		text     string
		expected int
	}{
		{name: "ascii", text: "abc", expected: 3},
		{name: "wide CJK", text: "界面", expected: 4},
		{name: "fullwidth", text: "Ａ", expected: 2},
		{name: "combining mark", text: "e\u0301", expected: 1},
		{name: "format control", text: "\u200b", expected: 0},
		{name: "emoji", text: "😀", expected: 2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, cellWidth(testCase.text))
		})
	}
}

func TestWrapLine(t *testing.T) {
	t.Parallel()

	t.Run("reversed brackets fall back to words", func(t *testing.T) {
		t.Parallel()

		lines := wrapLine("] one two three [", 10)

		assert.Equal(t, []string{"] one two", "three ["}, lines)
	})

	t.Run("wide runes count as two cells", func(t *testing.T) {
		t.Parallel()

		line := "界 面 文 字 漢 語 日 本 中 華"
		lines := wrapLine(line, 20)

		assert.Equal(t, 2, len(lines))
		assert.Equal(t, line, strings.Join(lines, " "))

		for _, wrapped := range lines {
			assert.True(t, cellWidth(wrapped) <= 20)
		}
	})
}

func TestWrapCell(t *testing.T) {
	t.Parallel()

	t.Run("preserves explicit line breaks and blank lines", func(t *testing.T) {
		t.Parallel()

		lines := wrapCell("First line.\n\nSecond paragraph that is long enough to wrap.", 20)

		assert.Equal(t, []string{"First line.", "", "Second paragraph", "that is long enough", "to wrap."}, lines)
	})

	t.Run("preserves a leading blank line", func(t *testing.T) {
		t.Parallel()

		lines := wrapCell("\nFirst line.", 20)

		assert.Equal(t, []string{"", "First line."}, lines)
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
