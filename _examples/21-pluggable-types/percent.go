package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// errInvalidLoad is the static sentinel for unparseable load percentages.
var errInvalidLoad = errors.New("invalid load percentage")

// Percent is a load percentage parsed from values like "75%". The type is
// foreign to CONGA and deliberately does not implement TextUnmarshaler, so
// it is wired via App.RegisterType in main.go instead.
type Percent int

// parsePercent parses "0%".."100%" values for App.RegisterType.
func parsePercent(raw string) (Percent, error) {
	value, err := strconv.Atoi(strings.TrimSuffix(raw, "%"))
	if err != nil || value < 0 || value > 100 {
		return 0, fmt.Errorf("%w %q (expected 0%%-100%%)", errInvalidLoad, raw)
	}

	return Percent(value), nil
}
