package predict

import "os"

// FileFilterable is implemented by custom domain file types that filter files with a custom predicate.
type FileFilterable interface {
	MatchFile(path string, entry os.DirEntry) bool
}

// DirFilterable is implemented by custom domain directory types that filter
// directories with a custom predicate. The engine only evaluates MatchDir for
// directory entries, including symlinks that resolve to directories.
type DirFilterable interface {
	MatchDir(path string, entry os.DirEntry) bool
}

// PathFilterable is implemented by custom domain path types that evaluate both files and directories.
type PathFilterable interface {
	MatchPath(path string, entry os.DirEntry) bool
}
