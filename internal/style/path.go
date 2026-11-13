package style

import (
	"cmp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

//nolint:gochecknoglobals // one-time lazy construction of the LS_COLORS cache below
var getLsColorsOnce sync.Once

//nolint:gochecknoglobals // immutable after getLsColorsOnce completes
var getLsColorsValue lsColors

// ForPathExt returns the style for a regular file by its name. The last
// LS_COLORS "*<suffix>" entry the name ends with wins ("*.go", "*.tar.gz",
// "*README"), matched case-insensitively and translated from its SGR codes
// into the style vocabulary ("01;36" → "bold cyan"), as ls does. Without a
// matching entry the documented built-in extension map applies, compound
// archive extensions (.tar.gz, .tar.bz2, .tar.xz) before the final one;
// otherwise candidates reset to standard terminal styling.
func ForPathExt(path string) string {
	lower := strings.ToLower(path)

	if s, ok := getLsColors().suffixStyle(lower); ok {
		return s
	}

	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return builtinExtensionStyle("tar.gz")
	case strings.HasSuffix(lower, ".tar.bz2"):
		return builtinExtensionStyle("tar.bz2")
	case strings.HasSuffix(lower, ".tar.xz"):
		return builtinExtensionStyle("tar.xz")
	}

	return builtinExtensionStyle(strings.TrimPrefix(filepath.Ext(lower), "."))
}

// ForDirectory returns the style for a directory: the LS_COLORS "di" entry,
// otherwise bold blue.
func ForDirectory() string {
	return getLsColors().directoryStyle()
}

// ForExecutable returns the style for an executable file: the LS_COLORS "ex"
// entry, otherwise green. As in ls, it outranks the file's suffix entry.
func ForExecutable() string {
	return getLsColors().executableStyle()
}

// ForSymlink returns the style for a symbolic link whose target exists: the
// LS_COLORS "ln" entry, otherwise cyan. Callers check SymlinkAsTarget first.
func ForSymlink() string {
	return getLsColors().symlinkStyle()
}

// ForOrphanSymlink returns the style for a symbolic link whose target is
// missing: the LS_COLORS "or" entry, otherwise the link style. Under
// "ln=target" a missing target has no style to borrow, so the link renders
// plain ("fg-default"), which also overrides the theme's value style.
func ForOrphanSymlink() string {
	return getLsColors().orphanSymlinkStyle()
}

// SymlinkAsTarget reports whether LS_COLORS sets "ln=target": a link whose
// target exists is then styled as its target would be (directory,
// executable, or by the target's name).
func SymlinkAsTarget() bool {
	return getLsColors().symlinkAsTarget
}

// lsColors holds the LS_COLORS entries the path styles consult: the
// directory, symlink, orphan, and executable indicators (already translated;
// "" when unset) and the "*<suffix>" entries. A suffix that is a plain
// extension (".go": one leading dot, no other dot or separator) sits in
// extensions, keyed by the suffix, so a lookup costs one map access however
// many entries LS_COLORS has; every other suffix (".tar.gz", "README", "~")
// sits in suffixes in declaration order.
type lsColors struct {
	directory       string
	symlink         string
	orphan          string
	executable      string
	symlinkAsTarget bool
	extensions      map[string]lsColorsSuffix
	suffixes        []lsColorsSuffix
	suffixCount     int
}

// lsColorsSuffix is one "*<suffix>" entry: the lowercased suffix, its
// translated style, and its position among the suffix entries, which decides
// between a matching extension and a matching other suffix.
type lsColorsSuffix struct {
	suffix string
	style  string
	order  int
}

func (c lsColors) directoryStyle() string {
	return cmp.Or(c.directory, Of(Bold, Blue))
}

func (c lsColors) executableStyle() string {
	return cmp.Or(c.executable, Green)
}

func (c lsColors) symlinkStyle() string {
	return cmp.Or(c.symlink, Cyan)
}

func (c lsColors) orphanSymlinkStyle() string {
	switch {
	case c.orphan != "":
		return c.orphan
	case c.symlinkAsTarget:
		return fgDefault
	default:
		return c.symlinkStyle()
	}
}

// suffixStyle returns the style of the last suffix entry lowerName ends
// with: a later LS_COLORS entry overrides an earlier one, as in ls. A plain
// extension entry matches exactly when it equals the name's extension, so
// the extension map yields the latest such entry; only other suffix entries
// declared after it can still win, and the scan stops at the first one
// declared before it.
func (c lsColors) suffixStyle(lowerName string) (string, bool) {
	extension, found := c.extensions[filepath.Ext(lowerName)]

	for i := len(c.suffixes) - 1; i >= 0; i-- {
		if found && c.suffixes[i].order < extension.order {
			break
		}

		if strings.HasSuffix(lowerName, c.suffixes[i].suffix) {
			return c.suffixes[i].style, true
		}
	}

	return extension.style, found
}

// add records one "*<suffix>" entry, numbering it in declaration order and
// routing a plain extension to the extension map, where a later duplicate
// replaces an earlier one.
func (c *lsColors) add(suffix, style string) {
	entry := lsColorsSuffix{suffix: suffix, style: style, order: c.suffixCount}
	c.suffixCount++

	if !isPlainExtension(suffix) {
		c.suffixes = append(c.suffixes, entry)

		return
	}

	if c.extensions == nil {
		c.extensions = make(map[string]lsColorsSuffix)
	}

	c.extensions[suffix] = entry
}

// getLsColors returns the one-time LS_COLORS environment cache. Construction
// is deferred to first use so the package stays free of package initializers
// and drops at link time when completion styling is disabled.
func getLsColors() lsColors {
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
		return Default
	}
}

