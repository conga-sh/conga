package hcl_test

import (
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	hashicorphcl "github.com/hashicorp/hcl/v2"

	"github.com/conga-sh/conga/decoder/hcl"
)

var (
	//go:embed testdata/duplicate_blocks.hcl
	fixtureDuplicateBlocks []byte

	//go:embed testdata/invalid_syntax.hcl
	fixtureInvalidSyntax []byte

	//go:embed testdata/large_number.hcl
	fixtureLargeNumber []byte

	//go:embed testdata/null_and_float.hcl
	fixtureNullAndFloat []byte

	//go:embed testdata/undefined_variable.hcl
	fixtureUndefinedVariable []byte

	//go:embed testdata/valid_flat.hcl
	fixtureValidFlat []byte

	//go:embed testdata/valid_nested.hcl
	fixtureValidNested []byte
)

func TestRawDecoder_ValidFlat(t *testing.T) {
	t.Parallel()

	result, err := hcl.RawDecoder(fixtureValidFlat)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
	assert.Equal(t, any(int64(8080)), result["port"])
	assert.Equal(t, any(true), result["debug"])
}

func TestRawDecoder_ValidNested(t *testing.T) {
	t.Parallel()

	result, err := hcl.RawDecoder(fixtureValidNested)

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
	assert.Equal(t, any(int64(30)), config["timeout"])
	assert.Equal(t, any(int64(3)), config["retries"])
}

func TestRawDecoder_InvalidSyntax(t *testing.T) {
	t.Parallel()

	_, err := hcl.RawDecoder(fixtureInvalidSyntax)

	assert.Error(t, err)
	assert.IsError(t, err, hcl.ErrHCLParse)
	assert.Contains(t, err.Error(), "hcl parse error")
}

func TestRawDecoder_DuplicateBlocks(t *testing.T) {
	t.Parallel()

	_, err := hcl.RawDecoder(fixtureDuplicateBlocks)

	assert.Error(t, err)
	assert.IsError(t, err, hcl.ErrHCLParse)
	assert.Equal(t, `hcl parse error: duplicate block "server"`, err.Error())
}

func TestRawDecoder_UndefinedVariable(t *testing.T) {
	t.Parallel()

	_, err := hcl.RawDecoder(fixtureUndefinedVariable)

	assert.Error(t, err)
	assert.IsError(t, err, hcl.ErrHCLParse)
	assert.Contains(t, err.Error(), "Variables not allowed")

	var diagnostics hashicorphcl.Diagnostics
	assert.True(t, errors.As(err, &diagnostics))
}

func TestRawDecoder_NullAndFloatValues(t *testing.T) {
	t.Parallel()

	result, err := hcl.RawDecoder(fixtureNullAndFloat)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
	assert.Equal(t, nil, result["missing"])
	assert.Equal(t, any(0.25), result["ratio"])
}

func TestRawDecoder_LargeNumbers(t *testing.T) {
	t.Parallel()

	result, err := hcl.RawDecoder(fixtureLargeNumber)

	assert.NoError(t, err)
	assert.Equal(t, any(uint64(18446744073709551615)), result["large_uint"])
	assert.Equal(t, any("999999999999999999999999999999"), result["huge_num"])
}
