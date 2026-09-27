//go:build darwin

package completion

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// parentProcess reports a process's parent pid and the full executable path
// (argv[0]) on macOS by querying the bundled BSD ps. The full command is used
// so shell hints can distinguish macOS's bundled bash (/bin/bash, version
// 3.2.57) from a Homebrew-installed bash by path alone.
func parentProcess(pid int) (int, string, error) {
	out, err := exec.Command("ps", "-ww", "-o", "ppid=", "-o", "command=", "-p", strconv.Itoa(pid)).Output() //nolint:gosec // fixed command, pid is an integer
	if err != nil {
		return 0, "", err
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return 0, "", os.ErrNotExist
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0, "", os.ErrNotExist
	}

	ppid, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", err
	}

	command := strings.TrimSpace(line[len(fields[0]):])
	if index := strings.IndexByte(command, ' '); index >= 0 {
		command = command[:index]
	}

	return ppid, command, nil
}