// parseLsColors reads the entries of an LS_COLORS value. Indicators other
// than di, ln, or, and ex are ignored, as are malformed entries and a bare
// "*" (an empty suffix).
func parseLsColors(value string) lsColors {
	var colors lsColors

	for entry := range strings.SplitSeq(value, ":") {
		key, sgr, found := strings.Cut(entry, "=")
		if !found || key == "" {
			continue
		}

		if suffix, ok := strings.CutPrefix(key, "*"); ok {
			if suffix != "" {
				colors.add(strings.ToLower(suffix), lsColorsStyle(sgr))
			}

			continue
		}

		switch key {
		case "di":
			colors.directory = lsColorsStyle(sgr)
		case "ln":
			colors.symlinkAsTarget = sgr == "target"
			colors.symlink = ""

			if !colors.symlinkAsTarget {
				colors.symlink = lsColorsStyle(sgr)
			}
		case "or":
			colors.orphan = lsColorsStyle(sgr)
		case "ex":
			colors.executable = lsColorsStyle(sgr)
		}
	}

	return colors
}

// lsColorsStyle translates an LS_COLORS SGR parameter list ("01;36",
// "38;5;208") into the style vocabulary ("bold cyan", "color208"), so every
// consumer of the style string sees tokens it understands. Attributes 1, 2,
// 3, 4, 5, and 7, the standard and bright colors, 39 and 49 (default
// foreground and background), and the 256-color and truecolor extended forms
// (38/48;5;n and 38/48;2;r;g;b) are translated. A 0 (reset) clears the
// tokens collected before it, as it does on the terminal. Every other code
// (8 concealed, 9 crossed out, malformed numbers) is dropped: the vocabulary
// has no token for it. An entry that leaves no token renders plain
// ("fg-default"), so it still overrides the built-in extension map and the
// callers' fallback colors.
func lsColorsStyle(sgr string) string {
	codes := strings.Split(sgr, ";")
	tokens := make([]string, 0, len(codes))

	for i := 0; i < len(codes); i++ {
		code, err := strconv.Atoi(codes[i])
		if err != nil {
			continue
		}

		if code == 0 {
			tokens = tokens[:0]

			continue
		}

		if code == 38 || code == 48 {
			token, consumed := extendedColorToken(codes[i+1:])
			i += consumed

			if token != "" {
				if code == 48 {
					token = bgPrefix + token
				}

				tokens = append(tokens, token)
			}

			continue
		}

		if token := sgrToken(code); token != "" {
			tokens = append(tokens, token)
		}
	}

	if len(tokens) == 0 {
		return fgDefault
	}

	return strings.Join(tokens, " ")
}

// sgrToken maps one plain SGR code to its vocabulary token, or "" when the
// vocabulary has none.
func sgrToken(code int) string {
	switch {
	case code == 1:
		return Bold
	case code == 2:
		return Dim
	case code == 3:
		return Italic
	case code == 4:
		return Underlined
	case code == 5:
		return Blink
	case code == 7:
		return Inverse
	case code == sgrDefaultFG:
		return fgDefault
	case code == sgrDefaultBG:
		return bgDefault
	case code >= fgStandardBase && code < fgStandardBase+len(standardColors):
		return standardColors[code-fgStandardBase]
	case code >= bgStandardBase && code < bgStandardBase+len(standardColors):
		return bgPrefix + standardColors[code-bgStandardBase]
	case code >= fgStandardBase+brightDelta && code < fgStandardBase+brightDelta+len(standardColors):
		return brightPrefix + standardColors[code-fgStandardBase-brightDelta]
	case code >= bgStandardBase+brightDelta && code < bgStandardBase+brightDelta+len(standardColors):
		return bgPrefix + brightPrefix + standardColors[code-bgStandardBase-brightDelta]
	default:
		return ""
	}
}

// extendedColorToken reads the parameters after a 38 or 48 code: "5;n"
// becomes "colorN" and "2;r;g;b" becomes "#rrggbb". It returns the token
// ("" when the parameters are malformed or out of range) and how many
// parameters it consumed. An unknown or malformed mode gives no way to tell
// how many parameters belong to it, so it consumes the rest of the list.
func extendedColorToken(rest []string) (string, int) {
	if len(rest) == 0 {
		return "", 0
	}

	mode, err := strconv.Atoi(rest[0])
	if err != nil {
		return "", len(rest)
	}

	switch mode {
	case 5:
		if len(rest) < 2 {
			return "", len(rest)
		}

		index, ok := sgrByte(rest[1])
		if !ok {
			return "", 2
		}

		return "color" + strconv.Itoa(index), 2
	case 2:
		if len(rest) < 4 {
			return "", len(rest)
		}

		red, okRed := sgrByte(rest[1])
		green, okGreen := sgrByte(rest[2])
		blue, okBlue := sgrByte(rest[3])

		if !okRed || !okGreen || !okBlue {
			return "", 4
		}

		return RGBHex(red, green, blue), 4
	default:
		return "", len(rest)
	}
}

// sgrByte parses one SGR parameter in the 0–255 range.
func sgrByte(parameter string) (int, bool) {
	value, err := strconv.Atoi(parameter)
	if err != nil || value < 0 || value > 255 {
		return 0, false
	}

	return value, true
}

// isPlainExtension reports whether an LS_COLORS suffix is a plain extension:
// a leading dot and no other dot or path separator, so a name ends with it
// exactly when filepath.Ext returns it.
func isPlainExtension(suffix string) bool {
	return strings.HasPrefix(suffix, ".") && !strings.ContainsAny(suffix[1:], `./\`)
}
