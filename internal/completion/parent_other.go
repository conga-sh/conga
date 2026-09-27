//go:build !linux && !darwin && !windows

package completion

import "os"

// parentProcess reports no parent-process information on platforms without a
// supported process-table reader, so shell detection falls back to the static
// pointer on those platforms.
func parentProcess(pid int) (int, string, error) {
	return 0, "", os.ErrNotExist
}
