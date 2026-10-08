package predict

import (
	"bytes"
	"cmp"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/conga-sh/conga/internal/pathx"
	"github.com/conga-sh/conga/internal/scanx"
	"github.com/conga-sh/conga/internal/style"
)

const (
	fsModeFiles fsScanMode = iota
	fsModeDirs
	fsModePaths
	fsModeExecutables
)

// Dirs configures in-process directory completion.
func Dirs() Predictor {
	return Callback(actionDirs)
}

// DirsFiltered configures in-process directory completion with a custom
// filter predicate. Only directories matching the predicate are offered;
// symlinks that resolve to directories are treated as directories. This
// filter is a predicate over directory entries, not the query match of
// Predictor.FilterText.
func DirsFiltered(filter func(path string, entry os.DirEntry) bool) Predictor {
	return Callback(func(c Context) Predictor {
		return actionDirsFiltered(c, filter)
	})
}

// DirsGlob configures in-process directory completion filtered by glob
// patterns. A directory is offered when its base name matches at least one
// pattern (filepath.Match syntax; a wildcard-free pattern such as "build"
// matches that literal name).
func DirsGlob(patterns ...string) Predictor {
	return Callback(func(c Context) Predictor {
		return actionDirsGlob(c, patterns)
	})
}

// FileContents reads a file relative to the completion context directory and extracts completion items using the provided parser.
func FileContents(filePath string, parser func(content []byte) []Item) Predictor {
	return Callback(func(c Context) Predictor {
		return actionFileContents(c, filePath, parser)
	})
}

// FileLines configures completion of non-empty, non-comment lines from a file.
func FileLines(filePath string) Predictor {
	return FileContents(filePath, parseLinesFile)
}

// Files configures in-process file completion with optional file extension filters.
func Files(extensions ...string) Predictor {
	return Callback(func(c Context) Predictor {
		return actionFiles(c, extensions...)
	})
}

// FilesFiltered configures file completion with a custom filter predicate.
// Directories are retained for traversal, while files are filtered using the provided predicate.
// This filter is a predicate over directory entries, not the query match of
// Predictor.FilterText.
func FilesFiltered(filter func(path string, entry os.DirEntry) bool) Predictor {
	return Callback(func(c Context) Predictor {
		return actionFilesFiltered(c, filter)
	})
}

// FilesGlob configures in-process file completion filtered by glob patterns.
// Directories are retained for traversal, while files must match at least one
// pattern against their base name (filepath.Match syntax; a wildcard-free
// pattern such as "Makefile" matches that literal name).
func FilesGlob(patterns ...string) Predictor {
	return Callback(func(c Context) Predictor {
		return actionFilesGlob(c, patterns)
	})
}

// PathsFiltered configures completion where both files and directories are evaluated against the filter predicate.
// This filter is a predicate over directory entries, not the query match of
// Predictor.FilterText.
func PathsFiltered(filter func(path string, entry os.DirEntry) bool) Predictor {
	return Callback(func(c Context) Predictor {
		return actionPathsFiltered(c, filter)
	})
}

// Symlinks configures completion of symbolic links.
func Symlinks() Predictor {
	return Callback(actionSymlinks)
}

type fsScanMode int

func actionDirs(ctx Context) Predictor {
	items := scanFilesystem(ctx, fsModeDirs, nil, nil)

	return Predictor{
		items:     items,
		directive: DirectiveNoFileCompletion | DirectiveNoSpace,
	}
}

func actionDirsFiltered(ctx Context, filter func(path string, entry os.DirEntry) bool) Predictor {
	items := scanFilesystem(ctx, fsModeDirs, nil, filter)

	return Predictor{
		items:     items,
		directive: DirectiveNoFileCompletion | DirectiveNoSpace,
	}
}

func actionDirsGlob(ctx Context, patterns []string) Predictor {
	filter := func(_ string, entry os.DirEntry) bool {
		return matchesAnyGlob(entry.Name(), patterns)
	}
	items := scanFilesystem(ctx, fsModeDirs, nil, filter)

	return Predictor{
		items:     items,
		directive: DirectiveNoFileCompletion | DirectiveNoSpace,
	}
}

