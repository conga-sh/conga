//go:build windows

package envx

import (
	"io"
	"os"
	"syscall"
)

// enableVirtualTerminalProcessing is the console mode bit that makes the
// console interpret escape codes.
const enableVirtualTerminalProcessing = 0x0004

// isTerminal reports whether writer is a console on which escape-code
// handling is on or could be switched on. It switches it on, OR-ed into the
// mode the console reported. The NUL device, which reports ModeCharDevice,
// fails GetConsoleMode and so does not count.
func isTerminal(writer io.Writer) bool {
	return enableConsole(writer)
}

// enableEscapeCodes switches escape-code handling on for a console writer,
// best effort, for CONGA_COLOR=1.
func enableEscapeCodes(writer io.Writer) {
	enableConsole(writer)
}

// enableConsole turns on escape-code handling for writer when it is a Windows
// console, and reports whether escape-code handling is enabled.
func enableConsole(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}

	handle := syscall.Handle(file.Fd())

	var mode uint32
	if err := syscall.GetConsoleMode(handle, &mode); err != nil {
		return false
	}

	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}

	// The lazy DLL is looked up here, not in a package-level variable, which
	// would be a dynamic initializer in this init-free package.
	setConsoleMode := syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")

	result, _, _ := setConsoleMode.Call(uintptr(handle), uintptr(mode|enableVirtualTerminalProcessing))

	return result != 0
}
