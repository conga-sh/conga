package system

import (
	"runtime"
	"syscall"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestParseSignal_PlatformSymmetry(t *testing.T) {
	t.Parallel()

	sig, err := parseSignal("TERM")
	assert.NoError(t, err)
	assert.Equal(t, syscall.SIGTERM, sig)

	sig, err = parseSignal("SIGINT")
	assert.NoError(t, err)
	assert.Equal(t, syscall.SIGINT, sig)

	sig, err = parseSignal("2")
	assert.NoError(t, err)
	assert.Equal(t, syscall.SIGINT, sig)

	if runtime.GOOS != "windows" {
		sigUSR1, err := parseSignal("USR1")
		assert.NoError(t, err)

		sigFull, err := parseSignal("SIGUSR1")
		assert.NoError(t, err)
		assert.Equal(t, sigFull, sigUSR1)
		assert.True(t, sigUSR1 > 0)

		switch runtime.GOOS {
		case "linux":
			assert.Equal(t, syscall.Signal(10), sigUSR1)
		case "darwin", "freebsd", "openbsd":
			assert.Equal(t, syscall.Signal(30), sigUSR1)
		}
	}
}

func TestParseSignal_UnknownSignal(t *testing.T) {
	t.Parallel()

	sig, err := parseSignal("NOTASIGNAL")

	assert.Error(t, err)
	assert.Equal(t, `unknown or invalid signal "NOTASIGNAL"`, err.Error())
	assert.Equal(t, syscall.Signal(0), sig)
}
