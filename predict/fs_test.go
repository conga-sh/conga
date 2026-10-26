package predict_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
)

var (
	// Text-file payloads consumed by the file-line and file-content predictors.
	//go:embed testdata/fixtures/pairs.txt
	fixturePairs []byte

	//go:embed testdata/fixtures/shells
	fixtureShells []byte
)

func TestFiles_Extensions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "app.yaml"), []byte("name: app"))
	writeFile(t, filepath.Join(tmpDir, "config.yml"), []byte("port: 8080"))
	writeFile(t, filepath.Join(tmpDir, "readme.md"), []byte("# Readme"))
	writeFile(t, filepath.Join(tmpDir, "data.json"), []byte("{}"))
	writeFile(t, filepath.Join(tmpDir, "archive.tar.gz"), []byte("data"))
	writeFile(t, filepath.Join(tmpDir, "main.go"), []byte("package main"))

	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "build"), 0755))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "dist"), 0755))

	result := predict.Files("*.yaml", "*.yml").Invoke(predict.Context{Dir: tmpDir})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "app.yaml"), "expected app.yaml")
	assert.True(t, slices.Contains(values, "config.yml"), "expected config.yml")
	assert.True(t, slices.Contains(values, "build/"), "expected build/")
	assert.True(t, slices.Contains(values, "dist/"), "expected dist/")

	assert.False(t, slices.Contains(values, "readme.md"), "should not contain readme.md")
	assert.False(t, slices.Contains(values, "data.json"), "should not contain data.json")
	assert.False(t, slices.Contains(values, "archive.tar.gz"), "should not contain archive.tar.gz")
	assert.False(t, slices.Contains(values, "main.go"), "should not contain main.go")
	assert.False(t, slices.Contains(values, "*.yaml"), "must not contain literal glob")

	for _, item := range result.Items {
		switch item.Value {
		case "build/", "dist/":
			assert.Equal(t, "directories", item.Tag)
			assert.Equal(t, "bold blue", item.Style)
		case "app.yaml", "config.yml":
			assert.Equal(t, "files", item.Tag)
			assert.Equal(t, "yellow", item.Style)
		}
	}
}

func TestFiles_NoExtensions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "app.yaml"), []byte("name: app"))
	writeFile(t, filepath.Join(tmpDir, "readme.md"), []byte("# Readme"))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "build"), 0755))

	result := predict.Files().Invoke(predict.Context{Dir: tmpDir})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "app.yaml"))
	assert.True(t, slices.Contains(values, "readme.md"))
	assert.True(t, slices.Contains(values, "build/"))
}

func TestFiles_MultiDotExtension(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "bundle.tar.gz"), []byte("data"))
	writeFile(t, filepath.Join(tmpDir, "app.yaml"), []byte("name: app"))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "src"), 0755))

	result := predict.Files("*.tar.gz").Invoke(predict.Context{Dir: tmpDir})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "bundle.tar.gz"))
	assert.True(t, slices.Contains(values, "src/"))
	assert.False(t, slices.Contains(values, "app.yaml"))
}

func TestFiles_CaseInsensitivity(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "MANIFEST.YAML"), []byte("name: app"))

	result := predict.Files("*.yaml").Invoke(predict.Context{Dir: tmpDir})

	assert.True(t, slices.Contains(itemValues(result.Items), "MANIFEST.YAML"))
}

func TestDirs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "app.go"), []byte("package main"))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "build"), 0755))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "dist"), 0755))

	result := predict.Dirs().Invoke(predict.Context{Dir: tmpDir})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "build/"))
	assert.True(t, slices.Contains(values, "dist/"))
	assert.False(t, slices.Contains(values, "app.go"))
	assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
	assert.True(t, result.Directive.Has(predict.DirectiveNoFileCompletion))
}

func TestDirsFiltered(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "2023-01"), 0755))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "2024-06"), 0755))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "archive"), 0755))
	writeFile(t, filepath.Join(tmpDir, "notes.txt"), []byte("notes"))
	writeFile(t, filepath.Join(tmpDir, "app.go"), []byte("package main"))

	pred := predict.DirsFiltered(func(path string, entry os.DirEntry) bool {
		return strings.HasPrefix(entry.Name(), "202")
	})
	result := pred.Invoke(predict.Context{Dir: tmpDir})

	assert.Equal(t, []string{"2023-01/", "2024-06/"}, itemValues(result.Items))
	assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
	assert.True(t, result.Directive.Has(predict.DirectiveNoFileCompletion))
}

