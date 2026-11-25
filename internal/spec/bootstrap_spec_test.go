package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/errlines"
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
	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct bootstrapSchemaCLI field Dir: tag "bootstrap" requires the command or an ancestor command to implement conga.Bootstrapper`, err.Error())
	assert.Equal(t, `implement Bootstrap(ctx context.Context, b *conga.Bootstrap) error or remove the tag`, errlines.Hint(err))
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
	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct featureCLI field Dir: tag "bootstrap" cannot be combined with tag "feature"`, err.Error())
	assert.Equal(t, `the bootstrap phase runs before feature gates are resolved`, errlines.Note(err))
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
	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct typeCLI field Input: bootstrap fields must be scalars, slices of scalars, or maps of scalars`, err.Error())
	assert.Equal(t, `structs and pointers are not supported`, errlines.Note(err))
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
	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct typeCLI field Input: bootstrap fields must be scalars, slices of scalars, or maps of scalars`, err.Error())
	assert.Equal(t, `structs and pointers are not supported`, errlines.Note(err))
}

func TestBuild_BootstrapDashArgsRequiresBootstrapper(t *testing.T) {
	t.Parallel()

	type dashCLI struct {
		Trail mockDashArgs `bootstrap:"true"`
	}

	_, err := buildSchema(&dashCLI{}, "app", spec.Options{})

	assert.Error(t, err)
	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct dashCLI field Trail: tag "bootstrap" requires the command or an ancestor command to implement conga.Bootstrapper`, err.Error())
	assert.Equal(t, `implement Bootstrap(ctx context.Context, b *conga.Bootstrap) error or remove the tag`, errlines.Hint(err))
}

func TestBuild_BootstrapRawArgsRequiresBootstrapper(t *testing.T) {
	t.Parallel()

	type rawCLI struct {
		Raw mockRawArgs `bootstrap:"true"`
	}

	_, err := buildSchema(&rawCLI{}, "app", spec.Options{})

	assert.Error(t, err)
	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct rawCLI field Raw: tag "bootstrap" requires the command or an ancestor command to implement conga.Bootstrapper`, err.Error())
	assert.Equal(t, `implement Bootstrap(ctx context.Context, b *conga.Bootstrap) error or remove the tag`, errlines.Hint(err))
}
