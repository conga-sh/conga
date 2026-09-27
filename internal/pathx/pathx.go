package pathx

import (
	"os"
	"path/filepath"
	"strings"
)

// Expand resolves environment variables ($VAR, ${VAR}, ${VAR:-default})
// and tilde (~, ~/) into a clean, platform-specific path.
func Expand(path string) string {
	if path == "" {
		return ""
	}

	expanded := os.Expand(path, func(key string) string {
		if strings.Contains(key, ":-") {
			parts := strings.SplitN(key, ":-", 2)
			if value := os.Getenv(parts[0]); value != "" {
				return value
			}

			return parts[1]
		}

		return os.Getenv(key)
	})

	return filepath.Clean(ExpandTilde(expanded, os.Getenv))
}

// HomeDir returns the user's home directory: the getenv("HOME") value when
// non-empty, otherwise os.UserHomeDir, otherwise "" when the home directory
// cannot be resolved.
func HomeDir(getenv func(string) string) string {
	if getenv != nil {
		if home := getenv("HOME"); home != "" {
			return home
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return home
}

// ExpandTilde resolves a leading tilde (~, ~/) into the user's home directory
// resolved via HomeDir. Paths without a tilde prefix are returned unchanged,
// as are tilde paths when the home directory cannot be resolved.
func ExpandTilde(path string, getenv func(string) string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path
	}

	home := HomeDir(getenv)
	if home == "" {
		return path
	}

	if path == "~" {
		return home
	}

	return filepath.Join(home, path[2:])
}
