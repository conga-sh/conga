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
// pattern (filepath.Match syntax; a wildcard-free pattern such as build
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
// pattern such as Makefile matches that literal name).
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
	return Predictor{
		items:     scanFilesystem(ctx, fsModeDirs, nil, nil),
		directive: DirectiveNoFileCompletion | DirectiveNoSpace,
	}
}

func actionDirsFiltered(ctx Context, filter func(path string, entry os.DirEntry) bool) Predictor {
	return Predictor{
		items:     scanFilesystem(ctx, fsModeDirs, nil, filter),
		directive: DirectiveNoFileCompletion | DirectiveNoSpace,
	}
}

func actionDirsGlob(ctx Context, patterns []string) Predictor {
	filter := func(_ string, entry os.DirEntry) bool {
		return matchesAnyGlob(entry.Name(), patterns)
	}

	return Predictor{
		items:     scanFilesystem(ctx, fsModeDirs, nil, filter),
		directive: DirectiveNoFileCompletion | DirectiveNoSpace,
	}
}

func actionFileContents(ctx Context, filePath string, parser func(content []byte) []Item) Predictor {
	if filePath == "" || parser == nil {
		return None()
	}

	content, err := os.ReadFile(resolveFilePath(ctx, filePath))
	if err != nil {
		return None()
	}

	return Items(parser(content)...)
}

func actionFiles(ctx Context, extensions ...string) Predictor {
	return Predictor{
		items:     scanFilesystem(ctx, fsModeFiles, extensions, nil),
		directive: DirectiveNoFileCompletion,
	}
}

func actionFilesFiltered(ctx Context, filter func(path string, entry os.DirEntry) bool) Predictor {
	return Predictor{
		items:     scanFilesystem(ctx, fsModeFiles, nil, filter),
		directive: DirectiveNoFileCompletion,
	}
}

func actionFilesGlob(ctx Context, patterns []string) Predictor {
	filter := func(_ string, entry os.DirEntry) bool {
		return matchesAnyGlob(entry.Name(), patterns)
	}

	return Predictor{
		items:     scanFilesystem(ctx, fsModeFiles, nil, filter),
		directive: DirectiveNoFileCompletion,
	}
}

func actionPathsFiltered(ctx Context, filter func(path string, entry os.DirEntry) bool) Predictor {
	return Predictor{
		items:     scanFilesystem(ctx, fsModePaths, nil, filter),
		directive: DirectiveNoFileCompletion,
	}
}

func actionSymlinks(ctx Context) Predictor {
	filter := func(_ string, entry os.DirEntry) bool {
		return entry.Type()&os.ModeSymlink != 0
	}

	return Predictor{
		items:     scanFilesystem(ctx, fsModePaths, nil, filter),
		directive: DirectiveNoFileCompletion,
	}
}

// fsScan holds the settings of one directory scan shared by every entry:
// where the entries live, the typed word split into its directory prefix and
// the name being completed, the scan mode with its extension filter (files
// mode only) and entry predicate, the PATHEXT extensions that mark a file
// executable on Windows, and the remaining budget of execute-bit checks for
// styling (see executableCheckBudget).
type fsScan struct {
	searchDir        string
	dirPrefix        string
	filterTerm       string
	mode             fsScanMode
	extensions       []string
	pathExtensions   []string
	filter           func(path string, entry os.DirEntry) bool
	executableChecks int
}

// fsLink describes a symbolic link entry, resolved once per entry: whether
// the entry is a link, the target's file info (nil when the target is
// missing), and the link text from os.Readlink ("" when it was not read or
// could not be).
type fsLink struct {
	isLink      bool
	target      os.FileInfo
	destination string
}

// executableCheckBudget caps the execute-bit checks one scan spends on
// styling. Each check is an lstat, and a scan of a very large directory must
// stay within the predictor timeout, so files past the budget are styled by
// name instead.
const executableCheckBudget = 1024

func newFSScan(ctx Context, mode fsScanMode, extensions []string, filter func(path string, entry os.DirEntry) bool) *fsScan {
	dirPrefix, filterTerm := splitPathTarget(ctx.ToComplete)

	return &fsScan{
		searchDir:        resolveSearchDir(ctx, dirPrefix),
		dirPrefix:        dirPrefix,
		filterTerm:       filterTerm,
		mode:             mode,
		extensions:       extensions,
		pathExtensions:   windowsPathExts(ctx),
		filter:           filter,
		executableChecks: executableCheckBudget,
	}
}

// candidateItems turns every directory entry the scan accepts into an item.
func (scan *fsScan) candidateItems(entries []os.DirEntry) []Item {
	items := make([]Item, 0, len(entries))

	for _, entry := range entries {
		if item, ok := scan.candidateItem(entry); ok {
			items = append(items, item)
		}
	}

	return items
}

func (scan *fsScan) candidateItem(entry os.DirEntry) (Item, bool) {
	name := entry.Name()
	if strings.HasPrefix(name, ".") && !strings.HasPrefix(scan.filterTerm, ".") {
		return Item{}, false
	}

	fullPath := filepath.Join(scan.searchDir, name)
	link := scan.resolveLink(entry, fullPath)
	isDir := entry.IsDir() || (link.target != nil && link.target.IsDir())

	if !scan.matchesMode(entry, fullPath, isDir) {
		return Item{}, false
	}

	candidateValue := scan.dirPrefix + name
	displayValue := name

	if isDir {
		candidateValue += "/"
		displayValue += "/"
	}

	tag := "files"
	if isDir {
		tag = "directories"
	}

	if scan.mode == fsModeExecutables && !isDir {
		tag = "executables"
	}

	var description string

	// Executable candidates keep the executables tag and style: the symlink
	// decoration belongs to the generic filesystem modes.
	if scan.mode != fsModeExecutables && link.isLink {
		tag = "symlinks"

		if link.destination != "" {
			description = "-> " + link.destination
		}
	}

	return Item{
		Value:       candidateValue,
		Display:     displayValue,
		Description: description,
		Style:       scan.entryStyle(entry, isDir, link),
		Tag:         tag,
	}, true
}

