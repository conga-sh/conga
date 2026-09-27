package filex_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/filex"
)

func TestReadLimited(t *testing.T) {
	t.Parallel()

	t.Run("reads file within size limit", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "small.txt")
		expected := []byte("hello world")
		err := os.WriteFile(filePath, expected, 0o600)
		assert.NoError(t, err)

		data, err := filex.ReadLimited(filePath, 100)

		assert.NoError(t, err)
		assert.Equal(t, expected, data)
	})

	t.Run("errors when regular file exceeds size limit on stat", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "large.txt")
		content := []byte(strings.Repeat("a", 200))
		err := os.WriteFile(filePath, content, 0o600)
		assert.NoError(t, err)

		data, err := filex.ReadLimited(filePath, 100)

		assert.Error(t, err)
		assert.IsError(t, err, filex.ErrFileSizeLimitExceeded)
		assert.Zero(t, data)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		missingPath := filepath.Join(dir, "does_not_exist.txt")

		data, err := filex.ReadLimited(missingPath, 100)

		assert.Error(t, err)
		assert.IsError(t, err, os.ErrNotExist)
		assert.Zero(t, data)
	})

	t.Run("accepts file at exactly the size limit", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "exact.txt")
		expected := []byte(strings.Repeat("a", 100))
		err := os.WriteFile(filePath, expected, 0o600)
		assert.NoError(t, err)

		data, err := filex.ReadLimited(filePath, 100)

		assert.NoError(t, err)
		assert.Equal(t, expected, data)
	})

	t.Run("returns non-limit error for directory input", func(t *testing.T) {
		t.Parallel()

		data, err := filex.ReadLimited(t.TempDir(), 100)

		assert.Error(t, err)
		assert.NotIsError(t, err, filex.ErrFileSizeLimitExceeded)
		assert.Zero(t, data)
	})
}

func TestReadLimitedReader(t *testing.T) {
	t.Parallel()

	t.Run("reads reader within limit", func(t *testing.T) {
		t.Parallel()

		content := []byte("stream content")
		reader := bytes.NewReader(content)

		data, err := filex.ReadLimitedReader(reader, 100)

		assert.NoError(t, err)
		assert.Equal(t, content, data)
	})

	t.Run("errors when stream exceeds limit", func(t *testing.T) {
		t.Parallel()

		content := []byte(strings.Repeat("x", 50))
		reader := bytes.NewReader(content)

		data, err := filex.ReadLimitedReader(reader, 20)

		assert.Error(t, err)
		assert.IsError(t, err, filex.ErrFileSizeLimitExceeded)
		assert.Zero(t, data)
	})

	t.Run("reads empty stream", func(t *testing.T) {
		t.Parallel()

		reader := bytes.NewReader([]byte{})

		data, err := filex.ReadLimitedReader(reader, 10)

		assert.NoError(t, err)
		assert.Equal(t, []byte{}, data)
	})

	t.Run("accepts stream at exactly the limit", func(t *testing.T) {
		t.Parallel()

		content := []byte(strings.Repeat("x", 20))
		reader := bytes.NewReader(content)

		data, err := filex.ReadLimitedReader(reader, 20)

		assert.NoError(t, err)
		assert.Equal(t, content, data)
	})
}
