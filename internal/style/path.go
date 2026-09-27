package style

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//nolint:gochecknoglobals // one-time lazy construction of the LS_COLORS cache below
var getLsColorsOnce sync.Once

//nolint:gochecknoglobals // immutable after getLsColorsOnce completes
var getLsColorsValue map[string]string

// ForExtension returns the style for a given file extension (e.g. "json",
// "go"). The leading dot is optional and matching is case-insensitive. An
// LS_COLORS entry ("*.<ext>") takes precedence and is returned verbatim;
// otherwise the documented built-in extension map applies; otherwise candidates
// reset to standard terminal styling.
func ForExtension(extension string) string {
	extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
	if s, ok := getLsColors()["*."+extension]; ok {
		return s
	}

	if s := builtinExtensionStyle(extension); s != "" {
		return s
	}

	return Default
}

// ForPathExt returns the style for a given path based on its extension.
// Compound archive extensions (.tar.gz, .tar.bz2, .tar.xz) are matched before
// the final path element extension.
func ForPathExt(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return ForExtension("tar.gz")
	case strings.HasSuffix(lower, ".tar.bz2"):
		return ForExtension("tar.bz2")
	case strings.HasSuffix(lower, ".tar.xz"):
		return ForExtension("tar.xz")
	}

	if extension := filepath.Ext(path); extension != "" {
		return ForExtension(extension)
	}

	return Default
}

// getLsColors returns the one-time LS_COLORS environment cache. Construction
// is deferred to first use so the package stays free of package initializers
// and drops at link time when completion styling is disabled.
func getLsColors() map[string]string {
	getLsColorsOnce.Do(func() {
		getLsColorsValue = parseLsColors(os.Getenv("LS_COLORS"))
	})

	return getLsColorsValue
}

// builtinExtensionStyle maps common file extension groups to styles.
func builtinExtensionStyle(extension string) string {
	switch extension {
	case "sh", "bash", "zsh", "exe", "bat", "cmd":
		return Green

	case "tar", "gz", "tar.gz", "tgz", "zip", "bz2", "tar.bz2", "xz", "tar.xz", "7z", "rar":
		return Red

	case "png", "jpg", "jpeg", "gif", "svg", "webp", "ico":
		return Magenta

	case "mp3", "mp4", "wav", "flac", "mkv", "avi", "mov":
		return Cyan

	case "json", "yaml", "yml", "toml", "xml", "csv":
		return Yellow

	case "go", "rs", "py", "js", "ts", "c", "cpp", "h":
		return Blue

	default:
		return ""
	}
}

func parseLsColors(value string) map[string]string {
	colors := make(map[string]string)
	if value == "" {
		return colors
	}

	for entry := range strings.SplitSeq(value, ":") {
		pattern, s, found := strings.Cut(entry, "=")
		if !found || pattern == "" {
			continue
		}

		colors[strings.ToLower(pattern)] = s
	}

	return colors
}