// resolveLink stats a symlink entry's target once, and reads the link text
// in the generic filesystem modes, where it feeds the description and the
// ln=target style.
func (scan *fsScan) resolveLink(entry os.DirEntry, fullPath string) fsLink {
	if entry.Type()&os.ModeSymlink == 0 {
		return fsLink{}
	}

	link := fsLink{isLink: true}

	if target, err := os.Stat(fullPath); err == nil {
		link.target = target
	}

	if scan.mode != fsModeExecutables {
		if destination, err := os.Readlink(fullPath); err == nil {
			link.destination = destination
		}
	}

	return link
}

// entryStyle picks a candidate's style by kind, in ls order: a symlink
// (generic filesystem modes only) by its link state, a directory, an
// executable file, then a file by its name. Every file the Executables
// predictor lists is executable. Elsewhere a file is checked for an execute
// bit or a PATHEXT extension when it can still match the typed name and the
// check budget is not spent (styledAsExecutable).
func (scan *fsScan) entryStyle(entry os.DirEntry, isDir bool, link fsLink) string {
	if scan.mode != fsModeExecutables && link.isLink {
		return scan.symlinkStyle(entry.Name(), link)
	}

	switch {
	case isDir:
		return style.ForDirectory()
	case scan.mode == fsModeExecutables || scan.styledAsExecutable(entry):
		return style.ForExecutable()
	default:
		return style.ForPathExt(entry.Name())
	}
}

// styledAsExecutable reports whether a file in a generic filesystem mode
// takes the executable style. The check costs an lstat on Unix, so it runs
// only for names that can survive the engine's smart-case prefix filter of
// the typed word, and at most executableCheckBudget times per scan.
func (scan *fsScan) styledAsExecutable(entry os.DirEntry) bool {
	if runtime.GOOS == "windows" {
		return hasPathExtension(entry.Name(), scan.pathExtensions)
	}

	if scan.executableChecks <= 0 {
		return false
	}

	if term := strings.Trim(scan.filterTerm, `'"`); term != "" && !matchesSmartCase(entry.Name(), term, strings.ToLower(term)) {
		return false
	}

	scan.executableChecks--

	return isExecutable(entry, scan.pathExtensions)
}

// symlinkStyle styles a symbolic link: the orphan style when its target is
// missing, the link style otherwise, or under "ln=target" the style its
// target would get, judged by the target's name as ls does.
func (scan *fsScan) symlinkStyle(name string, link fsLink) string {
	if link.target == nil {
		return style.ForOrphanSymlink()
	}

	if !style.SymlinkAsTarget() {
		return style.ForSymlink()
	}

	if link.destination != "" {
		name = filepath.Base(link.destination)
	}

	switch {
	case link.target.IsDir():
		return style.ForDirectory()
	case isExecutableFile(name, link.target.Mode(), scan.pathExtensions):
		return style.ForExecutable()
	default:
		return style.ForPathExt(name)
	}
}

func (scan *fsScan) matchesMode(entry os.DirEntry, fullPath string, isDir bool) bool {
	switch scan.mode {
	case fsModeDirs:
		return isDir && passesFilter(fullPath, entry, scan.filter)

	case fsModeFiles:
		return isDir || (matchesExtension(entry.Name(), scan.extensions) && passesFilter(fullPath, entry, scan.filter))

	case fsModePaths:
		return passesFilter(fullPath, entry, scan.filter)

	case fsModeExecutables:
		return isDir || isExecutable(entry, scan.pathExtensions)

	default:
		return true
	}
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

func matchesExtension(name string, extensions []string) bool {
	return len(extensions) == 0 || matchesAnyExtension(name, extensions)
}

func passesFilter(fullPath string, entry os.DirEntry, filter func(path string, entry os.DirEntry) bool) bool {
	return filter == nil || filter(fullPath, entry)
}

func parseLinesFile(content []byte) []Item {
	var items []Item

	_ = scanx.ScanLines(bytes.NewReader(content), func(line string) bool { //nolint:errcheck // best-effort candidate scan
		items = append(items, Item{
			Value:   line,
			Display: line,
		})

		return true
	})

	return items
}

func resolveFilePath(ctx Context, rawPath string) string {
	if strings.HasPrefix(rawPath, "~/") || rawPath == "~" {
		return pathx.ExpandTilde(rawPath, ctx.Getenv)
	}

	if filepath.IsAbs(rawPath) {
		return rawPath
	}

	return filepath.Join(cmp.Or(ctx.Dir, "."), filepath.FromSlash(rawPath))
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
	if ctx.ToComplete == "~" {
		return []Item{
			{
				Value:   "~/",
				Display: "~/",
				Style:   style.ForDirectory(),
				Tag:     "directories",
			},
		}
	}

	scan := newFSScan(ctx, mode, extensions, filter)

	entries, err := os.ReadDir(scan.searchDir)
	if err != nil {
		return nil
	}

	return scan.candidateItems(entries)
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
