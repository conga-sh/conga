package wire

import (
	"os"
	"strconv"
	"strings"
	"unicode"
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

	// EnvColor forces color styling on ("1") or off ("0").
	EnvColor = "CONGA_COLOR"
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
	// EnvZshStyleLimit caps the number of candidates inline-styled in zsh.
	EnvZshStyleLimit = "CONGA_ZSH_STYLE_LIMIT"

	// EnvCLIColor is the standard CLICOLOR variable; "0" disables color.
	EnvCLIColor = "CLICOLOR"
	// EnvNoColor is the standard NO_COLOR variable; presence disables color.
	EnvNoColor = "NO_COLOR"
)

// AliasesEnabled reports whether alias candidates are shown: the
// application-prefixed variable (prefix + CompletionAliasesSuffix) parsed as a
// boolean. Unset or unparseable keeps the default (shown). An empty prefix has
// no application namespace to match, so it always keeps the default.
func AliasesEnabled(prefix string) bool {
	if prefix == "" {
		return true
	}

	return booleanSetting(prefix + CompletionAliasesSuffix)
}

// DescriptionsEnabled reports whether candidate descriptions are shown: the
// application-prefixed variable (prefix + CompletionDescriptionsSuffix) parsed
// as a boolean. Unset or unparseable keeps the default (shown). An empty prefix
// has no application namespace to match, so it always keeps the default.
func DescriptionsEnabled(prefix string) bool {
	if prefix == "" {
		return true
	}

	return booleanSetting(prefix + CompletionDescriptionsSuffix)
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

	value, ok := os.LookupEnv(prefix + CompletionBareSpaceFlagsSuffix)
	if !ok {
		return false
	}

	parsed, err := strconv.ParseBool(value)

	return err == nil && parsed
}

// ColorDisabled reports whether styled output is disabled: CONGA_COLOR is
// honored first ("0" disables, any other non-empty value forces color), then
// the NO_COLOR standard (present and non-empty disables), then CLICOLOR=0.
func ColorDisabled() bool {
	if value, ok := os.LookupEnv(EnvColor); ok && value != "" {
		return value == "0"
	}

	if value, ok := os.LookupEnv(EnvNoColor); ok && value != "" {
		return true
	}

	return os.Getenv(EnvCLIColor) == "0"
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
	value, ok := os.LookupEnv(EnvMergeFlags)
	if !ok {
		return false, false
	}

	parsed, err := strconv.ParseBool(value)

	return parsed, err == nil
}

// Shell returns the CONGA_SHELL override, which forces the target shell and
// takes precedence over detection.
func Shell() string {
	return os.Getenv(EnvShell)
}

// Tooltip reports whether tooltip-style description rendering is requested
// (powershell): CONGA_TOOLTIP set to a truthy value.
func Tooltip() bool {
	switch strings.ToLower(os.Getenv(EnvTooltip)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
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

// booleanSetting parses name as a boolean, defaulting to true when it is unset
// or unparseable.
func booleanSetting(name string) bool {
	value, ok := os.LookupEnv(name)
	if !ok {
		return true
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}

	return parsed
}
