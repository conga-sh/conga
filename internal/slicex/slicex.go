// Package slicex provides generic functional utilities for slices.
package slicex

import "strings"

// Map transforms each element in items using the given transform function.
func Map[T any, R any](items []T, fn func(T) R) []R {
	result := make([]R, len(items))
	for i, item := range items {
		result[i] = fn(item)
	}

	return result
}

// Filter returns a new slice containing only the elements of items for which pred returns true.
func Filter[T any](items []T, pred func(T) bool) []T {
	result := make([]T, 0, len(items))
	for _, item := range items {
		if pred(item) {
			result = append(result, item)
		}
	}

	return result
}

// SplitAndTrim splits input by sep, trims leading and trailing whitespace from each substring,
// and returns a slice of non-empty substrings. Returns nil if input is empty.
func SplitAndTrim(input, sep string) []string {
	if input == "" {
		return nil
	}

	var result []string

	for item := range strings.SplitSeq(input, sep) {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
