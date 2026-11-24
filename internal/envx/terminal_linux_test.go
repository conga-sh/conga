//go:build linux

package envx_test

import (
	"os"
	"strconv"
	"syscall"
	"testing"

	"github.com/alecthomas/assert/v2"
	"golang.org/x/sys/unix"
)

// openTerminal returns the slave end of a new pseudo-terminal, which the
// kernel has not sized yet (0x0). It is a real terminal for the TIOCGWINSZ
// probe.
func openTerminal(t *testing.T) *os.File {
	t.Helper()

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal available: %v", err)
	}

	t.Cleanup(func() { assert.NoError(t, master.Close()) })

	assert.NoError(t, unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0))

	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	assert.NoError(t, err)

	slave, err := os.OpenFile("/dev/pts/"+strconv.Itoa(number), os.O_RDWR|syscall.O_NOCTTY, 0)
	assert.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, slave.Close()) })

	return slave
}
