package conga_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/hcl"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
)

type ConfigCompleteSingleCLI struct {
	ConfigFile conga.ConfigFile        `help:"Config file path" short:"c"`
	Port       conga.OptionalFlag[int] `default:"8080"`
}

type ConfigCompleteMultiCLI struct {
	ConfigFiles conga.ConfigFile        `help:"Config files to merge" multi:"true" short:"c"`
	Port        conga.OptionalFlag[int] `default:"8080"`
}

func TestConfigFile_Completion_RestrictedToSingleDecoder(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.toml"), []byte("port = 8080\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.json"), []byte("{\"port\": 8080}\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.txt"), []byte("hello\n"), 0o600))
	assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "subdir"), 0o755))

	app := conga.New[ConfigCompleteSingleCLI]().
		EnableCompletion().
		RegisterDecoder(".toml", toml.Decode)

	result, _ := app.CompleteForShell("fish", []string{"--config-file", filepath.Join(tmpDir, "") + string(filepath.Separator)})

	values := slicex.Map(result.Items, func(item predict.Item) string { return item.Value })

	assert.True(t, slices.Contains(values, filepath.Join(tmpDir, "app.toml")))
	assert.True(t, slices.Contains(values, filepath.Join(tmpDir, "subdir")+string(filepath.Separator)))
	assert.False(t, slices.Contains(values, filepath.Join(tmpDir, "app.json")))
	assert.False(t, slices.Contains(values, filepath.Join(tmpDir, "app.txt")))
}

func TestConfigFile_Completion_MultipleDecoders(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.toml"), []byte("port = 8080\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.json"), []byte("{\"port\": 8080}\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.hcl"), []byte("port = 8080\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.txt"), []byte("hello\n"), 0o600))

	app := conga.New[ConfigCompleteSingleCLI]().
		EnableCompletion().
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".hcl", hcl.Decode)

	result, _ := app.CompleteForShell("fish", []string{"--config-file", filepath.Join(tmpDir, "") + string(filepath.Separator)})

	values := slicex.Map(result.Items, func(item predict.Item) string { return item.Value })

	assert.True(t, slices.Contains(values, filepath.Join(tmpDir, "app.toml")))
	assert.True(t, slices.Contains(values, filepath.Join(tmpDir, "app.hcl")))
	assert.False(t, slices.Contains(values, filepath.Join(tmpDir, "app.json")))
	assert.False(t, slices.Contains(values, filepath.Join(tmpDir, "app.txt")))
}

func TestConfigFile_Completion_UnconstrainedWhenNoDecoders(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.toml"), []byte("port = 8080\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.json"), []byte("{\"port\": 8080}\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.txt"), []byte("hello\n"), 0o600))

	app := conga.New[ConfigCompleteSingleCLI]().EnableCompletion()

	result, _ := app.CompleteForShell("fish", []string{"--config-file", filepath.Join(tmpDir, "") + string(filepath.Separator)})

	values := slicex.Map(result.Items, func(item predict.Item) string { return item.Value })

	assert.True(t, slices.Contains(values, filepath.Join(tmpDir, "app.toml")))
	assert.True(t, slices.Contains(values, filepath.Join(tmpDir, "app.json")))
	assert.True(t, slices.Contains(values, filepath.Join(tmpDir, "app.txt")))
}

func TestConfigFile_Completion_ConfigFilesMulti(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.toml"), []byte("port = 8080\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.txt"), []byte("hello\n"), 0o600))

	app := conga.New[ConfigCompleteMultiCLI]().
		EnableCompletion().
		RegisterDecoder(".toml", toml.Decode)

	result, _ := app.CompleteForShell("fish", []string{"--config-file", filepath.Join(tmpDir, "") + string(filepath.Separator)})

	values := slicex.Map(result.Items, func(item predict.Item) string { return item.Value })

	assert.True(t, slices.Contains(values, filepath.Join(tmpDir, "app.toml")))
	assert.False(t, slices.Contains(values, filepath.Join(tmpDir, "app.txt")))
}

func TestCompletion_ShellDetectionUnified(t *testing.T) {
	t.Setenv("CONGA_SHELL", "fish")

	app := conga.New[ConfigCompleteSingleCLI]().EnableCompletion()
	_, exportFormatted := app.CompleteForShell("export", []string{"--"})

	result, formatted := app.CompleteForShell("", []string{"--"})

	assert.True(t, len(result.Items) > 0)
	assert.NotEqual(t, exportFormatted, formatted, "CONGA_SHELL=fish must select fish framing, not the export fallback")
}
