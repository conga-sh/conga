package hcl_test

import (
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	hashicorphcl "github.com/hashicorp/hcl/v2"

	"github.com/conga-sh/conga/decoder/hcl"
	"github.com/conga-sh/conga/internal/errlines"
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

	assert.EqualError(t, err, `line 3, column 9: Invalid expression`)
	assert.Equal(t, "Expected the start of an expression, but found an invalid expression token.", errlines.Note(err))
	assert.Equal(t, "", errlines.Hint(err))
}

func TestDecode_DuplicateBlocks(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureDuplicateBlocks)

	assert.Equal(t, `line 5, column 1: duplicate block "server", first declared at line 1, column 1`, err.Error())
}

func TestDecode_UndefinedVariable(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureUndefinedVariable)

	assert.EqualError(t, err, `line 1, column 8: Variables not allowed`)
	assert.Equal(t, "Variables may not be used here.", errlines.Note(err))

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

	assert.Equal(t, `line 1, column 1: attribute "huge_num": number 999999999999999999999999999999 out of range`, err.Error())
}

func TestDecode_NestedIntegerBeyond64BitsRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode([]byte("list = [1, 99999999999999999999]\n"))

	assert.Equal(t, `line 1, column 1: attribute "list": number 99999999999999999999 out of range`, err.Error())
}

func TestDecode_FloatOverflowRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode([]byte("ratio = 1.5e400\n"))

	assert.Equal(t, `line 1, column 1: attribute "ratio": number 1.5e+400 out of range`, err.Error())
}

func TestDecode_LabeledBlockRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureLabeledBlock)

	assert.Equal(t, `line 1, column 1: block "server" has labels ["web"]`, err.Error())
	assert.Equal(t, "labeled blocks are not supported", errlines.Note(err))
	assert.Equal(t, "use an unlabeled block", errlines.Hint(err))
}

func TestDecode_AttributeThenBlockRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureAttributeThenBlock)

	assert.Equal(t, `line 3, column 1: block "server" conflicts with the attribute of the same name at line 1, column 1`, err.Error())
}

func TestDecode_BlockThenAttributeRejected(t *testing.T) {
	t.Parallel()

	_, err := hcl.Decode(fixtureBlockThenAttribute)

	assert.Equal(t, `line 1, column 1: block "server" conflicts with the attribute of the same name at line 5, column 1`, err.Error())
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
