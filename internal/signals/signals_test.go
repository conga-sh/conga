package signals_test

import (
	"syscall"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/signals"
)

func TestKnown_TableNonEmpty(t *testing.T) {
	t.Parallel()

	known := signals.Known()

	assert.True(t, len(known) > 0, "expected a non-empty platform signal table")

	for _, entry := range known {
		assert.True(t, entry.Name != "", "expected every signal entry to have a name")
		assert.True(t, entry.Signal > 0, "expected every signal entry to have a positive number")
	}
}

func TestLookup_KnownSignals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected syscall.Signal
	}{
		{name: "full name uppercase", raw: "SIGTERM", expected: syscall.SIGTERM},
		{name: "full name lowercase", raw: "sighup", expected: syscall.SIGHUP},
		{name: "short name uppercase", raw: "HUP", expected: syscall.SIGHUP},
		{name: "short name lowercase", raw: "term", expected: syscall.SIGTERM},
		{name: "numeric POSIX signal", raw: "15", expected: syscall.SIGTERM},
		{name: "numeric out of table", raw: "999", expected: syscall.Signal(999)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sig, ok := signals.Lookup(test.raw)

			assert.True(t, ok)
			assert.Equal(t, test.expected, sig)
		})
	}
}

func TestLookup_UnknownSignals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "whitespace only", raw: "   "},
		{name: "zero", raw: "0"},
		{name: "negative number", raw: "-1"},
		{name: "unknown word", raw: "NOTASIGNAL"},
		{name: "unknown SIG prefix", raw: "SIG_NOPE"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sig, ok := signals.Lookup(test.raw)

			assert.False(t, ok)
			assert.Equal(t, syscall.Signal(0), sig)
		})
	}
}
