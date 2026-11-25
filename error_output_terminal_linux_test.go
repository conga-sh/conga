//go:build linux

package conga_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
	"golang.org/x/sys/unix"

	"github.com/conga-sh/conga"
)

// openPseudoTerminal returns both ends of a new pseudo-terminal. The slave is a
// real terminal for the TIOCGWINSZ probe, still unsized (0x0).
func openPseudoTerminal(t *testing.T) (*os.File, *os.File) {
	t.Helper()

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal available: %v", err)
	}

	t.Cleanup(func() { master.Close() })

	assert.NoError(t, unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0))

	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	assert.NoError(t, err)

	slave, err := os.OpenFile("/dev/pts/"+strconv.Itoa(number), os.O_RDWR|syscall.O_NOCTTY, 0)
	assert.NoError(t, err)
	t.Cleanup(func() { slave.Close() })

	return master, slave
}

// exitOnATerminal runs App.Exit with stderr on a pseudo-terminal, under the
// given color environment, and returns what the terminal received.
func exitOnATerminal(t *testing.T, environment map[string]string) string {
	t.Helper()

	t.Setenv("CONGA_COLOR", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("TERM", "xterm")

	for name, value := range environment {
		t.Setenv(name, value)
	}

	master, slave := openPseudoTerminal(t)

	conga.New[outputCLI]().
		Name("app").
		Stderr(slave).
		ExitFunc(func(int) {}).
		Exit(&devError{message: "failed", hint: "act"})

	assert.NoError(t, slave.Close())
	assert.NoError(t, master.SetReadDeadline(time.Now().Add(5*time.Second)))

	var received bytes.Buffer

	buffer := make([]byte, 256)

	for {
		count, err := master.Read(buffer)
		if count > 0 {
			received.Write(buffer[:count])
		}

		if errors.Is(err, io.EOF) || errors.Is(err, syscall.EIO) {
			break
		}

		assert.NoError(t, err)
	}

	return received.String()
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOutput_ColorOnATerminal(t *testing.T) {
	const (
		colored = "app: \x1b[1;31merror:\x1b[0m failed\r\n  \x1b[1;36mhint:\x1b[0m act\r\n"
		plain   = "app: error: failed\r\n  hint: act\r\n"
	)

	cases := []struct {
		name        string
		environment map[string]string
		expected    string
	}{
		{name: "no preference colors a terminal", environment: map[string]string{}, expected: colored},
		{name: "CONGA_COLOR 1 colors a terminal", environment: map[string]string{"CONGA_COLOR": "1"}, expected: colored},
		{name: "CONGA_COLOR 0 turns it off", environment: map[string]string{"CONGA_COLOR": "0"}, expected: plain},
		{name: "NO_COLOR turns it off", environment: map[string]string{"NO_COLOR": "1"}, expected: plain},
		{name: "CLICOLOR 0 turns it off", environment: map[string]string{"CLICOLOR": "0"}, expected: plain},
		{name: "TERM dumb turns it off", environment: map[string]string{"TERM": "dumb"}, expected: plain},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, exitOnATerminal(t, testCase.environment))
		})
	}
}
