package help_test

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/help"
)

func TestVersionSummary(t *testing.T) {
	t.Parallel()

	t.Run("explicit version", func(t *testing.T) {
		t.Parallel()

		summary := help.VersionSummary("testapp", "1.0.0")
		assert.Contains(t, summary, "testapp has version 1.0.0 built with")
	})

	t.Run("empty version defaults to dev", func(t *testing.T) {
		t.Parallel()

		summary := help.VersionSummary("testapp", "")
		assert.Contains(t, summary, "testapp has version dev built with")
	})
}

func TestVersionDetailed(t *testing.T) {
	t.Parallel()

	t.Run("explicit version", func(t *testing.T) {
		t.Parallel()

		detailed := help.VersionDetailed("testapp", "1.0.0")
		assert.Contains(t, detailed, "testapp:\n")
		assert.Contains(t, detailed, "Version:    1.0.0")
		assert.Contains(t, detailed, "Go version:")
		assert.Contains(t, detailed, "OS/Arch:")
	})

	t.Run("empty version defaults to dev", func(t *testing.T) {
		t.Parallel()

		detailed := help.VersionDetailed("testapp", "")
		assert.Contains(t, detailed, "testapp:\n")
		assert.Contains(t, detailed, "Version:    dev")
	})
}

func TestVersionJSON(t *testing.T) {
	t.Parallel()

	var decoded map[string]any

	assert.NoError(t, json.Unmarshal([]byte(help.VersionJSON("1.0.0")), &decoded))
	assert.Equal(t, "1.0.0", decoded["version"])
	assert.Equal(t, false, decoded["modified"])
	assert.Equal[any](t, runtime.GOOS, decoded["os"])
	assert.Equal[any](t, runtime.GOARCH, decoded["arch"])
}

func TestGetBuildInfo(t *testing.T) {
	t.Parallel()

	info := help.GetBuildInfo("")
	assert.Equal(t, "dev", info.Version)
	assert.NotZero(t, info.GoVersion)
	assert.NotZero(t, info.OS)
	assert.NotZero(t, info.Arch)
}
