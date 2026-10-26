package tokenizer_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/tokenizer"
)

func TestTokenize_POSIXCompliance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected []tokenizer.Token
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: []tokenizer.Token{},
		},
		{
			name: "single dash is positional (stdin)",
			args: []string{"cat", "-"},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypePositional, Raw: "cat", Index: 0},
				{Type: tokenizer.TokenTypePositional, Raw: "-", Index: 1},
			},
		},
		{
			name: "double dash terminator disables flag parsing",
			args: []string{"rm", "--", "-rf", "--force", "-"},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypePositional, Raw: "rm", Index: 0},
				{Type: tokenizer.TokenTypeTerminator, Raw: "--", Index: 1},
				{Type: tokenizer.TokenTypePositional, Raw: "-rf", Index: 2},
				{Type: tokenizer.TokenTypePositional, Raw: "--force", Index: 3},
				{Type: tokenizer.TokenTypePositional, Raw: "-", Index: 4},
			},
		},
		{
			name: "long flags without values",
			args: []string{"--verbose", "--dry-run"},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypeLongFlag, Raw: "--verbose", Name: "verbose", Value: "", HasAttachedValue: false, Index: 0},
				{Type: tokenizer.TokenTypeLongFlag, Raw: "--dry-run", Name: "dry-run", Value: "", HasAttachedValue: false, Index: 1},
			},
		},
		{
			name: "long flag with attached value",
			args: []string{"--port=8080", "--host=localhost:9090", "--msg=hello world"},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypeLongFlag, Raw: "--port=8080", Name: "port", Value: "8080", HasAttachedValue: true, Index: 0},
				{Type: tokenizer.TokenTypeLongFlag, Raw: "--host=localhost:9090", Name: "host", Value: "localhost:9090", HasAttachedValue: true, Index: 1},
				{Type: tokenizer.TokenTypeLongFlag, Raw: "--msg=hello world", Name: "msg", Value: "hello world", HasAttachedValue: true, Index: 2},
			},
		},
		{
			name: "long flag with empty attached value",
			args: []string{"--empty="},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypeLongFlag, Raw: "--empty=", Name: "empty", Value: "", HasAttachedValue: true, Index: 0},
			},
		},
		{
			name: "single short flags",
			args: []string{"-v", "-h"},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypeShortFlag, Raw: "-v", Name: "v", ShortRunes: []rune{'v'}, Index: 0},
				{Type: tokenizer.TokenTypeShortFlag, Raw: "-h", Name: "h", ShortRunes: []rune{'h'}, Index: 1},
			},
		},
		{
			name: "short flag cluster / bundle",
			args: []string{"-xzvf"},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypeShortFlag, Raw: "-xzvf", Name: "x", ShortRunes: []rune{'x', 'z', 'v', 'f'}, Index: 0},
			},
		},
		{
			name: "short flag with attached value using equals",
			args: []string{"-p=8080"},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypeShortFlag, Raw: "-p=8080", Name: "p", Value: "8080", HasAttachedValue: true, ShortRunes: []rune{'p'}, Index: 0},
			},
		},
		{
			name: "interleaved positional and flags",
			args: []string{"build", "-v", "main.go", "--output=bin", "extra.go"},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypePositional, Raw: "build", Index: 0},
				{Type: tokenizer.TokenTypeShortFlag, Raw: "-v", Name: "v", ShortRunes: []rune{'v'}, Index: 1},
				{Type: tokenizer.TokenTypePositional, Raw: "main.go", Index: 2},
				{Type: tokenizer.TokenTypeLongFlag, Raw: "--output=bin", Name: "output", Value: "bin", HasAttachedValue: true, Index: 3},
				{Type: tokenizer.TokenTypePositional, Raw: "extra.go", Index: 4},
			},
		},
		{
			name: "unicode flags and arguments",
			args: []string{"--🚀=fire", "-🔥", "世界"},
			expected: []tokenizer.Token{
				{Type: tokenizer.TokenTypeLongFlag, Raw: "--🚀=fire", Name: "🚀", Value: "fire", HasAttachedValue: true, Index: 0},
				{Type: tokenizer.TokenTypeShortFlag, Raw: "-🔥", Name: "🔥", ShortRunes: []rune{'🔥'}, Index: 1},
				{Type: tokenizer.TokenTypePositional, Raw: "世界", Index: 2},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, tokenizer.Tokenize(testCase.args))
		})
	}
}