func TestDirsFiltered_SymlinkedDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "real"), 0755))

	symlinkPath := filepath.Join(tmpDir, "link")
	if err := os.Symlink(filepath.Join(tmpDir, "real"), symlinkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	pred := predict.DirsFiltered(func(path string, entry os.DirEntry) bool {
		return entry.Name() == "link"
	})
	result := pred.Invoke(predict.Context{Dir: tmpDir})

	assert.Equal(t, []string{"link/"}, itemValues(result.Items))
	assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
	assert.True(t, result.Directive.Has(predict.DirectiveNoFileCompletion))
}

func TestDirsGlob(t *testing.T) {
	t.Parallel()

	t.Run("literal pattern matches exact directory name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "build"), 0755))
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "dist"), 0755))
		writeFile(t, filepath.Join(tmpDir, "Makefile"), []byte("all:"))

		pred := predict.DirsGlob("build")
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		assert.Equal(t, []string{"build/"}, itemValues(result.Items))
		assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
		assert.True(t, result.Directive.Has(predict.DirectiveNoFileCompletion))
	})

	t.Run("multiple patterns union", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "env-prod"), 0755))
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "env-staging"), 0755))
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "archive"), 0755))

		pred := predict.DirsGlob("env-*", "archive")
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		assert.Equal(t, []string{"archive/", "env-prod/", "env-staging/"}, itemValues(result.Items))
	})

	t.Run("malformed pattern matches nothing", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "build"), 0755))

		pred := predict.DirsGlob("[")
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		assert.Zero(t, result.Items)
	})
}

func TestFiles_NestedTraversal(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "nested", "sub")
	assert.NoError(t, os.MkdirAll(filepath.Join(subDir, "child"), 0755))
	writeFile(t, filepath.Join(subDir, "file.yaml"), []byte("key: val"))
	writeFile(t, filepath.Join(subDir, "data.json"), []byte("{}"))

	pred := predict.Files("*.yaml")

	t.Run("traverse into subfolder", func(t *testing.T) {
		t.Parallel()

		result := pred.Invoke(predict.Context{
			Dir:        tmpDir,
			ToComplete: "nested/sub/",
		})

		values := itemValues(result.Items)

		assert.True(t, slices.Contains(values, "nested/sub/file.yaml"))
		assert.True(t, slices.Contains(values, "nested/sub/child/"))
		assert.False(t, slices.Contains(values, "nested/sub/data.json"))
	})

	t.Run("filter by filename prefix in subfolder", func(t *testing.T) {
		t.Parallel()

		result := pred.Invoke(predict.Context{
			Dir:        tmpDir,
			ToComplete: "nested/sub/fi",
		})

		values := itemValues(result.Items)

		assert.True(t, slices.Contains(values, "nested/sub/file.yaml"))
		assert.False(t, slices.Contains(values, "nested/sub/child/"))
		assert.False(t, slices.Contains(values, "nested/sub/data.json"))
	})
}

func TestFiles_HiddenFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, ".env"), []byte("SECRET=1"))
	writeFile(t, filepath.Join(tmpDir, "normal.txt"), []byte("hello"))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, ".github"), 0755))

	pred := predict.Files()

	t.Run("omits hidden files by default", func(t *testing.T) {
		t.Parallel()

		result := pred.Invoke(predict.Context{Dir: tmpDir, ToComplete: ""})
		values := itemValues(result.Items)

		assert.True(t, slices.Contains(values, "normal.txt"))
		assert.False(t, slices.Contains(values, ".env"))
		assert.False(t, slices.Contains(values, ".github/"))
	})

	t.Run("includes hidden files when query starts with dot", func(t *testing.T) {
		t.Parallel()

		result := pred.Invoke(predict.Context{Dir: tmpDir, ToComplete: "."})
		values := itemValues(result.Items)

		assert.True(t, slices.Contains(values, ".env"))
		assert.True(t, slices.Contains(values, ".github/"))
	})
}

func TestFiles_AbsolutePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "abs-test.yaml"), []byte("data"))

	absPrefix := filepath.ToSlash(tmpDir) + "/"

	result := predict.Files("*.yaml").Invoke(predict.Context{
		ToComplete: absPrefix,
	})

	values := itemValues(result.Items)
	expectedValue := absPrefix + "abs-test.yaml"

	assert.True(t, slices.Contains(values, expectedValue), "expected "+expectedValue)
}

