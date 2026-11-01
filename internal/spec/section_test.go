package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

type SectionWhitespaceFlagCLI struct {
	Format mockFlag[string] `name:"format" section:" Input"`
}

type SectionWhitespaceSubcommandCLI struct {
	Child mockCmd[LeafTestCmd] `section:" Lifecycle"`
}

type SectionWhitespaceConfigFileCLI struct {
	Config mockConfigFile `section:" Config"`
}

type SectionWhitespaceBuiltinCLI struct {
	Help mockHelpCmd `section:" Meta"`
}

type SectionEmptyCLI struct {
	Format mockFlag[string] `name:"format" section:""`
}

func TestBuild_SectionWhitespaceRejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target func() any
		want   string
	}{
		{
			name:   "flag",
			target: func() any { return new(SectionWhitespaceFlagCLI) },
			want:   `struct SectionWhitespaceFlagCLI field Format: schema error: invalid struct tag: tag "section" cannot have leading or trailing whitespace`,
		},
		{
			name:   "subcommand",
			target: func() any { return new(SectionWhitespaceSubcommandCLI) },
			want:   `struct SectionWhitespaceSubcommandCLI field Child: schema error: invalid struct tag: tag "section" cannot have leading or trailing whitespace`,
		},
		{
			name:   "config file flag",
			target: func() any { return new(SectionWhitespaceConfigFileCLI) },
			want:   `struct SectionWhitespaceConfigFileCLI field Config: schema error: invalid struct tag: tag "section" cannot have leading or trailing whitespace`,
		},
		{
			name:   "built-in command",
			target: func() any { return new(SectionWhitespaceBuiltinCLI) },
			want:   `struct SectionWhitespaceBuiltinCLI field Help: schema error: invalid struct tag: tag "section" cannot have leading or trailing whitespace`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildSchema(test.target(), "app", spec.Options{})

			assert.Error(t, err)
			assert.Equal(t, test.want, err.Error())
			assert.IsError(t, err, spec.ErrInvalidStructTag)
		})
	}
}

func TestBuild_EmptySectionIsUnsectioned(t *testing.T) {
	t.Parallel()

	var cli SectionEmptyCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, "", cmdSpec.Flags[0].Section)
}
