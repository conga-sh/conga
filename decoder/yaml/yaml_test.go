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

	//go:embed testdata/numbers.yaml
	fixtureNumbers []byte

	//go:embed testdata/trailing_garbage.yaml
	fixtureTrailingGarbage []byte

	//go:embed testdata/valid_flat.yaml
	fixtureValidFlat []byte

	//go:embed testdata/valid_nested.yaml
	fixtureValidNested []byte
)

func TestDecode_ValidFlat(t *testing.T) {
	t.Parallel()

	result, err := yaml.Decode(fixtureValidFlat)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
	assert.Equal(t, any(8080), result["port"])
	assert.Equal(t, any(true), result["debug"])
}

func TestDecode_ValidNested(t *testing.T) {
	t.Parallel()

	result, err := yaml.Decode(fixtureValidNested)

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

func TestDecode_InvalidSyntax(t *testing.T) {
	t.Parallel()

	_, err := yaml.Decode(fixtureInvalidSyntax)

	assert.IsError(t, err, yaml.ErrParse)
}

func TestDecode_TrailingGarbage(t *testing.T) {
	t.Parallel()

	_, err := yaml.Decode(fixtureTrailingGarbage)

	assert.IsError(t, err, yaml.ErrParse)
}

func TestDecode_TrailingWhitespaceSucceeds(t *testing.T) {
	t.Parallel()

	payload := []byte("name: testapp\n  \n")
	result, err := yaml.Decode(payload)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
}

func TestDecode_EmptyValid(t *testing.T) {
	t.Parallel()

	result, err := yaml.Decode([]byte(""))

	assert.NoError(t, err)
	assert.Zero(t, result)
}

func TestDecode_NilInput(t *testing.T) {
	t.Parallel()

	result, err := yaml.Decode(nil)

	assert.NoError(t, err)
	assert.Zero(t, result)
}

func TestDecode_NumbersWithin64Bits(t *testing.T) {
	t.Parallel()

	result, err := yaml.Decode(fixtureNumbers)

	assert.NoError(t, err)
	assert.Equal(t, any(1000000), result["million"])
	assert.Equal(t, any(uint64(18446744073709551615)), result["max_uint64"])
	assert.Equal(t, any("999999999999999999999999999999"), result["quoted"])
	assert.Equal(t, any(1e30), result["tagged"])
	assert.Equal(t, any(8.0), result["leading_zero"])
	assert.Equal(t, any(1000), result["underscored"])
}

func TestDecode_IntegerBeyond64BitsRejected(t *testing.T) {
	t.Parallel()

	_, err := yaml.Decode([]byte("id: 999999999999999999999999999999\n"))

	assert.IsError(t, err, yaml.ErrParse)
	assert.Equal(t, "yaml parse error: line 1: integer 999999999999999999999999999999 out of range", err.Error())
}

func TestDecode_NestedIntegerBeyond64BitsRejected(t *testing.T) {
	t.Parallel()

	_, err := yaml.Decode([]byte("nested:\n  list: [1, -99999999999999999999]\n"))

	assert.IsError(t, err, yaml.ErrParse)
	assert.Equal(t, "yaml parse error: line 2: integer -99999999999999999999 out of range", err.Error())
}

func TestDecode_HexIntegerBeyond64BitsRejected(t *testing.T) {
	t.Parallel()

	_, err := yaml.Decode([]byte("mask: 0xFFFFFFFFFFFFFFFFFF\n"))

	assert.IsError(t, err, yaml.ErrParse)
	assert.Equal(t, "yaml parse error: line 1: integer 0xFFFFFFFFFFFFFFFFFF out of range", err.Error())
}