func TestFiles_TildePath(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	writeFile(t, filepath.Join(homeDir, "home.yaml"), []byte("home"))

	pred := predict.Files("*.yaml")

	t.Run("tilde slash prefix", func(t *testing.T) {
		t.Parallel()

		result := pred.Invoke(predict.Context{
			Env:        map[string]string{"HOME": homeDir},
			ToComplete: "~/",
		})

		values := itemValues(result.Items)
		assert.True(t, slices.Contains(values, "~/home.yaml"))
	})

	t.Run("bare tilde completion", func(t *testing.T) {
		t.Parallel()

		result := pred.Invoke(predict.Context{
			Env:        map[string]string{"HOME": homeDir},
			ToComplete: "~",
		})

		values := itemValues(result.Items)
		assert.True(t, slices.Contains(values, "~/"))
	})
}

func TestDirs_TildePath(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(homeDir, "homework"), 0755))

	pred := predict.Dirs()

	t.Run("tilde slash prefix lists home directories", func(t *testing.T) {
		t.Parallel()

		result := pred.Invoke(predict.Context{
			Env:        map[string]string{"HOME": homeDir},
			ToComplete: "~/",
		})

		values := itemValues(result.Items)
		assert.True(t, slices.Contains(values, "~/homework/"))
	})

	t.Run("bare tilde offers tilde slash", func(t *testing.T) {
		t.Parallel()

		result := pred.Invoke(predict.Context{
			Env:        map[string]string{"HOME": homeDir},
			ToComplete: "~",
		})

		values := itemValues(result.Items)
		assert.True(t, slices.Contains(values, "~/"))
	})
}

func TestFiles_NonExistentDir(t *testing.T) {
	t.Parallel()

	result := predict.Files("*.yaml").Invoke(predict.Context{
		Dir: filepath.Join(t.TempDir(), "nonexistent", "subfolder"),
	})

	assert.Zero(t, result.Items)
}

func TestFiles_Symlink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "real-target")
	assert.NoError(t, os.Mkdir(targetDir, 0755))
	writeFile(t, filepath.Join(targetDir, "data.yaml"), []byte("real"))

	symlinkPath := filepath.Join(tmpDir, "symlink-dir")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	result := predict.Files("*.yaml").Invoke(predict.Context{Dir: tmpDir})

	values := itemValues(result.Items)
	assert.True(t, slices.Contains(values, "symlink-dir/"), "expected directory symlink to have trailing slash")
}

func TestFilesFiltered(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "docker-compose.yml"), []byte("services: {}"))
	writeFile(t, filepath.Join(tmpDir, "docker-compose.prod.yaml"), []byte("services: {}"))
	writeFile(t, filepath.Join(tmpDir, "app.go"), []byte("package main"))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "deploy"), 0755))

	pred := predict.FilesFiltered(func(path string, entry os.DirEntry) bool {
		name := entry.Name()

		return strings.HasPrefix(name, "docker-compose") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml"))
	})
	result := pred.Invoke(predict.Context{Dir: tmpDir})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "docker-compose.yml"))
	assert.True(t, slices.Contains(values, "docker-compose.prod.yaml"))
	assert.True(t, slices.Contains(values, "deploy/"), "directories must be kept for traversal")
	assert.False(t, slices.Contains(values, "app.go"))
}

