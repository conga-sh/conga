package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// errInvalidLoad is the static sentinel for unparseable load percentages.
var errInvalidLoad = errors.New("invalid load percentage")

// Percent is a load percentage parsed from values like "75%" or "75". The type
// is foreign to CONGA and deliberately does not implement TextUnmarshaler, so
// it is wired via App.RegisterType in main.go instead.
type Percent int

// String returns the percentage in its canonical form, e.g. "75%".
func (p Percent) String() string {
	return strconv.Itoa(int(p)) + "%"
}

// parsePercent parses a "0%".."100%" flag value, with or without the trailing
// percent sign, for App.RegisterType.
func parsePercent(raw string) (Percent, error) {
	value, err := strconv.ParseUint(strings.TrimSuffix(raw, "%"), 10, 8)
	if err != nil || value > 100 {
		return 0, fmt.Errorf("%w %q (expected 0%%-100%%)", errInvalidLoad, raw)
	}

	return Percent(value), nil
}
