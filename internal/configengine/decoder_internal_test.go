package configengine

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/decoder/json"
)

func TestRegistry_DecoderNormalization(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.Register("CUSTOMEXT", json.RawDecoder)

	assert.Equal(t, json.RawDecoder, registry.Decoder(".customext"))
	assert.Equal(t, json.RawDecoder, registry.Decoder("CUSTOMEXT"))
	assert.Equal(t, json.RawDecoder, registry.Decoder("*.customext"))
	assert.Equal(t, nil, registry.Decoder(".neverregisteredext"))
}

func TestRegistry_Extensions(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.Register("toml", json.RawDecoder)
	registry.Register(".json", json.RawDecoder)

	extensions := registry.Extensions()

	assert.Equal(t, []string{".json", ".toml"}, extensions)
}

func TestRegistry_NilReceiver(t *testing.T) {
	t.Parallel()

	var registry *Registry

	assert.Zero(t, registry.Decoder(".json"))
	assert.Zero(t, registry.Extensions())

	registry.Register(".json", json.RawDecoder)

	assert.Zero(t, registry.Decoder(".json"))
}

func TestNormalizeExtension(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{input: ".json", want: ".json"},
		{input: "json", want: ".json"},
		{input: "*.yaml", want: ".yaml"},
		{input: "*yaml", want: ".yaml"},
		{input: "  .TOML  ", want: ".toml"},
		{input: "  TOML  ", want: ".toml"},
		{input: "  *.HCL  ", want: ".hcl"},
	}

	for _, testCase := range cases {
		t.Run(testCase.input, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, normalizeExtension(testCase.input))
		})
	}
}
