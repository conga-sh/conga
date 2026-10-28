//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package termwidth

import (
	"os"
	"syscall"
	"unsafe"
)

// winsize mirrors the kernel's struct winsize for the TIOCGWINSZ ioctl. The
// kernel only writes the first two fields; the pixel fields keep the struct
// layout identical to the C definition.
type winsize struct {
	rows, columns, xPixel, yPixel uint16
}

// widthOf reports the terminal width of file, or 0 when file is not a terminal.
func widthOf(file *os.File) int {
	var size winsize

	//nolint:gosec // fixed TIOCGWINSZ ioctl on the caller's own file descriptor
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size)))

	if errno != 0 || size.columns == 0 {
		return 0
	}

	return int(size.columns)
}
