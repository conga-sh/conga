// Package assertx provides testing assertion extensions augmenting github.com/alecthomas/assert/v2.
package assertx

import (
	"fmt"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// PanicsWithValue asserts that action panics and that the recovered value equals expected.
func PanicsWithValue(tb testing.TB, expected any, action func(), msgAndArgs ...any) {
	tb.Helper()

	defer func() {
		tb.Helper()

		recovered := recover()
		if recovered == nil {
			tb.Fatal("Expected function to panic")

			return
		}

		assert.Equal(tb, expected, recovered, msgAndArgs...)
	}()

	action()
}

// Unreachable asserts that a code location was not reached, failing the test immediately.
func Unreachable(tb testing.TB, msgAndArgs ...any) {
	tb.Helper()

	if len(msgAndArgs) > 0 {
		tb.Fatalf("Unreachable code reached: %s", fmt.Sprint(msgAndArgs...))

		return
	}

	tb.Fatal("Unreachable code reached")
}
