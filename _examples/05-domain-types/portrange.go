package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errInvalidPortRange = errors.New("invalid port range")

// PortRange is a custom domain type implementing encoding.TextUnmarshaler and conga.SelfValidator.
type PortRange struct {
	Min int
	Max int
}

func (p *PortRange) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))

	minStr, maxStr, ok := strings.Cut(raw, "-")
	if !ok || strings.Contains(maxStr, "-") {
		return fmt.Errorf("%w %q (expected min-max)", errInvalidPortRange, raw)
	}

	minValue, err := strconv.Atoi(minStr)
	if err != nil {
		return fmt.Errorf("%w in %q: %w", errInvalidPortRange, raw, err)
	}

	maxValue, err := strconv.Atoi(maxStr)
	if err != nil {
		return fmt.Errorf("%w in %q: %w", errInvalidPortRange, raw, err)
	}

	p.Min = minValue
	p.Max = maxValue

	return nil
}

func (p *PortRange) Validate(_ context.Context) error {
	if p.Min < 1 || p.Max > 65535 || p.Min > p.Max {
		return fmt.Errorf("%w %d-%d (must be 1..65535 and min <= max)", errInvalidPortRange, p.Min, p.Max)
	}

	return nil
}

func (p *PortRange) String() string {
	if p.Min == 0 && p.Max == 0 {
		return ""
	}

	return fmt.Sprintf("%d-%d", p.Min, p.Max)
}
