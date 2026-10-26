package predict

import (
	"cmp"
	"os"
	"path/filepath"
	"runtime"
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

func isExecutable(entry os.DirEntry, pathExtensions []string) bool {
	if entry.IsDir() {
		return false
	}

	name := entry.Name()

	if runtime.GOOS == "windows" {
		lowerName := strings.ToLower(name)

		for _, extension := range pathExtensions {
			if strings.HasSuffix(lowerName, extension) {
				return true
			}
		}

		return false
	}

	info, err := entry.Info()
	if err != nil {
		return false
	}

	return info.Mode()&0111 != 0
}

func scanFilesystemExecutables(ctx Context) []Item {
	dirPrefix, filterTerm := splitPathTarget(ctx.ToComplete)
	searchDir := resolveSearchDir(ctx, dirPrefix)

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	pathExtensions := windowsPathExts(ctx)

	var items []Item

	for _, entry := range entries {
		if item, ok := createCandidateItem(entry, searchDir, dirPrefix, filterTerm, fsModeExecutables, pathExtensions, nil); ok {
			items = append(items, item)
		}
	}

	return items
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
			Style:   cmp.Or(style.ForPathExt(name), style.Green),
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
