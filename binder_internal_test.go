package conga

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/tokenizer"
)

func TestHasBuiltinRequest_NoCLILongShadowDoesNotMaskBuiltin(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "tool",
		Flags: []*spec.Flag{
			{Name: "help", NoCLI: true},
		},
	}
	chain := []*spec.Command{root}

	found := hasBuiltinRequest(tokenizer.Tokenize([]string{"--help"}), chain, newFlagIndex(root))

	assert.True(t, found)
}

func TestHasBuiltinRequest_NoCLIShortShadowDoesNotMaskBuiltin(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name:      "tool",
		HelpShort: 'h',
		Flags: []*spec.Flag{
			{Name: "help", NoCLI: true, Short: 'h', IsBool: true},
		},
	}
	chain := []*spec.Command{root}

	found := hasBuiltinRequest(tokenizer.Tokenize([]string{"-h"}), chain, newFlagIndex(root))

	assert.True(t, found)
}
