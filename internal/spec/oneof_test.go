package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

type OneOfSchemaCLI struct {
	JSON mockFlag[bool] `name:"json" oneof:"format"`
	YAML mockFlag[bool] `name:"yaml" oneof:"format"`
}

type OneOfRequiredCLI struct {
	Format mockReqFlag[string] `name:"format" oneof:"format"`
}

type XorRequiredCLI struct {
	Format mockReqFlag[string] `name:"format" xor:"format"`
}

type DefaultRequiredCLI struct {
	Format mockReqFlag[string] `default:"json" name:"format"`
}

type DefaultOneOfCLI struct {
	Format mockFlag[string] `default:"json" name:"format" oneof:"format"`
}

type EmptyOneOfCLI struct {
	Format mockFlag[string] `name:"format" oneof:""`
}

type EmptyXorCLI struct {
	Format mockFlag[string] `name:"format" xor:""`
}

type EmptyAndCLI struct {
	Format mockFlag[string] `and:"" name:"format"`
}

type WhitespaceOneOfCLI struct {
	Format mockFlag[string] `name:"format" oneof:" format"`
}

type CounterOneOfCLI struct {
	Mode mockCounter `name:"mode" oneof:"mode"`
}

type CoexistingGroupCLI struct {
	JSON  mockFlag[bool] `and:"auth"  name:"json"    oneof:"format" xor:"transport"`
	YAML  mockFlag[bool] `name:"yaml" oneof:"format"`
	Plain mockFlag[bool] `and:"auth"  name:"plain"`
}

type OneOfAndOverlapCLI struct {
	JSON mockFlag[bool] `and:"auth" name:"json" oneof:"format"`
	YAML mockFlag[bool] `and:"auth" name:"yaml" oneof:"format"`
}

type OneOfAndPartnerCLI struct {
	JSON mockFlag[bool]   `and:"auth"  name:"json"    oneof:"format"`
	Key  mockFlag[string] `and:"auth"  name:"key"`
	YAML mockFlag[bool]   `name:"yaml" oneof:"format"`
}

type CounterGroupTagsCLI struct {
	Mode mockCounter `and:"auth" name:"mode" oneof:"format" xor:"transport"`
}

func TestBuild_OneOfGroupParsed(t *testing.T) {
	t.Parallel()

	var cli OneOfSchemaCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, 2, len(cmdSpec.Flags))
	assert.Equal(t, "format", cmdSpec.Flags[0].OneOfGroup)
	assert.Equal(t, "format", cmdSpec.Flags[1].OneOfGroup)
}

func TestBuild_OneOfOnCounterAllowed(t *testing.T) {
	t.Parallel()

	var cli CounterOneOfCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, "mode", cmdSpec.Flags[0].OneOfGroup)
}

func TestBuild_OneOfCoexistsWithXorAnd(t *testing.T) {
	t.Parallel()

	var cli CoexistingGroupCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, "format", cmdSpec.Flags[0].OneOfGroup)
	assert.Equal(t, "transport", cmdSpec.Flags[0].XorGroup)
	assert.Equal(t, "auth", cmdSpec.Flags[0].AndGroup)
}

func TestBuild_OneOfAndPartnerAllowed(t *testing.T) {
	t.Parallel()

	var cli OneOfAndPartnerCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, 3, len(cmdSpec.Flags))
	assert.Equal(t, "auth", cmdSpec.Flags[0].AndGroup)
	assert.Equal(t, "format", cmdSpec.Flags[0].OneOfGroup)
}

func TestBuild_CounterAcceptsGroupTags(t *testing.T) {
	t.Parallel()

	var cli CounterGroupTagsCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, "format", cmdSpec.Flags[0].OneOfGroup)
	assert.Equal(t, "transport", cmdSpec.Flags[0].XorGroup)
	assert.Equal(t, "auth", cmdSpec.Flags[0].AndGroup)
}

func TestBuild_OneOfSchemaErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target func() any
		want   string
	}{
		{
			name:   "oneof on required flag",
			target: func() any { return new(OneOfRequiredCLI) },
			want:   `struct OneOfRequiredCLI field Format: schema error: invalid struct tag: tag "oneof" cannot be used on required flags (required flags are individually mandatory; declare xor/oneof groups on optional flags, and use oneof when exactly one must be supplied)`,
		},
		{
			name:   "xor on required flag",
			target: func() any { return new(XorRequiredCLI) },
			want:   `struct XorRequiredCLI field Format: schema error: invalid struct tag: tag "xor" cannot be used on required flags (required flags are individually mandatory; declare xor/oneof groups on optional flags, and use oneof when exactly one must be supplied)`,
		},
		{
			name:   "default on required flag",
			target: func() any { return new(DefaultRequiredCLI) },
			want:   `struct DefaultRequiredCLI field Format: schema error: invalid struct tag: tag "default" cannot be used on required flags (the requirement must be satisfied by CLI, environment, or configuration input)`,
		},
		{
			name:   "default on oneof flag",
			target: func() any { return new(DefaultOneOfCLI) },
			want:   `struct DefaultOneOfCLI field Format: schema error: invalid struct tag: tag "default" cannot be used on oneof flags (a default would always satisfy the group; remove the default or the oneof tag)`,
		},
		{
			name:   "oneof member sharing an and group",
			target: func() any { return new(OneOfAndOverlapCLI) },
			want:   `command "app": schema error: invalid struct tag: flags --json and --yaml belong to oneof group "format" and share "and" group "auth" (selecting either flag forces the other, so the exactly-one group can never be satisfied; remove one of the group tags)`,
		},
		{
			name:   "empty oneof group",
			target: func() any { return new(EmptyOneOfCLI) },
			want:   `struct EmptyOneOfCLI field Format: schema error: invalid struct tag: tag "oneof" cannot be empty`,
		},
		{
			name:   "empty xor group",
			target: func() any { return new(EmptyXorCLI) },
			want:   `struct EmptyXorCLI field Format: schema error: invalid struct tag: tag "xor" cannot be empty`,
		},
		{
			name:   "empty and group",
			target: func() any { return new(EmptyAndCLI) },
			want:   `struct EmptyAndCLI field Format: schema error: invalid struct tag: tag "and" cannot be empty`,
		},
		{
			name:   "whitespace oneof group",
			target: func() any { return new(WhitespaceOneOfCLI) },
			want:   `struct WhitespaceOneOfCLI field Format: schema error: invalid struct tag: tag "oneof" cannot have leading or trailing whitespace`,
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
