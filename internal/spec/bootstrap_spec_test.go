package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

type bootstrapSchemaCLI struct {
	Dir mockFlag[string] `bootstrap:"true"`
}

type recursiveBootstrapSlice []recursiveBootstrapSlice

func TestBuild_BootstrapFieldAccepted(t *testing.T) {
	t.Parallel()

	cmd, err := buildSchema(&bootstrapSchemaCLI{}, "app", spec.Options{
		HasBootstrapper: true,
	})

	assert.NoError(t, err)
	assert.True(t, cmd.Flags[0].Bootstrap)
}

func TestBuild_BootstrapRequiresBootstrapper(t *testing.T) {
	t.Parallel()

	_, err := buildSchema(&bootstrapSchemaCLI{}, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `struct bootstrapSchemaCLI field Dir: schema error: tag "bootstrap" requires the command or an ancestor command to implement conga.Bootstrapper; implement Bootstrap(ctx context.Context, b *conga.Bootstrap) error or remove the tag`, err.Error())
}

func TestBuild_BootstrapRejectsFeatureTag(t *testing.T) {
	t.Parallel()

	type featureCLI struct {
		Dir mockFlag[string] `bootstrap:"true" feature:"strict"`
	}

	_, err := buildSchema(&featureCLI{}, "app", spec.Options{
		HasBootstrapper: true,
	})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct featureCLI field Dir: schema error: invalid struct tag: tag "bootstrap" cannot be combined with tag "feature" (the bootstrap phase runs before feature gates are resolved)`, err.Error())
}

func TestBuild_BootstrapRejectsStructType(t *testing.T) {
	t.Parallel()

	type structValue struct{}

	type typeCLI struct {
		Input mockFlag[structValue] `bootstrap:"true"`
	}

	_, err := buildSchema(&typeCLI{}, "app", spec.Options{
		HasBootstrapper: true,
	})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct typeCLI field Input: schema error: invalid struct tag: bootstrap fields must be scalars, slices of scalars, or maps of scalars (structs and pointers are not supported)`, err.Error())
}

func TestBuild_BootstrapRejectsRecursiveSliceType(t *testing.T) {
	t.Parallel()

	type typeCLI struct {
		Input mockFlag[recursiveBootstrapSlice] `bootstrap:"true"`
	}

	_, err := buildSchema(&typeCLI{}, "app", spec.Options{
		HasBootstrapper: true,
	})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct typeCLI field Input: schema error: invalid struct tag: bootstrap fields must be scalars, slices of scalars, or maps of scalars (structs and pointers are not supported)`, err.Error())
}

func TestBuild_BootstrapDashArgsRequiresBootstrapper(t *testing.T) {
	t.Parallel()

	type dashCLI struct {
		Trail mockDashArgs `bootstrap:"true"`
	}

	_, err := buildSchema(&dashCLI{}, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `struct dashCLI field Trail: schema error: tag "bootstrap" requires the command or an ancestor command to implement conga.Bootstrapper; implement Bootstrap(ctx context.Context, b *conga.Bootstrap) error or remove the tag`, err.Error())
}

func TestBuild_BootstrapRawArgsRequiresBootstrapper(t *testing.T) {
	t.Parallel()

	type rawCLI struct {
		Raw mockRawArgs `bootstrap:"true"`
	}

	_, err := buildSchema(&rawCLI{}, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `struct rawCLI field Raw: schema error: tag "bootstrap" requires the command or an ancestor command to implement conga.Bootstrapper; implement Bootstrap(ctx context.Context, b *conga.Bootstrap) error or remove the tag`, err.Error())
}
