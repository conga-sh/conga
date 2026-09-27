package pathx_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/pathx"
)

func TestExpand_Tilde(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	assert.NoError(t, err)

	assert.Equal(t, home, pathx.Expand("~"))
	assert.Equal(t, filepath.Join(home, "config.yaml"), pathx.Expand("~/config.yaml"))
	assert.Equal(t, filepath.Join(home, ".config", "app.json"), pathx.Expand("~/.config/app.json"))
}

func TestExpand_EnvVars(t *testing.T) {
	t.Setenv("TEST_APP_CONF_DIR", "/custom/dir")
	t.Setenv("TEST_APP_FILE", "app.yaml")

	assert.Equal(t, filepath.Clean("/custom/dir/app.yaml"), pathx.Expand("$TEST_APP_CONF_DIR/$TEST_APP_FILE"))
	assert.Equal(t, filepath.Clean("/custom/dir/app.yaml"), pathx.Expand("${TEST_APP_CONF_DIR}/${TEST_APP_FILE}"))
	assert.Equal(t, filepath.Clean("/custom/dir/default.yaml"), pathx.Expand("${UNSET_VAR:-/custom/dir/default.yaml}"))
}

func TestExpand_Empty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", pathx.Expand(""))
}

func TestHomeDir(t *testing.T) {
	t.Parallel()

	t.Run("getenv HOME wins when non-empty", func(t *testing.T) {
		t.Parallel()

		getenv := func(key string) string {
			if key == "HOME" {
				return "/custom/home"
			}

			return ""
		}

		assert.Equal(t, "/custom/home", pathx.HomeDir(getenv))
	})

	t.Run("empty HOME falls back to os.UserHomeDir", func(t *testing.T) {
		t.Parallel()

		getenv := func(string) string { return "" }

		home, err := os.UserHomeDir()
		assert.NoError(t, err)

		assert.Equal(t, home, pathx.HomeDir(getenv))
	})

	t.Run("nil getenv falls back to os.UserHomeDir", func(t *testing.T) {
		t.Parallel()

		home, err := os.UserHomeDir()
		assert.NoError(t, err)

		assert.Equal(t, home, pathx.HomeDir(nil))
	})
}

func TestExpandTilde(t *testing.T) {
	t.Parallel()

	t.Run("bare tilde resolves to home", func(t *testing.T) {
		t.Parallel()

		getenv := func(key string) string {
			if key == "HOME" {
				return "/custom/home"
			}

			return ""
		}

		assert.Equal(t, "/custom/home", pathx.ExpandTilde("~", getenv))
	})
	t.Run("tilde slash resolves into home", func(t *testing.T) {
		t.Parallel()

		home := "/custom/home"
		getenv := func(key string) string {
			if key == "HOME" {
				return home
			}

			return ""
		}

		assert.Equal(t, filepath.Join(home, "docs", "file.txt"), pathx.ExpandTilde("~/docs/file.txt", getenv))
	})

	t.Run("no tilde prefix is unchanged", func(t *testing.T) {
		t.Parallel()

		getenv := func(key string) string {
			if key == "HOME" {
				return "/custom/home"
			}

			return ""
		}

		assert.Equal(t, "relative/path.txt", pathx.ExpandTilde("relative/path.txt", getenv))
		assert.Equal(t, "/abs/path.txt", pathx.ExpandTilde("/abs/path.txt", getenv))
	})
}

// TestExpandTilde_UnresolvableHome covers the home-unresolvable corner: with
// no HOME from getenv and os.UserHomeDir failing, the path is left unchanged
// rather than silently resolved against an empty home.
//

func TestExpandTilde_UnresolvableHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.UserHomeDir on Windows falls back to USERPROFILE, so the unresolvable case cannot be forced")
	}

	t.Setenv("HOME", "")

	getenv := func(string) string { return "" }

	assert.Equal(t, "~/docs", pathx.ExpandTilde("~/docs", getenv))
}
