package style_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/style"
)

func TestForExtensionBuiltinGroups(t *testing.T) {
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

			assert.Equal(t, expected, style.ForExtension(extension))
		})
	}
}

func TestForExtensionNormalization(t *testing.T) {
	t.Parallel()

	assert.Equal(t, style.Blue, style.ForExtension(".CPP"))
	assert.Equal(t, style.Blue, style.ForExtension("  cpp  "))
	assert.Equal(t, style.Red, style.ForExtension("TAR.GZ"))
	assert.Equal(t, style.Default, style.ForExtension(""))
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

func TestForExtensionLsColorsPrecedence(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "01;36", style.ForExtension("go"), "LS_COLORS entry must take precedence over the built-in map")
	assert.Equal(t, "044", style.ForExtension("json"), "LS_COLORS entry must take precedence over the built-in map")
	assert.Equal(t, "01;31", style.ForExtension("TAR"), "extension matching must be case-insensitive against lowercased LS_COLORS keys")
	assert.Equal(t, style.Magenta, style.ForExtension("png"), "extensions without an LS_COLORS entry must fall back to the built-in map")
	assert.Equal(t, style.Default, style.ForExtension("unknownext"), "extensions without an LS_COLORS entry or built-in style must reset styling")
}

func TestForPathExtLsColorsPrecedence(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "01;36", style.ForPathExt("main.go"), "ForPathExt must consult LS_COLORS through ForExtension")
	assert.Equal(t, style.Red, style.ForPathExt("archive.tar.gz"), "compound archive extensions without an LS_COLORS entry must use the built-in map")
}