func TestToken_IsNegativeNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw      string
		expected bool
	}{
		{"-10", true},
		{"-0", true},
		{"-3.14", true},
		{"-.5", true},
		{"-0.5", true},
		{"-5.", true},
		{"-1e-3", true},
		{"-1e5", true},
		{"-0x10", true},
		{"-.", false},
		{"-abc", false},
		{"-v", false},
		{"-p=8080", false},
		{"--10", false},
		{"-", false},
		{"10", false},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			tok := tokenizer.Token{Raw: tt.raw}
			assert.Equal(t, tt.expected, tok.IsNegativeNumber())
		})
	}
}

func TestScanner(t *testing.T) {
	t.Parallel()

	tokens := tokenizer.Tokenize([]string{"app", "--port=8080", "-v", "arg1"})

	scanner := tokenizer.NewScanner(tokens)
	assert.True(t, scanner.HasNext())

	// Next 1
	tok, ok := scanner.Next()
	assert.True(t, ok)
	assert.Equal(t, "app", tok.Raw)

	// Next 2
	tok, ok = scanner.Next()
	assert.True(t, ok)
	assert.Equal(t, "port", tok.Name)

	// Next 3
	tok, ok = scanner.Next()
	assert.True(t, ok)
	assert.Equal(t, "-v", tok.Raw)

	// Next 4
	tok, ok = scanner.Next()
	assert.True(t, ok)
	assert.Equal(t, "arg1", tok.Raw)

	// Exhaust scanner
	assert.False(t, scanner.HasNext())
	_, ok = scanner.Next()
	assert.False(t, ok)
}

func TestStringers(t *testing.T) {
	t.Parallel()

	// Test TokenType String
	assert.Equal(t, "Unknown", tokenizer.TokenTypeUnknown.String())
	assert.Equal(t, "Positional", tokenizer.TokenTypePositional.String())
	assert.Equal(t, "LongFlag", tokenizer.TokenTypeLongFlag.String())
	assert.Equal(t, "ShortFlag", tokenizer.TokenTypeShortFlag.String())
	assert.Equal(t, "Terminator", tokenizer.TokenTypeTerminator.String())
	assert.Equal(t, "Unknown", tokenizer.TokenType(999).String())

	// Test Token String representations
	tokens := []tokenizer.Token{
		{Type: tokenizer.TokenTypeLongFlag, Name: "port", Value: "8080", HasAttachedValue: true},
		{Type: tokenizer.TokenTypeLongFlag, Name: "verbose"},
		{Type: tokenizer.TokenTypeShortFlag, Name: "x", ShortRunes: []rune{'x', 'z', 'v', 'f'}},
		{Type: tokenizer.TokenTypeShortFlag, Name: "p", Value: "8080", HasAttachedValue: true},
		{Type: tokenizer.TokenTypeShortFlag, Name: "v"},
		{Type: tokenizer.TokenTypePositional, Raw: "build"},
		{Type: tokenizer.TokenTypeTerminator},
		{Type: tokenizer.TokenTypeUnknown, Raw: "???"},
	}

	expected := []string{
		`LongFlag(--port="8080")`,
		"LongFlag(--verbose)",
		"ShortBundle(-xzvf)",
		`ShortFlag(-p="8080")`,
		"ShortFlag(-v)",
		`Positional("build")`,
		"Terminator(--)",
		`Unknown("???")`,
	}

	for index, tok := range tokens {
		assert.Equal(t, expected[index], tok.String())
	}
}
