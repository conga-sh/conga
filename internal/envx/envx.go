// Package envx reads the on/off switches the engine takes from the process
// environment: the completion sideband booleans and the colour preference
// shared by the completion drivers and the --why report. A boolean switch is
// parsed with strconv.ParseBool, the parser behind boolean flags and features,
// and an unset, empty, or unrecognised value leaves the caller's default in
// place. The package has no package-level state, so importing it from an
// opt-in subsystem adds no initializer to the binary.
package envx

import (
	"os"
	"strconv"
)

// Colour environment variables.
const (
	// EnvColor forces colour styling on (a true boolean) or off (a false
	// boolean). Any other value defers to EnvNoColor and EnvCLIColor.
	EnvColor = "CONGA_COLOR"
	// EnvCLIColor is the standard CLICOLOR variable; "0" disables colour.
	EnvCLIColor = "CLICOLOR"
	// EnvNoColor is the standard NO_COLOR variable; a non-empty value
	// disables colour.
	EnvNoColor = "NO_COLOR"
)

// Colour preferences returned by ColorPreference.
const (
	// ColorDefault means the environment states no preference, so the caller
	// applies its own rule (TTY detection for --why, colour on for
	// completion output).
	ColorDefault Color = iota
	// ColorOn means CONGA_COLOR forces colour on.
	ColorOn
	// ColorOff means CONGA_COLOR, NO_COLOR, or CLICOLOR=0 disables colour.
	ColorOff
)

// Color is the colour preference the environment expresses.
type Color int

// ColorPreference returns the colour preference of the environment. A
// CONGA_COLOR boolean decides first, then a non-empty NO_COLOR disables
// colour, then CLICOLOR=0 disables it. Otherwise it returns ColorDefault.
func ColorPreference() Color {
	if forced, ok := EnvBool(EnvColor); ok {
		if forced {
			return ColorOn
		}

		return ColorOff
	}

	if os.Getenv(EnvNoColor) != "" || os.Getenv(EnvCLIColor) == "0" {
		return ColorOff
	}

	return ColorDefault
}

// EnvBool reads the environment variable name as a boolean with
// strconv.ParseBool. It returns the parsed value and whether the variable held
// a value ParseBool accepts. Both are false when the variable is unset, empty,
// or holds a value ParseBool rejects.
func EnvBool(name string) (bool, bool) {
	raw, set := os.LookupEnv(name)
	if !set {
		return false, false
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false
	}

	return parsed, true
}
