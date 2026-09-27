package yaml_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga/decoder/yaml"
)

var (
	//go:embed testdata/invalid_syntax.yaml
	fixtureInvalidSyntax []byte

	//go:embed testdata/trailing_garbage.yaml
	fixtureTrailingGarbage []byte

	//go:embed testdata/valid_flat.yaml
	fixtureValidFlat []byte

	//go:embed testdata/valid_nested.yaml
	fixtureValidNested []byte
)

func TestRawDecoder_ValidFlat(t *testing.T) {
	t.Parallel()

	result, err := yaml.RawDecoder(fixtureValidFlat)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
	assert.Equal(t, any(8080), result["port"])
	assert.Equal(t, any(true), result["debug"])
}

func TestRawDecoder_ValidNested(t *testing.T) {
	t.Parallel()

	result, err := yaml.RawDecoder(fixtureValidNested)

	assert.NoError(t, err)

	tags, ok := result["tags"].([]any)
	assert.True(t, ok)
	assert.Equal(t, 2, len(tags))
	assert.Equal(t, any("prod"), tags[0])
	assert.Equal(t, any("v2"), tags[1])

	cluster, ok := result["cluster"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, any("us-east-1"), cluster["region"])

	node, ok := cluster["node"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, any("us-east-1a"), node["zone"])

	config, ok := node["config"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, any(30), config["timeout"])
	assert.Equal(t, any(3), config["retries"])
}

func TestRawDecoder_InvalidSyntax(t *testing.T) {
	t.Parallel()

	_, err := yaml.RawDecoder(fixtureInvalidSyntax)

	assert.Error(t, err)
}

func TestRawDecoder_TrailingGarbage(t *testing.T) {
	t.Parallel()

	_, err := yaml.RawDecoder(fixtureTrailingGarbage)

	assert.Error(t, err)
	assert.IsError(t, err, yaml.ErrYAMLParse)
}

func TestRawDecoder_TrailingWhitespaceSucceeds(t *testing.T) {
	t.Parallel()

	payload := []byte("name: testapp\n  \n")
	result, err := yaml.RawDecoder(payload)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
}

func TestRawDecoder_EmptyValid(t *testing.T) {
	t.Parallel()

	result, err := yaml.RawDecoder([]byte(""))

	assert.NoError(t, err)
	assert.Zero(t, result)
}
