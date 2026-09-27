package configengine

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestIsConfigCandidate(t *testing.T) {
	t.Parallel()

	t.Run("generic config stem matches case-insensitively", func(t *testing.T) {
		t.Parallel()

		assert.True(t, isConfigCandidate("config", "server-precedence"))
		assert.True(t, isConfigCandidate("CONFIG", "server-precedence"))
	})

	t.Run("app-name stem matches case-insensitively", func(t *testing.T) {
		t.Parallel()

		assert.True(t, isConfigCandidate("server-precedence", "server-precedence"))
		assert.True(t, isConfigCandidate("Server-Precedence", "server-precedence"))
		assert.True(t, isConfigCandidate("cascading-app", "Cascading-App"))
	})

	t.Run("dot-prefixed stems are rejected", func(t *testing.T) {
		t.Parallel()

		assert.False(t, isConfigCandidate(".config", "server-precedence"))
		assert.False(t, isConfigCandidate(".server-precedence", "server-precedence"))
	})

	t.Run("rc-style stems are rejected", func(t *testing.T) {
		t.Parallel()

		assert.False(t, isConfigCandidate("server-precedencerc", "server-precedence"))
		assert.False(t, isConfigCandidate(".server-precedencerc", "server-precedence"))
	})

	t.Run("unrelated stems are rejected", func(t *testing.T) {
		t.Parallel()

		assert.False(t, isConfigCandidate("notes", "server-precedence"))
		assert.False(t, isConfigCandidate("config-backup", "server-precedence"))
	})

	t.Run("empty app name only matches the generic config stem", func(t *testing.T) {
		t.Parallel()

		assert.True(t, isConfigCandidate("config", ""))
		assert.False(t, isConfigCandidate("server-precedence", ""))
	})
}

func TestSplitList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		value     string
		separator string
		want      []string
	}{
		{name: "default comma separator", value: "a.json,b.json", separator: "", want: []string{"a.json", "b.json"}},
		{name: "custom separator", value: "a.json;b.json", separator: ";", want: []string{"a.json", "b.json"}},
		{name: "whitespace trimmed and empties dropped", value: " a.json , ,b.json ", separator: ",", want: []string{"a.json", "b.json"}},
		{name: "none disables splitting", value: "  a.json,b.json  ", separator: "none", want: []string{"a.json,b.json"}},
		{name: "off disables splitting", value: "a.json", separator: "off", want: []string{"a.json"}},
		{name: "dash disables splitting", value: "a.json", separator: "-", want: []string{"a.json"}},
		{name: "blank value yields nil", value: "   ", separator: ",", want: nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, SplitList(testCase.value, testCase.separator))
		})
	}
}
