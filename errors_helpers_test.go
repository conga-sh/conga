package conga_test

import (
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

// assertUsageError fails the test unless err carries a *conga.UsageError, and
// returns it.
func assertUsageError(t *testing.T, err error) *conga.UsageError {
	t.Helper()

	assert.Error(t, err)

	usage, ok := errors.AsType[*conga.UsageError](err)
	assert.True(t, ok, "expected a *conga.UsageError, got %T: %v", err, err)

	return usage
}

// assertNotUsageError fails the test unless err is non-nil and carries no
// *conga.UsageError.
func assertNotUsageError(t *testing.T, err error) {
	t.Helper()

	assert.Error(t, err)

	_, ok := errors.AsType[*conga.UsageError](err)
	assert.False(t, ok, "expected no *conga.UsageError, got %T: %v", err, err)
}
