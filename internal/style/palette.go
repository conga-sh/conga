package style

// cubeLevels are the RGB levels of the xterm 6x6x6 color cube. The xterm
// 256-color palette structure (public specification): 16 standard colors, a
// 6x6x6 color cube with levels 0x00/0x5f/0x87/0xaf/0xd7/0xff (indices
// 16-231), and a 24-step grayscale ramp 8+10n (indices 232-255).
//
//nolint:gochecknoglobals // immutable, spec-defined palette table
var cubeLevels = [6]int{0x00, 0x5f, 0x87, 0xaf, 0xd7, 0xff}

// standard16 holds the RGB values of the xterm palette's 16 base colors
// (indices 0-15).
//
//nolint:gochecknoglobals // immutable, spec-defined palette table
var standard16 = [16][3]int{
	{0x00, 0x00, 0x00}, {0x80, 0x00, 0x00}, {0x00, 0x80, 0x00}, {0x80, 0x80, 0x00},
	{0x00, 0x00, 0x80}, {0x80, 0x00, 0x80}, {0x00, 0x80, 0x80}, {0xc0, 0xc0, 0xc0},
	{0x80, 0x80, 0x80}, {0xff, 0x00, 0x00}, {0x00, 0xff, 0x00}, {0xff, 0xff, 0x00},
	{0x00, 0x00, 0xff}, {0xff, 0x00, 0xff}, {0x00, 0xff, 0xff}, {0xff, 0xff, 0xff},
}

// RGBHex renders an RGB triple as a "#rrggbb" hex code.
func RGBHex(red, green, blue int) string {
	const hexDigits = "0123456789abcdef"

	return string([]byte{
		'#',
		hexDigits[red>>4], hexDigits[red&0xf],
		hexDigits[green>>4], hexDigits[green&0xf],
		hexDigits[blue>>4], hexDigits[blue&0xf],
	})
}

// IndexedHex returns the "#rrggbb" rendering of the xterm 256-color palette
// entry with the given index, and false when the index is out of range.
func IndexedHex(index int) (string, bool) {
	if index < 0 || index > 255 {
		return "", false
	}

	red, green, blue := paletteRGB(index)

	return RGBHex(red, green, blue), true
}

// NearestIndexed returns the index (0-255) of the xterm 256-color palette
// entry nearest to the given RGB triple by squared RGB distance; ties resolve
// to the lowest index.
func NearestIndexed(red int, green int, blue int) int {
	best := 0
	bestDistance := -1

	for index := range 256 {
		entryRed, entryGreen, entryBlue := paletteRGB(index)
		distance := colorDistance(red, green, blue, entryRed, entryGreen, entryBlue)

		if bestDistance < 0 || distance < bestDistance {
			best = index
			bestDistance = distance
		}
	}

	return best
}

// paletteRGB returns the RGB components of the xterm 256-color palette entry
// with the given index (0-255).
func paletteRGB(index int) (int, int, int) {
	switch {
	case index < 16:
		return standard16[index][0], standard16[index][1], standard16[index][2]
	case index < 232:
		position := index - 16

		return cubeLevels[(position/36)%6], cubeLevels[(position/6)%6], cubeLevels[position%6]
	default:
		value := 8 + (index-232)*10

		return value, value, value
	}
}

// colorDistance computes the squared RGB distance between two colors.
func colorDistance(redA, greenA, blueA, redB, greenB, blueB int) int {
	redDelta := redA - redB
	greenDelta := greenA - greenB
	blueDelta := blueA - blueB

	return redDelta*redDelta + greenDelta*greenDelta + blueDelta*blueDelta
}
