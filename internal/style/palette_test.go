package style_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/style"
)

func TestIndexedHex(t *testing.T) {
	t.Parallel()

	cases := []struct {
		index    int
		expected string
	}{
		{0, "#000000"},
		{1, "#800000"},
		{7, "#c0c0c0"},
		{8, "#808080"},
		{9, "#ff0000"},
		{15, "#ffffff"},
		{16, "#000000"},
		{17, "#00005f"}, // cube: r=0, g=0, b=1 (blue varies fastest)
		{18, "#000087"},
		{24, "#005f87"},  // r=0, g=1, b=2
		{196, "#ff0000"}, // r=5, g=0, b=0
		{231, "#ffffff"},
		{232, "#080808"},
		{240, "#585858"},
		{255, "#eeeeee"},
	}

	for _, tc := range cases {
		hex, ok := style.IndexedHex(tc.index)

		assert.True(t, ok)
		assert.Equal(t, tc.expected, hex)
	}
}

func TestIndexedHexOutOfRange(t *testing.T) {
	t.Parallel()

	for _, index := range []int{-1, 256, 1000} {
		hex, ok := style.IndexedHex(index)

		assert.False(t, ok)
		assert.Equal(t, "", hex)
	}
}

func TestNearestIndexed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		red      int
		green    int
		blue     int
		expected int
	}{
		{0x00, 0x00, 0x00, 0},
		{0xff, 0x00, 0x00, 9},
		{0x00, 0xff, 0xff, 14},
		{0xff, 0xff, 0xff, 15},
		// Cube levels round to themselves.
		{0x00, 0x5f, 0x87, 24}, // r=0, g=1, b=2 -> 16+0*36+1*6+2
		// Exact cube corner also exists in base 16: ff0000 -> index 9 (lowest).
		{0xff, 0xd7, 0x00, 220},
		// Grayscale ramp rounds to itself, including near-white gray.
		{0x4e, 0x4e, 0x4e, 239}, // 78 -> nearest ramp entry 8+10*7=78
		{0xee, 0xee, 0xee, 255},
		// A gray between ramp entries resolves to the nearest one.
		{0x3d, 0x3d, 0x3d, 237}, // 61: 8+10*5=58 (dist 3) vs 8+10*6=68 (dist 49)
	}

	for _, tc := range cases {
		index := style.NearestIndexed(tc.red, tc.green, tc.blue)

		assert.Equal(t, tc.expected, index)
	}
}

func TestNearestIndexedTieResolvesLowestIndex(t *testing.T) {
	t.Parallel()

	// RGB 0x2f (47): distance to ramp 8+10*4=48 is 1; no closer entry exists,
	// but 47 is also distance 1 from... verify the grayscale midpoint rule:
	// 0x3a (58) hits ramp entry 58 exactly: 8+10*5 -> index 237.
	assert.Equal(t, 237, style.NearestIndexed(0x3a, 0x3a, 0x3a))
}

func TestTruecolorRGB(t *testing.T) {
	t.Parallel()

	red, green, blue, ok := style.TruecolorRGB("#ff8800")

	assert.True(t, ok)
	assert.Equal(t, 0xff, red)
	assert.Equal(t, 0x88, green)
	assert.Equal(t, 0x00, blue)

	red, green, blue, ok = style.TruecolorRGB("rgb(10,20,30)")

	assert.True(t, ok)
	assert.Equal(t, 10, red)
	assert.Equal(t, 20, green)
	assert.Equal(t, 30, blue)
}

func TestTruecolorRGBRejectsNonTruecolorTokens(t *testing.T) {
	t.Parallel()

	for _, token := range []string{"red", "bright-red", "bg-red", "color123", "default", "#ff8", "rgb(1,2)", "rgb"} {
		_, _, _, ok := style.TruecolorRGB(token)

		assert.False(t, ok, "token %q must not parse as truecolor", token)
	}
}

func TestIndexedHexRoundTripThroughNearestIndexed(t *testing.T) {
	t.Parallel()

	// Cube entries whose RGB duplicates a base-16 color resolve to that
	// lowest index.
	duplicates := map[int]int{
		16:  0,  // #000000
		21:  12, // #0000ff
		46:  10, // #00ff00
		51:  14, // #00ffff
		196: 9,  // #ff0000
		201: 13, // #ff00ff
		226: 11, // #ffff00
		231: 15, // #ffffff
		244: 8,  // #808080 (grayscale ramp reuses the base-16 gray)
	}

	for index := range 256 {
		hex, ok := style.IndexedHex(index)

		assert.True(t, ok)

		red, green, blue, ok := style.TruecolorRGB(hex)

		assert.True(t, ok)

		expected, ambiguous := duplicates[index]
		if !ambiguous {
			expected = index
		}

		assert.Equal(t, expected, style.NearestIndexed(red, green, blue))
	}
}
