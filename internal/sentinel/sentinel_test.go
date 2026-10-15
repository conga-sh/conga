package sentinel_test

import (
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/sentinel"
)

func TestSentinelErrors_Unwrap(t *testing.T) {
	t.Parallel()

	t.Run("config read unwraps to ErrConfig", func(t *testing.T) {
		t.Parallel()

		assert.True(t, errors.Is(sentinel.ErrConfigRead, sentinel.ErrConfig))
		assert.Equal(t, "config error: reading config file", sentinel.ErrConfigRead.Error())
	})

	t.Run("config parse unwraps to ErrConfig", func(t *testing.T) {
		t.Parallel()

		assert.True(t, errors.Is(sentinel.ErrConfigParse, sentinel.ErrConfig))
		assert.Equal(t, "config error: parsing config file", sentinel.ErrConfigParse.Error())
	})

	t.Run("config unknown field unwraps to ErrConfig", func(t *testing.T) {
		t.Parallel()

		assert.True(t, errors.Is(sentinel.ErrConfigUnknownField, sentinel.ErrConfig))
		assert.Equal(t, "config error: unknown field", sentinel.ErrConfigUnknownField.Error())
	})

	t.Run("config conflict unwraps to ErrConfig", func(t *testing.T) {
		t.Parallel()

		assert.True(t, errors.Is(sentinel.ErrConfigConflict, sentinel.ErrConfig))
		assert.Equal(t, "config error: multiple conflicting config files found", sentinel.ErrConfigConflict.Error())
	})

	t.Run("dotenv read unwraps to ErrDotenv", func(t *testing.T) {
		t.Parallel()

		assert.True(t, errors.Is(sentinel.ErrDotenvRead, sentinel.ErrDotenv))
		assert.Equal(t, "dotenv error: reading .env file", sentinel.ErrDotenvRead.Error())
	})

	t.Run("dotenv parse unwraps to ErrDotenv", func(t *testing.T) {
		t.Parallel()

		assert.True(t, errors.Is(sentinel.ErrDotenvParse, sentinel.ErrDotenv))
		assert.Equal(t, "dotenv error: parsing .env file", sentinel.ErrDotenvParse.Error())
	})
}
