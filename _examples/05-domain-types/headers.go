package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var errInvalidHeader = errors.New("invalid header")

// HeaderMap is a custom map domain type implementing conga.SelfValidator.
// It ensures that HTTP header names are non-empty, conform to HTTP header conventions,
// and do not leak sensitive credentials via command line flags.
type HeaderMap map[string]string

func (h HeaderMap) Validate(_ context.Context) error {
	for key := range h {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%w: header name cannot be empty", errInvalidHeader)
		}

		if strings.ContainsAny(key, " \t:/@") {
			return fmt.Errorf("%w: header name %q contains invalid characters", errInvalidHeader, key)
		}

		if strings.EqualFold(key, "Authorization") {
			return fmt.Errorf("%w: header %q cannot be passed via command line", errInvalidHeader, key)
		}
	}

	return nil
}
