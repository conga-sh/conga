package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

func TestWalkShorthand(t *testing.T) {
	t.Parallel()

	verbose := &spec.Flag{Name: "verbose", Short: 'v', IsBool: true}
	count := &spec.Flag{Name: "count", Short: 'c', IsCounter: true}
	output := &spec.Flag{Name: "output", Short: 'o'}
	deep := &spec.Flag{Name: "deep", Short: 'd', IsBool: true}

	resolve := func(r rune) (*spec.Flag, int) {
		switch r {
		case 'v':
			return verbose, 0
		case 'c':
			return count, 0
		case 'o':
			return output, 0
		case 'h':
			return spec.BuiltinHelpFlag, 0
		case 'd':
			return deep, 2
		default:
			return nil, 0
		}
	}

	tests := []struct {
		name             string
		runes            string
		hasAttachedValue bool
		expected         spec.ShorthandWalk
	}{
		{
			name:  "bool",
			runes: "v",
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: verbose}},
				UnresolvedAt: -1,
			},
		},
		{
			name:  "counter repeated",
			runes: "ccc",
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: count}, {Flag: count}, {Flag: count}},
				UnresolvedAt: -1,
			},
		},
		{
			name:  "value last without attached value consumes next",
			runes: "o",
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: output, ValueStart: true}},
				ConsumesNext: true,
				UnresolvedAt: -1,
			},
		},
		{
			name:             "value last with attached value",
			runes:            "o",
			hasAttachedValue: true,
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: output, ValueStart: true}},
				UnresolvedAt: -1,
			},
		},
		{
			name:  "value mid takes the remainder",
			runes: "ovalue",
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: output, ValueStart: true}},
				UnresolvedAt: -1,
			},
		},
		{
			name:             "value mid with attached value",
			runes:            "ov",
			hasAttachedValue: true,
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: output, ValueStart: true}},
				UnresolvedAt: -1,
			},
		},
		{
			name:  "bool then value",
			runes: "vo",
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: verbose}, {Flag: output, ValueStart: true}},
				ConsumesNext: true,
				UnresolvedAt: -1,
			},
		},
		{
			name:  "unknown first rune",
			runes: "z",
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{},
				UnresolvedAt: 0,
			},
		},
		{
			name:  "unknown mid-cluster keeps the resolved prefix",
			runes: "vzo",
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: verbose}},
				UnresolvedAt: 1,
			},
		},
		{
			name:  "builtin rune is an ordinary step",
			runes: "hv",
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: spec.BuiltinHelpFlag}, {Flag: verbose}},
				UnresolvedAt: -1,
			},
		},
		{
			name:  "level attribution",
			runes: "vd",
			expected: spec.ShorthandWalk{
				Steps:        []spec.ShorthandStep{{Flag: verbose}, {Flag: deep, ImplicitLevel: 2}},
				UnresolvedAt: -1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			walk := spec.WalkShorthand([]rune(test.runes), test.hasAttachedValue, resolve)

			assert.Equal(t, test.expected, walk)
		})
	}
}