func actionFileContents(ctx Context, filePath string, parser func(content []byte) []Item) Predictor {
	if filePath == "" || parser == nil {
		return None()
	}

	resolvedPath := resolveFilePath(ctx, filePath)
	//nolint:gosec // Completion file path is controlled by predictor definition.
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return None()
	}

	items := parser(content)

	return Items(items...)
}

func actionFiles(ctx Context, extensions ...string) Predictor {
	items := scanFilesystem(ctx, fsModeFiles, extensions, nil)

	return Predictor{
		items:     items,
		directive: DirectiveNoFileCompletion,
	}
}

func actionFilesFiltered(ctx Context, filter func(path string, entry os.DirEntry) bool) Predictor {
	items := scanFilesystem(ctx, fsModeFiles, nil, filter)

	return Predictor{
		items:     items,
		directive: DirectiveNoFileCompletion,
	}
}

func actionFilesGlob(ctx Context, patterns []string) Predictor {
	filter := func(_ string, entry os.DirEntry) bool {
		return matchesAnyGlob(entry.Name(), patterns)
	}
	items := scanFilesystem(ctx, fsModeFiles, nil, filter)

	return Predictor{
		items:     items,
		directive: DirectiveNoFileCompletion,
	}
}

func actionPathsFiltered(ctx Context, filter func(path string, entry os.DirEntry) bool) Predictor {
	items := scanFilesystem(ctx, fsModePaths, nil, filter)

	return Predictor{
		items:     items,
		directive: DirectiveNoFileCompletion,
	}
}

func actionSymlinks(ctx Context) Predictor {
	filter := func(_ string, entry os.DirEntry) bool {
		return entry.Type()&os.ModeSymlink != 0
	}
	items := scanFilesystem(ctx, fsModePaths, nil, filter)

	return Predictor{
		items:     items,
		directive: DirectiveNoFileCompletion,
	}
}

func createCandidateItem(entry os.DirEntry, searchDir, dirPrefix, filterTerm string, mode fsScanMode, extensions []string, filter func(path string, entry os.DirEntry) bool) (Item, bool) {
	name := entry.Name()
	if strings.HasPrefix(name, ".") && !strings.HasPrefix(filterTerm, ".") {
		return Item{}, false
	}

	isDir := isEntryDir(entry, searchDir, name)
	fullPath := filepath.Join(searchDir, name)

	if !matchesScanMode(entry, fullPath, isDir, mode, extensions, filter) {
		return Item{}, false
	}

	candidateValue := dirPrefix + name
	displayValue := name

	if isDir {
		candidateValue += "/"
		displayValue += "/"
	}

	tag := "files"
	if isDir {
		tag = "directories"
	}

	itemStyle := formatItemStyle(name, isDir)

	if mode == fsModeExecutables && !isDir {
		tag = "executables"
		itemStyle = cmp.Or(style.ForPathExt(name), style.Green)
	}

	var description string

	// Executables are reported by executable/directory kind only; symlink
	// decoration belongs to the generic filesystem modes.
	if mode != fsModeExecutables && entry.Type()&os.ModeSymlink != 0 {
		tag = "symlinks"
		itemStyle = style.Cyan

		if target, err := os.Readlink(fullPath); err == nil {
			description = "-> " + target
		}
	}

	return Item{
		Value:       candidateValue,
		Display:     displayValue,
		Description: description,
		Style:       itemStyle,
		Tag:         tag,
	}, true
}

func formatItemStyle(name string, isDir bool) string {
	if isDir {
		return style.Of(style.Bold, style.Blue)
	}

	return style.ForPathExt(name)
}

func isEntryDir(entry os.DirEntry, searchDir, name string) bool {
	if entry.IsDir() {
		return true
	}

	if entry.Type()&os.ModeSymlink != 0 {
		fullPath := filepath.Join(searchDir, name)
		if fileInfo, err := os.Stat(fullPath); err == nil {
			return fileInfo.IsDir()
		}
	}

	return false
}

