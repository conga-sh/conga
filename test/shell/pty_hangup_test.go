//go:build shelltest && unix

package shell_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/test/shell"
)

func TestPtyHangup(t *testing.T) {
	t.Parallel()

	for _, shellName := range []string{"bash", "zsh"} {
		t.Run(shellName, func(t *testing.T) {
			t.Parallel()

			workspace := shell.NewWorkspace(t)
			workspace.BuildBinary(t, "./testdata/huptest", "huptest")

			session := workspace.StartPtySession(t, "huptest", shellName)

			readyPath := filepath.Join(workspace.WorkDir, "ready")
			finallyPath := filepath.Join(workspace.WorkDir, "finally")
			exitCodePath := filepath.Join(workspace.WorkDir, "exitcode")

			session.Type(fmt.Sprintf("huptest wrap %s -- huptest run %s %s", exitCodePath, readyPath, finallyPath))
			session.PressEnter()

			awaitFileContent(t, readyPath, 10*time.Second)

			session.KillSession()

			assert.Equal(t, "done\n", awaitFileContent(t, finallyPath, 10*time.Second))
			assert.Equal(t, "129", awaitFileContent(t, exitCodePath, 10*time.Second))
		})
	}
}

func awaitFileContent(t *testing.T, path string, deadline time.Duration) string {
	t.Helper()

	deadlineTime := time.Now().Add(deadline)

	for {
		data, err := os.ReadFile(filepath.Clean(path))
		if err == nil && len(data) > 0 {
			return string(data)
		}

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read %s: %v", path, err)
		}

		if time.Now().After(deadlineTime) {
			t.Fatalf("timed out waiting for %s", path)
		}

		time.Sleep(25 * time.Millisecond)
	}
}
