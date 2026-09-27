//go:build windows

package completion

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type processInfo struct {
	ppid    uint32
	exeFile string
}

// parentProcess identifies the caller's shell on Windows by walking the process
// tree ancestry using Toolhelp32 snapshots and matching executable names against
// supported shells via shellFromExecPath.
func parentProcess(pid int) (int, string, error) {
	if pid <= 0 {
		return 0, "", os.ErrNotExist
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, "", err
	}
	defer func() {
		_ = windows.CloseHandle(snapshot)
	}()

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		return 0, "", err
	}

	procs := make(map[uint32]processInfo)
	for {
		procs[entry.ProcessID] = processInfo{
			ppid:    entry.ParentProcessID,
			exeFile: windows.UTF16ToString(entry.ExeFile[:]),
		}

		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	curr := uint32(pid)
	visited := make(map[uint32]bool)

	for curr > 0 && !visited[curr] {
		visited[curr] = true

		info, ok := procs[curr]
		if !ok {
			break
		}

		if shell := shellFromExecPath(info.exeFile); shell != "" {
			return 0, info.exeFile, nil
		}

		if info.ppid == curr || info.ppid <= 0 {
			break
		}

		curr = info.ppid
	}

	return 0, "", os.ErrNotExist
}
