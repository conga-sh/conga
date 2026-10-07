package json_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga/decoder/json"
)

var (
	//go:embed testdata/duplicate_keys.json
	fixtureDuplicateKeys []byte

	//go:embed testdata/invalid_array.json
	fixtureInvalidArray []byte

	//go:embed testdata/invalid_syntax.json
	fixtureInvalidSyntax []byte

	//go:embed testdata/invalid_utf8.json
	fixtureInvalidUTF8 []byte

	//go:embed testdata/trailing_garbage.json
	fixtureTrailingGarbage []byte

	//go:embed testdata/valid_flat.json
	fixtureValidFlat []byte

	//go:embed testdata/valid_nested.json
	fixtureValidNested []byte
)

func TestRawDecoder_ValidFlat(t *testing.T) {
	t.Parallel()

	result, err := json.RawDecoder(fixtureValidFlat)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
	assert.Equal(t, any(float64(8080)), result["port"])
	assert.Equal(t, any(true), result["debug"])
}

func TestRawDecoder_ValidNested(t *testing.T) {
	t.Parallel()

	result, err := json.RawDecoder(fixtureValidNested)

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
	assert.Equal(t, any(float64(30)), config["timeout"])
	assert.Equal(t, any(float64(3)), config["retries"])
}

func TestRawDecoder_InvalidSyntax(t *testing.T) {
	t.Parallel()

	_, err := json.RawDecoder(fixtureInvalidSyntax)

	assert.Error(t, err)
}

func TestRawDecoder_InvalidArray(t *testing.T) {
	t.Parallel()

	_, err := json.RawDecoder(fixtureInvalidArray)

	assert.Error(t, err)
}

func TestRawDecoder_TrailingGarbage(t *testing.T) {
	t.Parallel()

	_, err := json.RawDecoder(fixtureTrailingGarbage)

	assert.Error(t, err)
	assert.IsError(t, err, json.ErrJSONParse)
}

func TestRawDecoder_TrailingWhitespaceSucceeds(t *testing.T) {
	t.Parallel()

	payload := []byte("{\"name\": \"testapp\"}\n  \t  \n")
	result, err := json.RawDecoder(payload)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
}

func TestRawDecoder_EmptyFails(t *testing.T) {
	t.Parallel()

	_, err := json.RawDecoder([]byte(""))

	assert.Error(t, err)
	assert.IsError(t, err, json.ErrJSONParse)
}

func TestRawDecoder_DuplicateKeys(t *testing.T) {
	t.Parallel()

	_, err := json.RawDecoder(fixtureDuplicateKeys)

	assert.Error(t, err)
	assert.IsError(t, err, json.ErrJSONParse)
}

func TestRawDecoder_InvalidUTF8(t *testing.T) {
	t.Parallel()

	_, err := json.RawDecoder(fixtureInvalidUTF8)

	assert.Error(t, err)
	assert.IsError(t, err, json.ErrJSONParse)
}
