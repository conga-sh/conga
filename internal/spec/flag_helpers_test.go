package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

func TestIsBuiltinFlag(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		flag *spec.Flag
		want bool
	}{
		{name: "help", flag: spec.BuiltinHelpFlag, want: true},
		{name: "version", flag: spec.BuiltinVersionFlag, want: true},
		{name: "why", flag: spec.BuiltinWhyFlag, want: true},
		{name: "completion", flag: spec.BuiltinCompletionFlag, want: true},
		{name: "user", flag: &spec.Flag{Name: "help"}, want: false},
		{name: "nil", flag: nil, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, spec.IsBuiltinFlag(test.flag))
		})
	}
}

func TestIsBuiltinShortFlag(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		flag *spec.Flag
		want bool
	}{
		{name: "help", flag: spec.BuiltinHelpFlag, want: true},
		{name: "version", flag: spec.BuiltinVersionFlag, want: true},
		{name: "why", flag: spec.BuiltinWhyFlag, want: false},
		{name: "completion", flag: spec.BuiltinCompletionFlag, want: false},
		{name: "user", flag: &spec.Flag{Name: "help"}, want: false},
		{name: "nil", flag: nil, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, spec.IsBuiltinShortFlag(test.flag))
		})
	}
}

func TestFindFlagInChain_NoCLIHelpShadow(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "tool",
		Flags: []*spec.Flag{
			{Name: "help", NoCLI: true},
		},
	}

	flagSpec := spec.FindFlagInChain([]*spec.Command{root}, "help")

	assert.Equal(t, spec.BuiltinHelpFlag, flagSpec)
}

func TestFindShortFlagInChain_NoCLIHelpShadow(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name:      "tool",
		HelpShort: 'h',
		Flags: []*spec.Flag{
			{Name: "help", NoCLI: true, Short: 'h', IsBool: true},
		},
	}

	flagSpec := spec.FindShortFlagInChain([]*spec.Command{root}, 'h')

	assert.Equal(t, spec.BuiltinHelpFlag, flagSpec)
}

func TestFlagTakesValue(t *testing.T) {
	t.Parallel()

	t.Run("boolean", func(t *testing.T) {
		t.Parallel()

		assert.False(t, (&spec.Flag{IsBool: true}).TakesValue())
	})

	t.Run("counter", func(t *testing.T) {
		t.Parallel()

		assert.False(t, (&spec.Flag{IsCounter: true}).TakesValue())
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		assert.True(t, (&spec.Flag{Name: "name"}).TakesValue())
	})

	t.Run("slice", func(t *testing.T) {
		t.Parallel()

		assert.True(t, (&spec.Flag{IsSlice: true}).TakesValue())
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		var flag *spec.Flag

		assert.False(t, flag.TakesValue())
	})
}

func TestResolveNegatedFlag(t *testing.T) {
	t.Parallel()

	negatable := &spec.Flag{Name: "color", IsBool: true, IsNegatable: true}
	nonNegatable := &spec.Flag{Name: "file"}
	nonBoolean := &spec.Flag{Name: "list", IsNegatable: true}
	flags := map[string]*spec.Flag{
		"color": negatable,
		"file":  nonNegatable,
		"list":  nonBoolean,
	}
	lookup := func(name string) *spec.Flag {
		return flags[name]
	}

	t.Run("resolves negatable boolean", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, negatable, spec.ResolveNegatedFlag("no-color", lookup))
	})

	t.Run("ignores name without prefix", func(t *testing.T) {
		t.Parallel()

		assert.Zero(t, spec.ResolveNegatedFlag("color", lookup))
	})

	t.Run("ignores missing positive flag", func(t *testing.T) {
		t.Parallel()

		assert.Zero(t, spec.ResolveNegatedFlag("no-missing", lookup))
	})

	t.Run("ignores non-negatable flag", func(t *testing.T) {
		t.Parallel()

		assert.Zero(t, spec.ResolveNegatedFlag("no-file", lookup))
	})

	t.Run("ignores non-boolean flag", func(t *testing.T) {
		t.Parallel()

		assert.Zero(t, spec.ResolveNegatedFlag("no-list", lookup))
	})
}