func TestFilesGlob(t *testing.T) {
	t.Parallel()

	t.Run("literal pattern matches exact file name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		writeFile(t, filepath.Join(tmpDir, "Makefile"), []byte("all:"))
		writeFile(t, filepath.Join(tmpDir, "makefile.txt"), []byte("noise"))

		pred := predict.FilesGlob("Makefile")
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		assert.Equal(t, []string{"Makefile"}, itemValues(result.Items))
	})

	t.Run("multiple patterns match any alternative", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		writeFile(t, filepath.Join(tmpDir, "Makefile"), []byte("all:"))
		writeFile(t, filepath.Join(tmpDir, "config.yaml"), []byte("key: value"))
		writeFile(t, filepath.Join(tmpDir, "config.yml"), []byte("key: value"))
		writeFile(t, filepath.Join(tmpDir, "app.go"), []byte("package main"))

		pred := predict.FilesGlob("Makefile", "*.yaml")
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		values := itemValues(result.Items)

		assert.True(t, slices.Contains(values, "Makefile"))
		assert.True(t, slices.Contains(values, "config.yaml"))
		assert.False(t, slices.Contains(values, "config.yml"))
		assert.False(t, slices.Contains(values, "app.go"))
	})

	t.Run("directories are retained for traversal", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		writeFile(t, filepath.Join(tmpDir, "app.go"), []byte("package main"))
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "deploy"), 0755))

		pred := predict.FilesGlob("*.yaml")
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		assert.Equal(t, []string{"deploy/"}, itemValues(result.Items))
	})

	t.Run("pattern filters nested traversal entries by base name", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "deploy"), 0755))
		writeFile(t, filepath.Join(tmpDir, "deploy", "ingress.yaml"), []byte("x"))
		writeFile(t, filepath.Join(tmpDir, "deploy", "notes.md"), []byte("x"))

		pred := predict.FilesGlob("*.yaml")
		result := pred.Invoke(predict.Context{Dir: tmpDir, ToComplete: "deploy/"})

		assert.Equal(t, []string{"deploy/ingress.yaml"}, itemValues(result.Items))
	})

	t.Run("character class pattern matches", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		writeFile(t, filepath.Join(tmpDir, "test1.go"), []byte("x"))
		writeFile(t, filepath.Join(tmpDir, "test2.go"), []byte("x"))
		writeFile(t, filepath.Join(tmpDir, "testX.go"), []byte("x"))

		pred := predict.FilesGlob("test[0-9].go")
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		values := itemValues(result.Items)

		assert.True(t, slices.Contains(values, "test1.go"))
		assert.True(t, slices.Contains(values, "test2.go"))
		assert.False(t, slices.Contains(values, "testX.go"))
	})

	t.Run("no matching files yields only directories", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		writeFile(t, filepath.Join(tmpDir, "app.go"), []byte("package main"))
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "assets"), 0755))

		pred := predict.FilesGlob("*.yaml")
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		assert.Equal(t, []string{"assets/"}, itemValues(result.Items))
	})

	t.Run("malformed pattern matches nothing", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		writeFile(t, filepath.Join(tmpDir, "app.go"), []byte("package main"))

		pred := predict.FilesGlob("[")
		result := pred.Invoke(predict.Context{Dir: tmpDir})

		assert.Zero(t, result.Items)
	})
}

func TestPathsFiltered(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "data.csv"), []byte("a,b"))
	writeFile(t, filepath.Join(tmpDir, "ignore.txt"), []byte("ignore"))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "valid_dir"), 0755))

	pred := predict.PathsFiltered(func(path string, entry os.DirEntry) bool {
		if entry.Name() == "node_modules" {
			return false
		}

		if entry.IsDir() {
			return true
		}

		return strings.HasSuffix(entry.Name(), ".csv")
	})
	result := pred.Invoke(predict.Context{Dir: tmpDir})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "data.csv"))
	assert.True(t, slices.Contains(values, "valid_dir/"))
	assert.False(t, slices.Contains(values, "node_modules/"))
	assert.False(t, slices.Contains(values, "ignore.txt"))
}

func TestSymlinks(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "actual.txt")
	writeFile(t, targetFile, []byte("target"))
	writeFile(t, filepath.Join(tmpDir, "regular.txt"), []byte("regular"))

	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	result := predict.Symlinks().Invoke(predict.Context{Dir: tmpDir})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "link.txt"))
	assert.False(t, slices.Contains(values, "regular.txt"))
	assert.False(t, slices.Contains(values, "actual.txt"))

	for _, item := range result.Items {
		if item.Value == "link.txt" {
			assert.Equal(t, "symlinks", item.Tag)
			assert.True(t, strings.HasPrefix(item.Description, "-> "))
		}
	}
}

func TestFileLines(t *testing.T) {
	t.Parallel()

	shellsFile := filepath.Join(t.TempDir(), "shells")
	writeFile(t, shellsFile, fixtureShells)

	result := predict.FileLines(shellsFile).Invoke(predict.Context{})

	values := itemValues(result.Items)

	assert.Equal(t, 3, len(values))
	assert.True(t, slices.Contains(values, "/bin/bash"))
	assert.True(t, slices.Contains(values, "/usr/bin/zsh"))
	assert.True(t, slices.Contains(values, "/bin/sh"))
	assert.False(t, slices.Contains(values, "# Valid shells"))
}

func TestFileContents(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "pairs.txt"), fixturePairs)

	pred := predict.FileContents("pairs.txt", func(content []byte) []predict.Item {
		var items []predict.Item

		for line := range strings.SplitSeq(string(content), "\n") {
			line = strings.TrimSpace(line)
			if key, value, found := strings.Cut(line, ":"); found {
				items = append(items, predict.Item{
					Value:       key,
					Description: value,
				})
			}
		}

		return items
	})

	result := pred.Invoke(predict.Context{Dir: tmpDir})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "alpha"))
	assert.True(t, slices.Contains(values, "beta"))
}

func itemValues(items []predict.Item) []string {
	return slicex.Map(items, func(item predict.Item) string { return item.Value })
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()

	assert.NoError(t, os.WriteFile(path, data, 0600))
}
