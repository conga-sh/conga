package help

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestFormatVersionSummary(t *testing.T) {
	t.Parallel()

	t.Run("long commit shortened to twelve characters", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{Version: "1.2.3", Commit: "abcdef1234567890", GoVersion: "go1.27.1"}

		assert.Equal(t, "app has version 1.2.3 built with go1.27.1 from abcdef123456", formatVersionSummary("app", info))
	})

	t.Run("short commit kept verbatim", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{Version: "1.2.3", Commit: "abc", GoVersion: "go1.27.1"}

		assert.Equal(t, "app has version 1.2.3 built with go1.27.1 from abc", formatVersionSummary("app", info))
	})

	t.Run("dirty suffix appended to commit", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{Version: "1.2.3", Commit: "abcdef1234567890", Dirty: true, GoVersion: "go1.27.1"}

		assert.Equal(t, "app has version 1.2.3 built with go1.27.1 from abcdef123456-dirty", formatVersionSummary("app", info))
	})

	t.Run("dirty without commit omits the commit clause", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{Version: "1.2.3", Dirty: true, GoVersion: "go1.27.1"}

		assert.Equal(t, "app has version 1.2.3 built with go1.27.1", formatVersionSummary("app", info))
	})

	t.Run("commit and timestamp appended", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{
			Version:    "1.2.3",
			Commit:     "abcdef1234567890",
			CommitDate: "2026-09-15T10:30:00Z",
			GoVersion:  "go1.27.1",
		}

		assert.Equal(t, "app has version 1.2.3 built with go1.27.1 from abcdef123456 on 2026-09-15T10:30:00Z", formatVersionSummary("app", info))
	})

	t.Run("pseudo-version does not repeat commit and timestamp", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{
			Version:    "v0.0.0-20260925165149-c2e2ea84591f",
			Commit:     "c2e2ea84591f0ef557489d0f7f2b0b1565555950",
			CommitDate: "2026-09-25T16:51:49Z",
			GoVersion:  "go1.27.1",
		}

		assert.Equal(t, "app has version v0.0.0-20260925165149-c2e2ea84591f built with go1.27.1", formatVersionSummary("app", info))
	})

	t.Run("dirty pseudo-version does not repeat the dirty marker", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{
			Version:    "v0.0.0-20260925165149-c2e2ea84591f+dirty",
			Commit:     "c2e2ea84591f0ef557489d0f7f2b0b1565555950",
			CommitDate: "2026-09-25T16:51:49Z",
			Dirty:      true,
			GoVersion:  "go1.27.1",
		}

		assert.Equal(t, "app has version v0.0.0-20260925165149-c2e2ea84591f+dirty built with go1.27.1", formatVersionSummary("app", info))
	})

	t.Run("no extras renders the plain sentence", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{Version: "1.2.3", GoVersion: "go1.27.1"}

		assert.Equal(t, "app has version 1.2.3 built with go1.27.1", formatVersionSummary("app", info))
	})
}

func TestFormatVersionDetailed(t *testing.T) {
	t.Parallel()

	t.Run("full metadata rendered", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{
			Version:    "1.2.3",
			Commit:     "abcdef1234567890",
			CommitDate: "2026-09-15T10:30:00Z",
			GoVersion:  "go1.26.5",
			OS:         "linux",
			Arch:       "amd64",
		}

		expected := "app:\n" +
			" Version:    1.2.3\n" +
			" Git commit: abcdef1234567890\n" +
			" Built:      2026-09-15T10:30:00Z\n" +
			" Go version: go1.26.5\n" +
			" OS/Arch:    linux/amd64"

		assert.Equal(t, expected, formatVersionDetailed("app", info))
	})

	t.Run("dirty commit annotated", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{Version: "1.2.3", Commit: "abcdef1", Dirty: true, GoVersion: "go1.26.5", OS: "linux", Arch: "amd64"}

		assert.Contains(t, formatVersionDetailed("app", info), " Git commit: abcdef1 (dirty)\n")
	})

	t.Run("dirty without commit reports status", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{Version: "1.2.3", Dirty: true, GoVersion: "go1.26.5", OS: "linux", Arch: "amd64"}

		assert.Contains(t, formatVersionDetailed("app", info), " Status:     dirty\n")
	})
}

func TestFormatVersionJSON(t *testing.T) {
	t.Parallel()

	t.Run("all keys rendered", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{
			Version:    "1.2.3",
			Commit:     "abcdef1234567890",
			CommitDate: "2026-09-15T10:30:00Z",
			GoVersion:  "go1.26.5",
			OS:         "linux",
			Arch:       "amd64",
			Dirty:      true,
		}

		expected := `{"version":"1.2.3","gitCommit":"abcdef1234567890","built":"2026-09-15T10:30:00Z","goVersion":"go1.26.5","os":"linux","arch":"amd64","modified":true}`

		assert.Equal(t, expected, formatVersionJSON(info))
	})

	t.Run("unknown metadata renders empty strings", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{Version: "1.2.3", GoVersion: "go1.26.5", OS: "linux", Arch: "amd64"}

		expected := `{"version":"1.2.3","gitCommit":"","built":"","goVersion":"go1.26.5","os":"linux","arch":"amd64","modified":false}`

		assert.Equal(t, expected, formatVersionJSON(info))
	})

	t.Run("version quotes escaped", func(t *testing.T) {
		t.Parallel()

		info := BuildInfoData{Version: `1.2.3"quote"`, GoVersion: "go1.26.5", OS: "linux", Arch: "amd64"}

		assert.Contains(t, formatVersionJSON(info), `"version":"1.2.3\"quote\""`)
	})
}

func TestVersionEmbedsTimestamp(t *testing.T) {
	t.Parallel()

	t.Run("matching RFC3339 UTC timestamp returns true", func(t *testing.T) {
		t.Parallel()

		result := versionEmbedsTimestamp("v0.0.0-20260925165149-c2e2ea84591f", "2026-09-25T16:51:49Z")

		assert.True(t, result)
	})

	t.Run("mismatched timestamp returns false", func(t *testing.T) {
		t.Parallel()

		result := versionEmbedsTimestamp("v0.0.0-20260925165149-c2e2ea84591f", "2026-09-25T16:51:50Z")

		assert.False(t, result)
	})

	t.Run("invalid length returns false", func(t *testing.T) {
		t.Parallel()

		result := versionEmbedsTimestamp("v0.0.0-20260925165149-c2e2ea84591f", "2026-09-25T16:51:49")

		assert.False(t, result)
	})

	t.Run("invalid delimiters return false", func(t *testing.T) {
		t.Parallel()

		result := versionEmbedsTimestamp("v0.0.0-20260925165149-c2e2ea84591f", "2026/09/25 16:51:49Z")

		assert.False(t, result)
	})

	t.Run("non-digit characters return false", func(t *testing.T) {
		t.Parallel()

		result := versionEmbedsTimestamp("v0.0.0-20260925165149-c2e2ea84591f", "2026-09-25T16:51:XXZ")

		assert.False(t, result)
	})
}
