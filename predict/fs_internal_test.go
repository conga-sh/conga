package predict

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/style"
)

func TestSplitPathTarget(t *testing.T) {
	t.Parallel()

	dir, file := splitPathTarget("foo/bar")
	assert.Equal(t, "foo/", dir)
	assert.Equal(t, "bar", file)

	if runtime.GOOS == "windows" {
		dirWin, fileWin := splitPathTarget(`foo\bar`)
		assert.Equal(t, `foo\`, dirWin)
		assert.Equal(t, "bar", fileWin)
	} else {
		dirUnix, fileUnix := splitPathTarget(`foo\bar`)
		assert.Equal(t, "", dirUnix, "backslash must not be treated as path separator on Unix")
		assert.Equal(t, `foo\bar`, fileUnix)
	}
}

func TestFSScan_ExecutableCheckSkippedForNamesTheTypedWordExcludes(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Windows judges executables by PATHEXT without an lstat")
	}

	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "run"), []byte("#!/bin/sh\n"), 0o755))
	scan := newFSScan(Context{Dir: dir, ToComplete: "x"}, fsModeFiles, nil, nil)
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)

	items := scan.candidateItems(entries)

	assert.Equal(t, style.ForPathExt("run"), items[0].Style)
	assert.Equal(t, executableCheckBudget, scan.executableChecks, "an excluded name must not spend the budget")
}

func TestFSScan_ExecutableCheckStopsWhenBudgetIsSpent(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Windows judges executables by PATHEXT without an lstat")
	}

	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "first"), []byte("#!/bin/sh\n"), 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "second"), []byte("#!/bin/sh\n"), 0o755))
	scan := newFSScan(Context{Dir: dir}, fsModeFiles, nil, nil)
	scan.executableChecks = 1
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)

	items := scan.candidateItems(entries)

	assert.Equal(t, style.ForExecutable(), items[0].Style)
	assert.Equal(t, style.ForPathExt("second"), items[1].Style)
}
