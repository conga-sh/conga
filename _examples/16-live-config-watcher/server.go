package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/conga-sh/conga"
)

// ConfigResponse represents the JSON response returned by the /config endpoint.
type ConfigResponse struct {
	Port             int   `json:"port"`
	RateLimit        int   `json:"rate_limit"`
	Debug            bool  `json:"debug"`
	BlocklistEntries int64 `json:"blocklist_entries"`
	Reloads          int64 `json:"reloads"`
}

func printWatcherStatus(session *conga.Session) {
	stdout := session.Stdout()

	candidates := session.CandidateConfigFiles()
	if len(candidates) == 1 {
		fmt.Fprintf(stdout, "[WATCHER] Live watcher active on %q (listening for updates)\n", candidates[0])
	} else if len(candidates) > 1 {
		fmt.Fprintf(stdout, "[WATCHER] Live watcher active on %v (listening for updates)\n", candidates)
	}

	if loaded := session.LoadedConfigFiles(); len(loaded) > 0 {
		fmt.Fprintf(stdout, "[WATCHER] Loaded config files: %v\n", loaded)
	}
}

// countFileLines returns the number of non-empty lines in a file.
func countFileLines(filePath string) int64 {
	content, err := os.ReadFile(filePath) //nolint:gosec // The path comes from the application's own flag
	if err != nil {
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0
	}

	return int64(len(lines))
}
