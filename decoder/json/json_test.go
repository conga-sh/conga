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

	//go:embed testdata/numbers.json
	fixtureNumbers []byte

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

func TestDecode_NilInput(t *testing.T) {
	t.Parallel()

	_, err := json.Decode(nil)

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

func TestDecode_NumbersKeepIntegerPrecision(t *testing.T) {
	t.Parallel()

	result, err := json.Decode(fixtureNumbers)

	assert.NoError(t, err)
	assert.Equal(t, any(int64(1000000)), result["million"])
	assert.Equal(t, any(int64(9007199254740993)), result["exact"])
	assert.Equal(t, any(uint64(18446744073709551615)), result["max_uint64"])
	assert.Equal(t, any(int64(-9223372036854775808)), result["min_int64"])
	assert.Equal(t, any(1.5), result["ratio"])
	assert.Equal(t, any(map[string]any{"list": []any{int64(1), 2.0, int64(-3)}}), result["nested"])
}

func TestDecode_IntegerAboveUint64Rejected(t *testing.T) {
	t.Parallel()

	_, err := json.Decode([]byte(`{"id": 18446744073709551616}`))

	assert.IsError(t, err, json.ErrParse)
	assert.Equal(t, `json parse error: /id: integer 18446744073709551616 out of range`, err.Error())
}

func TestDecode_IntegerBelowInt64Rejected(t *testing.T) {
	t.Parallel()

	_, err := json.Decode([]byte(`{"id": -9223372036854775809}`))

	assert.IsError(t, err, json.ErrParse)
	assert.Equal(t, `json parse error: /id: integer -9223372036854775809 out of range`, err.Error())
}

func TestDecode_NestedIntegerOutOfRangeRejected(t *testing.T) {
	t.Parallel()

	_, err := json.Decode([]byte(`{"a": {"b": [1, 99999999999999999999]}}`))

	assert.IsError(t, err, json.ErrParse)
	assert.Equal(t, `json parse error: /a/b/1: integer 99999999999999999999 out of range`, err.Error())
}

func TestDecode_FloatOverflowRejected(t *testing.T) {
	t.Parallel()

	_, err := json.Decode([]byte(`{"ratio": 1e400}`))

	assert.IsError(t, err, json.ErrParse)
	assert.Equal(t, `json parse error: /ratio: number 1e400 out of range`, err.Error())
}
