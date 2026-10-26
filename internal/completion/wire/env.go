package wire

import (
	"os"
	"strconv"
	"strings"
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

	// EnvAliases shows ("1") or suppresses ("0") alias candidates.
	EnvAliases = "CONGA_COMPLETION_ALIASES"
	// EnvColor forces color styling on ("1") or off ("0").
	EnvColor = "CONGA_COLOR"
	// EnvDescriptionLength limits the maximum description length in runes.
	EnvDescriptionLength = "CONGA_DESCRIPTION_LENGTH"
	// EnvDescriptions shows ("1") or suppresses ("0") candidate descriptions.
	EnvDescriptions = "CONGA_COMPLETION_DESCRIPTIONS"
	// EnvMergeFlags merges the "shorthand flags" and "longhand flags" tags
	// into "flags" (zsh grouping).
	EnvMergeFlags = "CONGA_MERGE_FLAGS"
	// EnvShell forces the target shell, overriding detection.
	EnvShell = "CONGA_SHELL"
	// EnvTooltip enables tooltip-style description rendering (powershell).
	EnvTooltip = "CONGA_TOOLTIP"
	// EnvZshStyleLimit caps the number of candidates inline-styled in zsh.
	EnvZshStyleLimit = "CONGA_ZSH_STYLE_LIMIT"

	// EnvCLIColor is the standard CLICOLOR variable; "0" disables color.
	EnvCLIColor = "CLICOLOR"
	// EnvNoColor is the standard NO_COLOR variable; presence disables color.
	EnvNoColor = "NO_COLOR"
)

// AliasesEnabled reports whether alias candidates are shown:
// CONGA_COMPLETION_ALIASES parsed as a boolean; unset or unparseable keeps the
// default (shown).
func AliasesEnabled() bool {
	return booleanSetting(EnvAliases)
}

// DescriptionsEnabled reports whether candidate descriptions are shown:
// CONGA_COMPLETION_DESCRIPTIONS parsed as a boolean; unset or unparseable
// keeps the default (shown).
func DescriptionsEnabled() bool {
	return booleanSetting(EnvDescriptions)
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
