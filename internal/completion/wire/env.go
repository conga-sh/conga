package wire

import (
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/conga-sh/conga/internal/envx"
)

// Environment variable names of the shell completion protocol: the bash-native
// variables and the CONGA_* sideband, plus the standard color variables.
const (
	// EnvCompLine is the bash-native current command line.
	EnvCompLine = "COMP_LINE"
	// EnvCompPoint is the bash-native cursor offset (bytes) into EnvCompLine.
	EnvCompPoint = "COMP_POINT"
	// EnvCompType is the bash-native readline completion type.
	EnvCompType = "COMP_TYPE"
	// EnvCompWordbreaks is the bash-native word-break character set.
	EnvCompWordbreaks = "COMP_WORDBREAKS"

	// CompletionAliasesSuffix is appended to the application prefix to form the
	// alias-suppression variable name (TASK_COMPLETION_ALIASES for a task app).
	CompletionAliasesSuffix = "COMPLETION_ALIASES"
	// CompletionBareSpaceFlagsSuffix is appended to the application prefix to
	// form the bare-space flag-restoration variable name
	// (TASK_COMPLETION_BARE_SPACE_FLAGS for a task app).
	CompletionBareSpaceFlagsSuffix = "COMPLETION_BARE_SPACE_FLAGS"
	// CompletionDescriptionsSuffix is appended to the application prefix to form
	// the description-suppression variable name.
	CompletionDescriptionsSuffix = "COMPLETION_DESCRIPTIONS"

	// EnvColor forces color styling on or off, parsed as a boolean.
	EnvColor = envx.EnvColor
	// EnvDescriptionLength limits the maximum description length in runes.
	EnvDescriptionLength = "CONGA_DESCRIPTION_LENGTH"
	// EnvMergeFlags merges the "shorthand flags" and "longhand flags" tags
	// into "flags" (zsh grouping).
	EnvMergeFlags = "CONGA_MERGE_FLAGS"
	// EnvShell forces the target shell, overriding detection.
	EnvShell = "CONGA_SHELL"
	// EnvTooltip enables tooltip-style description rendering (powershell).
	EnvTooltip = "CONGA_TOOLTIP"
	// EnvZshListSeparator overrides the separator zsh renders between a match
	// and its description (the user's list-separator style).
	EnvZshListSeparator = "CONGA_ZSH_LIST_SEPARATOR"
	// EnvZshQuote carries zsh's compstate[quote] for the word being
	// completed: the quote character the user opened ('\'' or '"'), or empty
	// when the word is unquoted.
	EnvZshQuote = "CONGA_ZSH_QUOTE"
	// EnvZshBanghist is "0" when the user's zsh has the banghist option off,
	// so a '!' inside double quotes needs no backslash.
	EnvZshBanghist = "CONGA_ZSH_BANGHIST"
	// EnvZshStyleLimit caps the number of candidates inline-styled in zsh.
	EnvZshStyleLimit = "CONGA_ZSH_STYLE_LIMIT"

	// EnvCLIColor is the standard CLICOLOR variable; "0" disables color.
	EnvCLIColor = envx.EnvCLIColor
	// EnvNoColor is the standard NO_COLOR variable; a non-empty value
	// disables color.
	EnvNoColor = envx.EnvNoColor
)

// AliasesEnabled reports whether alias candidates are shown: the
// application-prefixed variable (prefix + CompletionAliasesSuffix) parsed as a
// boolean. Unset or unparseable keeps the default (shown). An empty prefix has
// no application namespace to match, so it always keeps the default.
func AliasesEnabled(prefix string) bool {
	if prefix == "" {
		return true
	}

	return booleanSetting(prefix+CompletionAliasesSuffix, true)
}

// DescriptionsEnabled reports whether candidate descriptions are shown: the
// application-prefixed variable (prefix + CompletionDescriptionsSuffix) parsed
// as a boolean. Unset or unparseable keeps the default (shown). An empty prefix
// has no application namespace to match, so it always keeps the default.
func DescriptionsEnabled(prefix string) bool {
	if prefix == "" {
		return true
	}

	return booleanSetting(prefix+CompletionDescriptionsSuffix, true)
}

