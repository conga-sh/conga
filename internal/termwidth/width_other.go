//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package termwidth

import "os"

// widthOf reports no terminal width on platforms without a portable ioctl
// probe; Resolve then falls back to COLUMNS or Default. The Windows console
// would need kernel32 through golang.org/x/sys, which the core footprint rules
// keep out of the always-linked set.
func widthOf(_ *os.File) int {
	return 0
}
