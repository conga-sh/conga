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

	//go:embed testdata/null.json
	fixtureNull []byte

	//go:embed testdata/trailing_garbage.json
	fixtureTrailingGarbage []byte

	//go:embed testdata/valid_flat.json
	fixtureValidFlat []byte

	//go:embed testdata/valid_nested.json
	fixtureValidNested []byte
)

func TestDecode_ValidFlat(t *testing.T) {
	t.Parallel()

	result, err := json.Decode(fixtureValidFlat)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
	assert.Equal(t, any(float64(8080)), result["port"])
	assert.Equal(t, any(true), result["debug"])
}

func TestDecode_ValidNested(t *testing.T) {
	t.Parallel()

	result, err := json.Decode(fixtureValidNested)

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

func TestDecode_InvalidSyntax(t *testing.T) {
	t.Parallel()

	_, err := json.Decode(fixtureInvalidSyntax)

	assert.IsError(t, err, json.ErrParse)
}

func TestDecode_InvalidArray(t *testing.T) {
	t.Parallel()

	_, err := json.Decode(fixtureInvalidArray)

	assert.IsError(t, err, json.ErrParse)
}

func TestDecode_TrailingGarbage(t *testing.T) {
	t.Parallel()

	_, err := json.Decode(fixtureTrailingGarbage)

	assert.IsError(t, err, json.ErrParse)
}

func TestDecode_TrailingWhitespaceSucceeds(t *testing.T) {
	t.Parallel()

	payload := []byte("{\"name\": \"testapp\"}\n  \t  \n")
	result, err := json.Decode(payload)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
}

func TestDecode_EmptyFails(t *testing.T) {
	t.Parallel()

	_, err := json.Decode([]byte(""))

	assert.IsError(t, err, json.ErrParse)
}

func TestDecode_NullDocument(t *testing.T) {
	t.Parallel()

	result, err := json.Decode(fixtureNull)

	assert.NoError(t, err)
	assert.True(t, result != nil)
	assert.Zero(t, result)
}

func TestDecode_DuplicateKeys(t *testing.T) {
	t.Parallel()

	_, err := json.Decode(fixtureDuplicateKeys)

	assert.IsError(t, err, json.ErrParse)
}

func TestDecode_InvalidUTF8(t *testing.T) {
	t.Parallel()

	_, err := json.Decode(fixtureInvalidUTF8)

	assert.IsError(t, err, json.ErrParse)
}