// BareSpaceFlagsEnabled reports whether flag candidates are restored at a bare
// space: the application-prefixed variable (prefix +
// CompletionBareSpaceFlagsSuffix) parsed as a boolean. Unset or unparseable
// keeps the default (suppressed); an empty prefix has no application namespace
// to match, so it also keeps the default.
func BareSpaceFlagsEnabled(prefix string) bool {
	if prefix == "" {
		return false
	}

	return booleanSetting(prefix+CompletionBareSpaceFlagsSuffix, false)
}

// ColorDisabled reports whether styled output is disabled. CONGA_COLOR parsed
// as a boolean decides first (true forces color, false disables it), then a
// non-empty NO_COLOR disables color, then CLICOLOR=0. Completion output goes
// to a pipe, so neither TERM nor TTY detection takes part.
func ColorDisabled() bool {
	return envx.ColorPreference() == envx.ColorOff
}

// DescriptionLength returns the maximum description length in runes:
// CONGA_DESCRIPTION_LENGTH when set to a positive integer, 80 otherwise.
func DescriptionLength() int {
	if parsed, err := strconv.Atoi(os.Getenv(EnvDescriptionLength)); err == nil && parsed > 0 {
		return parsed
	}

	return 80
}

// MergeFlags reports whether flag tag merging is requested (first return) and
// whether CONGA_MERGE_FLAGS was explicitly set to a parseable boolean (second
// return). Shells apply their own default when the variable is unset or
// unparseable.
func MergeFlags() (bool, bool) {
	return envx.EnvBool(EnvMergeFlags)
}

// Shell returns the CONGA_SHELL override, which forces the target shell and
// takes precedence over detection.
func Shell() string {
	return os.Getenv(EnvShell)
}

// Tooltip reports whether tooltip-style description rendering is requested
// (powershell): CONGA_TOOLTIP parsed as a boolean. Unset or unparseable keeps
// the default (off).
func Tooltip() bool {
	return booleanSetting(EnvTooltip, false)
}

// ZshQuote returns the quote the user opened in the word being completed,
// read from CONGA_ZSH_QUOTE: '\”, '"', or 0 for an unquoted word (or any
// other value).
func ZshQuote() byte {
	switch os.Getenv(EnvZshQuote) {
	case "'":
		return '\''
	case `"`:
		return '"'
	default:
		return 0
	}
}

// ZshBanghist reports whether zsh history expansion is on for the user's
// shell: true unless CONGA_ZSH_BANGHIST is "0".
func ZshBanghist() bool {
	return os.Getenv(EnvZshBanghist) != "0"
}

// ZshListSeparator returns the separator to embed in zsh list-colors patterns:
// CONGA_ZSH_LIST_SEPARATOR when it is a usable literal, "--" otherwise. A value
// that is empty, longer than 32 bytes, or contains a colon or control character
// falls back to the default: a colon would corrupt the colon-joined
// specification and control characters would corrupt the pattern.
func ZshListSeparator() string {
	value, ok := os.LookupEnv(EnvZshListSeparator)
	if !ok || value == "" || len(value) > 32 || strings.ContainsRune(value, ':') {
		return "--"
	}

	for _, r := range value {
		if unicode.IsControl(r) {
			return "--"
		}
	}

	return value
}

// ZshStyleLimit returns the maximum number of candidates to inline-style in
// zsh: CONGA_ZSH_STYLE_LIMIT when parseable, 300 otherwise.
func ZshStyleLimit() int {
	limit, err := strconv.Atoi(os.Getenv(EnvZshStyleLimit))
	if err != nil {
		return 300
	}

	return limit
}

// booleanSetting parses name as a boolean, returning fallback when it is unset
// or unparseable.
func booleanSetting(name string, fallback bool) bool {
	if value, ok := envx.EnvBool(name); ok {
		return value
	}

	return fallback
}