func matchesAnyGlob(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if match, err := filepath.Match(pattern, name); err == nil && match {
			return true
		}
	}

	return false
}

func matchesAnyExtension(filename string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}

	lowerName := strings.ToLower(filename)

	for _, extension := range extensions {
		extension = strings.TrimSpace(extension)
		if extension == "" || extension == "*" || extension == "*.*" {
			return true
		}

		extension = strings.TrimPrefix(extension, "*")

		if strings.HasPrefix(extension, ".") {
			if strings.HasSuffix(lowerName, strings.ToLower(extension)) {
				return true
			}
		} else {
			if strings.HasSuffix(lowerName, "."+strings.ToLower(extension)) {
				return true
			}
		}
	}

	return false
}

func matchesScanMode(entry os.DirEntry, fullPath string, isDir bool, mode fsScanMode, extensions []string, filter func(path string, entry os.DirEntry) bool) bool {
	switch mode {
	case fsModeDirs:
		return isDir && passesFilter(fullPath, entry, filter)

	case fsModeFiles:
		return isDir || (matchesExtension(entry.Name(), extensions) && passesFilter(fullPath, entry, filter))

	case fsModePaths:
		return passesFilter(fullPath, entry, filter)

	case fsModeExecutables:
		return isDir || isExecutable(entry, extensions)

	default:
		return true
	}
}

func matchesExtension(name string, extensions []string) bool {
	return len(extensions) == 0 || matchesAnyExtension(name, extensions)
}

func passesFilter(fullPath string, entry os.DirEntry, filter func(path string, entry os.DirEntry) bool) bool {
	return filter == nil || filter(fullPath, entry)
}

func parseLinesFile(content []byte) []Item {
	var items []Item

	if err := scanx.ScanLines(bytes.NewReader(content), func(line string) bool {
		items = append(items, Item{
			Value:   line,
			Display: line,
		})

		return true
	}); err != nil {
		return items
	}

	return items
}

func resolveFilePath(ctx Context, rawPath string) string {
	if strings.HasPrefix(rawPath, "~/") || rawPath == "~" {
		return pathx.ExpandTilde(rawPath, ctx.Getenv)
	}

	if filepath.IsAbs(rawPath) {
		return rawPath
	}

	baseDir := cmp.Or(ctx.Dir, ".")

	return filepath.Join(baseDir, filepath.FromSlash(rawPath))
}

func resolveSearchDir(ctx Context, dirPrefix string) string {
	baseDir := cmp.Or(ctx.Dir, ".")

	if dirPrefix == "" {
		return baseDir
	}

	if strings.HasPrefix(dirPrefix, "~/") || dirPrefix == "~" {
		return pathx.ExpandTilde(dirPrefix, ctx.Getenv)
	}

	if filepath.IsAbs(dirPrefix) {
		return filepath.Clean(filepath.FromSlash(dirPrefix))
	}

	return filepath.Join(baseDir, filepath.FromSlash(dirPrefix))
}

func scanFilesystem(ctx Context, mode fsScanMode, extensions []string, filter func(path string, entry os.DirEntry) bool) []Item {
	dirPrefix, filterTerm := splitPathTarget(ctx.ToComplete)

	if ctx.ToComplete == "~" {
		return []Item{
			{
				Value:   "~/",
				Display: "~/",
				Style:   style.Of(style.Bold, style.Blue),
				Tag:     "directories",
			},
		}
	}

	searchDir := resolveSearchDir(ctx, dirPrefix)

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	items := make([]Item, 0, len(entries))

	for _, entry := range entries {
		if item, ok := createCandidateItem(entry, searchDir, dirPrefix, filterTerm, mode, extensions, filter); ok {
			items = append(items, item)
		}
	}

	return items
}

func splitPathTarget(toComplete string) (string, string) {
	var lastSlash int
	if runtime.GOOS == "windows" {
		lastSlash = strings.LastIndexAny(toComplete, `/\`)
	} else {
		lastSlash = strings.LastIndexByte(toComplete, '/')
	}

	if lastSlash == -1 {
		return "", toComplete
	}

	return toComplete[:lastSlash+1], toComplete[lastSlash+1:]
}
