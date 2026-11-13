package predict

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/style"
)

// Executables configures completion of executable binaries found in the system $PATH
// or local relative paths when starting with '.' or '/'.
func Executables() Predictor {
	return Callback(actionExecutables)
}

func actionExecutables(ctx Context) Predictor {
	return Predictor{
		items:     scanExecutables(ctx),
		directive: DirectiveNoFileCompletion,
	}
}

// isExecutable reports whether a directory entry counts as executable without
// resolving symlinks: on Windows by a PATHEXT extension, elsewhere by an
// execute bit in the entry's own lstat mode. A symlink therefore counts by its
// own bits (0777 on Linux), which keeps links in PATH directories listed;
// isExecutableFile judges a resolved target instead.
func isExecutable(entry os.DirEntry, pathExtensions []string) bool {
	if entry.IsDir() {
		return false
	}

	if runtime.GOOS == "windows" {
		return hasPathExtension(entry.Name(), pathExtensions)
	}

	info, err := entry.Info()
	if err != nil {
		return false
	}

	return info.Mode()&0111 != 0
}

// isExecutableFile reports whether a resolved (non-symlink) file counts as
// executable: a regular file with a PATHEXT extension on Windows, or with an
// execute bit elsewhere.
func isExecutableFile(name string, mode os.FileMode, pathExtensions []string) bool {
	if !mode.IsRegular() {
		return false
	}

	if runtime.GOOS == "windows" {
		return hasPathExtension(name, pathExtensions)
	}

	return mode&0111 != 0
}

// hasPathExtension reports whether name ends with one of the lowercased
// PATHEXT extensions.
func hasPathExtension(name string, pathExtensions []string) bool {
	lowerName := strings.ToLower(name)

	return slices.ContainsFunc(pathExtensions, func(extension string) bool {
		return strings.HasSuffix(lowerName, extension)
	})
}

func scanFilesystemExecutables(ctx Context) []Item {
	scan := newFSScan(ctx, fsModeExecutables, nil, nil)

	entries, err := os.ReadDir(scan.searchDir)
	if err != nil {
		return nil
	}

	return scan.candidateItems(entries)
}

func scanPathDir(dir, toComplete string, seen map[string]bool, pathExtensions []string) []Item {
	if dir == "" {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var items []Item

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(toComplete, ".") {
			continue
		}

		if seen[name] {
			continue
		}

		if !isExecutable(entry, pathExtensions) {
			continue
		}

		seen[name] = true
		items = append(items, Item{
			Value:   name,
			Display: name,
			Style:   style.ForExecutable(),
			Tag:     "executables",
		})
	}

	return items
}

func scanExecutables(ctx Context) []Item {
	toComplete := ctx.ToComplete

	if strings.HasPrefix(toComplete, "./") || strings.HasPrefix(toComplete, "../") || filepath.IsAbs(toComplete) || (runtime.GOOS == "windows" && strings.ContainsAny(toComplete, "/\\")) {
		return scanFilesystemExecutables(ctx)
	}

	pathEnv := ctx.Getenv("PATH")

	if pathEnv == "" {
		return nil
	}

	seen := make(map[string]bool)

	var items []Item

	pathExtensions := windowsPathExts(ctx)

	for _, dir := range filepath.SplitList(pathEnv) {
		items = append(items, scanPathDir(dir, toComplete, seen, pathExtensions)...)
	}

	return items
}

func windowsPathExts(ctx Context) []string {
	if runtime.GOOS != "windows" {
		return nil
	}

	return parsePathExtensions(ctx.Getenv("PATHEXT"))
}

func parsePathExtensions(extsEnv string) []string {
	if extsEnv == "" {
		return []string{".com", ".exe", ".bat", ".cmd"}
	}

	var extensions []string

	for extension := range strings.SplitSeq(extsEnv, ";") {
		if trimmed := strings.ToLower(strings.TrimSpace(extension)); trimmed != "" {
			extensions = append(extensions, trimmed)
		}
	}

	return extensions
}
