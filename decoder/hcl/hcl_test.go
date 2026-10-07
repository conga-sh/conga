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

func TestDecode_ValidFlat(t *testing.T) {
	t.Parallel()

	result, err := hcl.Decode(fixtureValidFlat)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
	assert.Equal(t, any(int64(8080)), result["port"])
	assert.Equal(t, any(true), result["debug"])
}

func TestDecode_ValidNested(t *testing.T) {
	t.Parallel()

	result, err := hcl.Decode(fixtureValidNested)

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

func TestDecode_InvalidSyntax(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureInvalidSyntax)

	assert.IsError(t, err, hcl.ErrParse)
	assert.Contains(t, err.Error(), "hcl parse error")
}

func TestDecode_DuplicateBlocks(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureDuplicateBlocks)

	assert.IsError(t, err, hcl.ErrParse)
	assert.Equal(t, `hcl parse error: duplicate block "server"`, err.Error())
}

func TestDecode_UndefinedVariable(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureUndefinedVariable)

	assert.IsError(t, err, hcl.ErrParse)
	assert.Contains(t, err.Error(), "Variables not allowed")

	var diagnostics hashicorphcl.Diagnostics
	assert.True(t, errors.As(err, &diagnostics))
}

func TestDecode_NullAndFloatValues(t *testing.T) {
	t.Parallel()

	result, err := hcl.Decode(fixtureNullAndFloat)

	assert.NoError(t, err)
	assert.Equal(t, any("testapp"), result["name"])
	assert.Equal(t, nil, result["missing"])
	assert.Equal(t, any(0.25), result["ratio"])
}

func TestDecode_LargeNumbers(t *testing.T) {
	t.Parallel()

	result, err := hcl.Decode(fixtureLargeNumber)

	assert.NoError(t, err)
	assert.Equal(t, any(uint64(18446744073709551615)), result["large_uint"])
	assert.Equal(t, any("999999999999999999999999999999"), result["huge_num"])
}

func TestDecode_EmptyValid(t *testing.T) {
	t.Parallel()

	result, err := hcl.Decode([]byte(""))

	assert.NoError(t, err)
	assert.Zero(t, result)
}
