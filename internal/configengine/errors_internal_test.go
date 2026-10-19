package configengine

import (
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/sentinel"
)

func TestConfigError_Error(t *testing.T) {
	t.Parallel()

	t.Run("explicit msg takes precedence", func(t *testing.T) {
		t.Parallel()

		err := &configError{
			path:  "config.yaml",
			msg:   "custom error message",
			cause: errors.New("underlying cause"),
		}

		msg := err.Error()

		assert.Equal(t, "custom error message", msg)
	})

	t.Run("formats path and cause when msg is empty", func(t *testing.T) {
		t.Parallel()

		err := &configError{
			path:  "app.json",
			cause: errors.New("permission denied"),
		}

		msg := err.Error()

		assert.Equal(t, `config file "app.json": permission denied`, msg)
	})

	t.Run("formats path when only path provided", func(t *testing.T) {
		t.Parallel()

		err := &configError{
			path: "app.json",
		}

		msg := err.Error()

		assert.Equal(t, `config file "app.json" error`, msg)
	})

	t.Run("returns cause error when only cause provided", func(t *testing.T) {
		t.Parallel()

		err := &configError{
			cause: errors.New("read failed"),
		}

		msg := err.Error()

		assert.Equal(t, "read failed", msg)
	})

	t.Run("returns generic error when all fields empty", func(t *testing.T) {
		t.Parallel()

		err := &configError{}

		msg := err.Error()

		assert.Equal(t, "config error", msg)
	})
}

func TestConfigError_Classification(t *testing.T) {
	t.Parallel()

	err := &configError{path: "app.json", cause: errors.New("read failed")}

	assert.IsError(t, err, sentinel.ErrConfig)
}
