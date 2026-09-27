package style_test

import (
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/style"
)

func TestThemePaletteDefaults(t *testing.T) {
	t.Parallel()

	assert.Equal(t, style.Default, style.Theme.Value)
	assert.Equal(t, style.Dim, style.Theme.Description)
	assert.Equal(t, "bold red", style.Theme.Error)
	assert.Equal(t, style.Dim, style.Theme.Usage)

	assert.Equal(t, style.Blue, style.Theme.Highlights[0])
	assert.Equal(t, style.Yellow, style.Theme.Highlights[1])
	assert.Equal(t, style.Magenta, style.Theme.Highlights[2])
	assert.Equal(t, style.Cyan, style.Theme.Highlights[3])
	assert.Equal(t, style.Green, style.Theme.Highlights[4])
	assert.Equal(t, "dim blue", style.Theme.Highlights[5])
	assert.Equal(t, "dim yellow", style.Theme.Highlights[6])
	assert.Equal(t, "dim magenta", style.Theme.Highlights[7])
	assert.Equal(t, "dim cyan", style.Theme.Highlights[8])
	assert.Equal(t, "dim green", style.Theme.Highlights[9])
	assert.Equal(t, style.Bold, style.Theme.Highlights[10])
	assert.Equal(t, "dim bold", style.Theme.Highlights[11])

	assert.Equal(t, style.Blue, style.Theme.FlagArg)
	assert.Equal(t, style.Magenta, style.Theme.FlagMultiArg)
	assert.Equal(t, style.Default, style.Theme.FlagNoArg)
	assert.Equal(t, style.Yellow, style.Theme.FlagOptArg)
}

func TestThemeHighlightLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		level    int
		expected string
	}{
		{level: 0, expected: style.Blue},
		{level: 1, expected: style.Yellow},
		{level: 2, expected: style.Magenta},
		{level: 3, expected: style.Cyan},
		{level: 4, expected: style.Green},
		{level: 5, expected: "dim blue"},
		{level: 6, expected: "dim yellow"},
		{level: 7, expected: "dim magenta"},
		{level: 8, expected: "dim cyan"},
		{level: 9, expected: "dim green"},
		{level: 10, expected: style.Bold},
		{level: 11, expected: "dim bold"},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.level), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, style.Theme.Highlight(tt.level))
		})
	}
}

func TestThemeHighlightOutOfRange(t *testing.T) {
	t.Parallel()

	tests := []int{-1, 12, 99}

	for _, level := range tests {
		t.Run(strconv.Itoa(level), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, style.Default, style.Theme.Highlight(level))
		})
	}
}
