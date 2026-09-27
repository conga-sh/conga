//go:build linux

package completion

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// parentProcess reports a process's parent pid and executable name on Linux by
// reading /proc/<pid>/stat. The executable name (comm) is returned as the
// exec path; Linux never needs the full path since the bash upgrade warning is
// macOS-only.
func parentProcess(pid int) (int, string, error) {
	data, err := os.ReadFile(filepath.Clean(fmt.Sprintf("/proc/%d/stat", pid)))
	if err != nil {
		return 0, "", err
	}

	stat := string(data)
	idx1 := strings.Index(stat, "(")
	idx2 := strings.LastIndex(stat, ")")

	if idx1 == -1 || idx2 == -1 || idx2+2 >= len(stat) {
		return 0, "", os.ErrNotExist
	}

	comm := stat[idx1+1 : idx2]

	fields := strings.Fields(stat[idx2+2:])
	if len(fields) < 2 {
		return 0, "", os.ErrNotExist
	}

	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, "", err
	}

	return ppid, comm, nil
}
