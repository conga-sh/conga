//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package termwidth

import (
	"os"
	"syscall"
	"unsafe"
)

// IsTerminal reports whether file is a terminal: the TIOCGWINSZ ioctl
// succeeds. A terminal that reports no size yet (0x0) still counts, and a
// character device that is not a terminal (/dev/null) does not.
func IsTerminal(file *os.File) bool {
	_, errno := windowSize(file)

	return errno == 0
}

// winsize mirrors the kernel's struct winsize for the TIOCGWINSZ ioctl. The
// kernel only writes the first two fields; the pixel fields keep the struct
// layout identical to the C definition.
type winsize struct {
	rows, columns, xPixel, yPixel uint16
}

// widthOf reports the terminal width of file, or 0 when file is not a terminal.
func widthOf(file *os.File) int {
	size, errno := windowSize(file)

	if errno != 0 || size.columns == 0 {
		return 0
	}

	return int(size.columns)
}

// windowSize runs the TIOCGWINSZ ioctl on file.
func windowSize(file *os.File) (winsize, syscall.Errno) {
	var size winsize

	//nolint:gosec // fixed TIOCGWINSZ ioctl on the caller's own file descriptor
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size)))

	return size, errno
}
