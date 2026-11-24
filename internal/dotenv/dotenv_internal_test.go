package dotenv

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestLoadSingleFile_SetFailureNamesTheFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".env.local")

	assert.NoError(t, os.WriteFile(path, []byte("KEY_FOR_SET_FAILURE=1\n"), 0o600))

	err := loadSingleFile(func(string, string) error { return errors.New("denied") }, path, true, map[string]string{})

	dotenvErr, ok := errors.AsType[*DotenvError](err)

	assert.True(t, ok)
	assert.Equal(t, `cannot set environment variable "KEY_FOR_SET_FAILURE": denied`, dotenvErr.Error())
	assert.Equal(t, []string{".env file \"" + path + "\""}, dotenvErr.Origins())
}

func TestDotenvError_ReadAndParseHaveNoOrigins(t *testing.T) {
	t.Parallel()

	assert.Zero(t, (&DotenvError{Path: ".env", Msg: `cannot read .env file ".env"`}).Origins())
}
