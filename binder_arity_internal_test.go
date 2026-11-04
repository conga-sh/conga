package conga

import (
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/tokenizer"
)

func TestSkipFlagValue_MatchesParserArity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		helpShort    rune
		versionShort rune
		flags        []*spec.Flag
		args         []string
	}{
		{
			name:  "long value flag",
			flags: []*spec.Flag{{Name: "output", Short: 'o'}},
			args:  []string{"--output", "value"},
		},
		{
			name:  "long attached value",
			flags: []*spec.Flag{{Name: "output", Short: 'o'}},
			args:  []string{"--output=value", "next"},
		},
		{
			name:  "short value flag",
			flags: []*spec.Flag{{Name: "output", Short: 'o'}},
			args:  []string{"-o", "value"},
		},
		{
			name: "short cluster with value last",
			flags: []*spec.Flag{
				{Name: "verbose", Short: 'v', IsBool: true},
				{Name: "output", Short: 'o'},
			},
			args: []string{"-vo", "value"},
		},
		{
			name: "short cluster with value first",
			flags: []*spec.Flag{
				{Name: "verbose", Short: 'v', IsBool: true},
				{Name: "output", Short: 'o'},
			},
			args: []string{"-ov", "value"},
		},
		{
			name:      "builtin help before value shorthand",
			helpShort: 'h',
			flags:     []*spec.Flag{{Name: "output", Short: 'o'}},
			args:      []string{"-ho", "value"},
		},
		{
			name:         "builtin version before value shorthand",
			versionShort: 'V',
			flags:        []*spec.Flag{{Name: "output", Short: 'o'}},
			args:         []string{"-Vo", "value"},
		},
		{
			name:      "builtin help short",
			helpShort: 'h',
			args:      []string{"-h", "value"},
		},
		{
			name:         "builtin version short",
			versionShort: 'V',
			args:         []string{"-V", "value"},
		},
		{
			name: "builtin long completion",
			args: []string{"--completion", "bash"},
		},
		{
			name: "builtin long help",
			args: []string{"--help", "value"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := &spec.Command{
				Name:         "tool",
				HelpShort:    test.helpShort,
				VersionShort: test.versionShort,
				Flags:        test.flags,
			}
			chain := []*spec.Command{root}
			tokens := tokenizer.Tokenize(test.args)

			_, posTokens, _, _, err := parseTokens(chain, tokens, newParseFlagIndex(chain))
			assert.NoError(t, err)

			skipped, _ := skipFlagValue(tokens, 0, newFlagIndex(root), chain)
			parserConsumedNext := len(tokens) > 1 && !slices.Contains(posTokens, tokens[1].Raw)

			assert.Equal(t, parserConsumedNext, skipped)
		})
	}
}
