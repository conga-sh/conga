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
	//go:embed testdata/attribute_then_block.hcl
	fixtureAttributeThenBlock []byte

	//go:embed testdata/block_then_attribute.hcl
	fixtureBlockThenAttribute []byte

	//go:embed testdata/duplicate_blocks.hcl
	fixtureDuplicateBlocks []byte

	//go:embed testdata/invalid_syntax.hcl
	fixtureInvalidSyntax []byte

	//go:embed testdata/labeled_block.hcl
	fixtureLabeledBlock []byte

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
	assert.Equal(t, `hcl parse error: config.hcl:5,1-7: duplicate block "server", first declared at config.hcl:1,1-7`, err.Error())
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
}

func TestDecode_IntegerBeyond64BitsRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode([]byte("huge_num = 999999999999999999999999999999\n"))

	assert.IsError(t, err, hcl.ErrParse)
	assert.Equal(t, `hcl parse error: config.hcl:1,1-42: attribute "huge_num": number 999999999999999999999999999999 out of range`, err.Error())
}

func TestDecode_NestedIntegerBeyond64BitsRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode([]byte("list = [1, 99999999999999999999]\n"))

	assert.IsError(t, err, hcl.ErrParse)
	assert.Equal(t, `hcl parse error: config.hcl:1,1-33: attribute "list": number 99999999999999999999 out of range`, err.Error())
}

func TestDecode_FloatOverflowRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode([]byte("ratio = 1.5e400\n"))

	assert.IsError(t, err, hcl.ErrParse)
	assert.Equal(t, `hcl parse error: config.hcl:1,1-16: attribute "ratio": number 1.5e+400 out of range`, err.Error())
}

func TestDecode_LabeledBlockRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureLabeledBlock)

	assert.IsError(t, err, hcl.ErrParse)
	assert.Equal(t, `hcl parse error: config.hcl:1,1-13: block "server" has labels ["web"]; labeled blocks are not supported, use an unlabeled block`, err.Error())
}

func TestDecode_AttributeThenBlockRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureAttributeThenBlock)

	assert.IsError(t, err, hcl.ErrParse)
	assert.Equal(t, `hcl parse error: config.hcl:3,1-7: block "server" conflicts with the attribute of the same name at config.hcl:1,1-11`, err.Error())
}

func TestDecode_BlockThenAttributeRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureBlockThenAttribute)

	assert.IsError(t, err, hcl.ErrParse)
	assert.Equal(t, `hcl parse error: config.hcl:1,1-7: block "server" conflicts with the attribute of the same name at config.hcl:5,1-11`, err.Error())
}

func TestDecode_EmptyValid(t *testing.T) {
	t.Parallel()

	result, err := hcl.Decode([]byte(""))

	assert.NoError(t, err)
	assert.Zero(t, result)
}

func TestDecode_NilInput(t *testing.T) {
	t.Parallel()

	result, err := hcl.Decode(nil)

	assert.NoError(t, err)
	assert.Zero(t, result)
}
