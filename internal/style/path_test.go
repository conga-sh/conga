package style_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/style"
)

func TestForPathExtBuiltinGroups(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"sh": style.Green, "exe": style.Green, "bat": style.Green,
		"tar.gz": style.Red, "tgz": style.Red, "7z": style.Red, "zip": style.Red,
		"png": style.Magenta, "svg": style.Magenta, "webp": style.Magenta,
		"mp3": style.Cyan, "mkv": style.Cyan, "mov": style.Cyan,
		"yaml": style.Yellow, "toml": style.Yellow, "xml": style.Yellow,
		"cpp": style.Blue, "rs": style.Blue, "ts": style.Blue,
		"unknownext": style.Default,
	}

	for extension, expected := range tests {
		t.Run(extension, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, expected, style.ForPathExt("file."+extension))
		})
	}
}

func TestForPathExtBuiltinCaseInsensitive(t *testing.T) {
	t.Parallel()

	assert.Equal(t, style.Blue, style.ForPathExt("MAIN.CPP"))
	assert.Equal(t, style.Red, style.ForPathExt("ARCHIVE.TAR.GZ"))
	assert.Equal(t, style.Default, style.ForPathExt(""))
}

func TestForPathExt(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"script.sh":          style.Green,
		"archive.tar.gz":     style.Red,
		"archive.TAR.BZ2":    style.Red,
		"backup.tar.xz":      style.Red,
		"photo.PNG":          style.Magenta,
		"notes.yaml":         style.Yellow,
		"/path/to/notes.yml": style.Yellow,
		"noext":              style.Default,
		"README":             style.Default,
	}

	for path, expected := range tests {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, expected, style.ForPathExt(path))
		})
	}
}

func TestForPathExtLsColorsPrecedence(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "bold cyan", style.ForPathExt("main.go"), "LS_COLORS entry must take precedence over the built-in map")
	assert.Equal(t, "bg-blue", style.ForPathExt("data.json"), "LS_COLORS entry must take precedence over the built-in map")
	assert.Equal(t, "bold red", style.ForPathExt("backup.tar"), "suffix matching must be case-insensitive")
	assert.Equal(t, "color208", style.ForPathExt("app.log"), "a 256-color LS_COLORS entry must translate to an indexed color token")
	assert.Equal(t, "fg-default", style.ForPathExt("run.cmd"), "a reset-only LS_COLORS entry must render plain over the built-in map")
	assert.Equal(t, style.Magenta, style.ForPathExt("photo.png"), "names without an LS_COLORS entry must fall back to the built-in map")
	assert.Equal(t, style.Red, style.ForPathExt("archive.tar.gz"), "compound archive extensions without an LS_COLORS entry must use the built-in map")
	assert.Equal(t, style.Default, style.ForPathExt("file.unknownext"), "names without an LS_COLORS entry or built-in style must reset styling")
}

func TestForPathExtLsColorsRendersSGR(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "1;36", style.SGR(style.ForPathExt("main.go")))
	assert.Equal(t, "44", style.SGR(style.ForPathExt("data.json")))
	assert.Equal(t, "38;5;208", style.SGR(style.ForPathExt("app.log")))
	assert.Equal(t, "39", style.SGR(style.ForPathExt("run.cmd")))
}

func TestKindStylesFollowLsColors(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "bold yellow", style.ForDirectory())
	assert.Equal(t, "underlined green", style.ForExecutable())
	assert.Equal(t, "magenta", style.ForSymlink())
	assert.Equal(t, "bg-red", style.ForOrphanSymlink())
	assert.False(t, style.SymlinkAsTarget())
}
