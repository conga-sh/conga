package predict_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/predict"
)

func TestExecutables_PathScanning(t *testing.T) {
	t.Parallel()

	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	bin1 := filepath.Join(tmpDir1, "mytool")
	bin2 := filepath.Join(tmpDir2, "othertool")
	nonExec := filepath.Join(tmpDir1, "readme.txt")

	if runtime.GOOS == "windows" {
		bin1 += ".exe"
		bin2 += ".bat"
	}

	writeExecutableFile(t, bin1, []byte("#!/bin/sh\necho hi"))
	writeExecutableFile(t, bin2, []byte("#!/bin/sh\necho other"))
	writeFile(t, nonExec, []byte("docs"))

	pathList := tmpDir1 + string(filepath.ListSeparator) + tmpDir2
	pred := predict.Executables()

	result := pred.Invoke(predict.Context{
		Env: map[string]string{
			"PATH": pathList,
		},
	})

	values := itemValues(result.Items)
	expected1 := filepath.Base(bin1)
	expected2 := filepath.Base(bin2)

	assert.True(t, slices.Contains(values, expected1), "expected "+expected1)
	assert.True(t, slices.Contains(values, expected2), "expected "+expected2)
	assert.False(t, slices.Contains(values, "readme.txt"), "non-executables must not be suggested")
}

func TestExecutables_Deduplication(t *testing.T) {
	t.Parallel()

	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	name := "sharedtool"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	bin1 := filepath.Join(tmpDir1, name)
	bin2 := filepath.Join(tmpDir2, name)

	writeExecutableFile(t, bin1, []byte("#!/bin/sh\n# version 1"))
	writeExecutableFile(t, bin2, []byte("#!/bin/sh\n# version 2"))

	pathList := tmpDir1 + string(filepath.ListSeparator) + tmpDir2
	pred := predict.Executables()

	result := pred.Invoke(predict.Context{
		Env: map[string]string{
			"PATH": pathList,
		},
	})

	count := 0

	for _, item := range result.Items {
		if item.Value == name {
			count++
		}
	}

	assert.Equal(t, 1, count, "duplicate names across PATH directories must be deduplicated")
}

func TestExecutables_RelativePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	bin := filepath.Join(tmpDir, "localrun")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	writeExecutableFile(t, bin, []byte("#!/bin/sh\necho local"))
	writeFile(t, filepath.Join(tmpDir, "notes.md"), []byte("notes"))

	pred := predict.Executables()

	result := pred.Invoke(predict.Context{
		Dir:        tmpDir,
		ToComplete: "./",
	})

	values := itemValues(result.Items)
	expected := "./" + filepath.Base(bin)

	assert.True(t, slices.Contains(values, expected), "expected relative binary "+expected)
	assert.False(t, slices.Contains(values, "./notes.md"), "non-executable must be omitted")
}

func TestExecutables_DirectoryDecoration(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755))

	pred := predict.Executables()
	result := pred.Invoke(predict.Context{
		Dir:        tmpDir,
		ToComplete: "./",
	})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "./node_modules/"), "directories must keep the trailing slash")

	for _, item := range result.Items {
		if item.Value == "./node_modules/" {
			assert.Equal(t, "directories", item.Tag)
			assert.Equal(t, "bold blue", item.Style)
		}
	}
}

func writeExecutableFile(t *testing.T, path string, data []byte) {
	t.Helper()

	assert.NoError(t, os.WriteFile(path, data, 0755))
}
