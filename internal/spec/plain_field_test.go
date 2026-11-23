package spec_test

import (
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

type PlainTaggedCLI struct {
	Extra PlainStruct `name:"extra"`
}

type PlainUntaggedCLI struct {
	Extra PlainStruct
}

type PlainForeignTagCLI struct {
	Extra PlainStruct `bogus:"x"`
}

type PlainRequiredCLI struct {
	Host string `required:"true"`
}

type PlainReservedCLI struct {
	Extra PlainStruct `~explicit:"/etc/app.yaml"`
}

type PlainStruct struct {
	Value string
}

type PlainDomainTagCLI struct {
	Log plainTagProvider `existing:"true"`
}

type plainTagProvider struct{}

func (plainTagProvider) TagSpecs() []spec.TagSpec {
	return []spec.TagSpec{{Name: "existing", Kind: spec.TagKindBool}}
}

func TestBuild_PlainFieldTagRejected(t *testing.T) {
	t.Parallel()

	var cli PlainTaggedCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.Equal(t, `struct PlainTaggedCLI field Extra: tag "name" is not valid on a plain field (hint: declare a flag, argument, command, config-file, features, or trailing-args container)`, err.Error())
	assert.True(t, spec.IsSchemaError(err))
}

func TestBuild_PlainFieldWithoutTagsIgnored(t *testing.T) {
	t.Parallel()

	var cli PlainUntaggedCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Zero(t, cmdSpec.Flags)
	assert.Zero(t, cmdSpec.Subcommands)
}

func TestBuild_PlainFieldForeignTagIgnored(t *testing.T) {
	t.Parallel()

	var cli PlainForeignTagCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Zero(t, cmdSpec.Flags)
}

func TestBuild_PlainFieldAllowedTagAccepted(t *testing.T) {
	t.Parallel()

	var cli PlainTaggedCLI

	_, err := buildSchema(&cli, "app", spec.Options{AllowedTags: []string{"name"}})

	assert.NoError(t, err)
}

func TestBuild_PlainFieldDomainTagRejected(t *testing.T) {
	t.Parallel()

	var cli PlainDomainTagCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.Equal(t, `struct PlainDomainTagCLI field Log: tag "existing" is not valid on a plain field (hint: declare a flag, argument, command, config-file, features, or trailing-args container)`, err.Error())
	assert.True(t, spec.IsSchemaError(err))
}

func TestBuild_PlainFieldDomainTagAllowedByAllowTags(t *testing.T) {
	t.Parallel()

	var cli PlainDomainTagCLI

	_, err := buildSchema(&cli, "app", spec.Options{AllowedTags: []string{"existing"}})

	assert.NoError(t, err)
}

func TestValidateNonContainerFieldTags_AmbiguousAllowListSkips(t *testing.T) {
	t.Parallel()

	err := spec.ValidateNonContainerFieldTags(reflect.StructTag(`name:"extra"`), nil, spec.ValidationOptions{SkipUnknownTagKeyCheck: true})

	assert.NoError(t, err)
}

func TestValidateNonContainerFieldTags_MalformedTagRejected(t *testing.T) {
	t.Parallel()

	err := spec.ValidateNonContainerFieldTags(reflect.StructTag("name"), nil, spec.ValidationOptions{})

	assert.Error(t, err)
	assert.Equal(t, `malformed struct tag "name"`, err.Error())
	assert.True(t, spec.IsSchemaError(err))
}

func TestValidateNonContainerFieldTags_DomainTagRejected(t *testing.T) {
	t.Parallel()

	err := spec.ValidateNonContainerFieldTags(reflect.StructTag(`existing:"true"`), []string{"existing"}, spec.ValidationOptions{})

	assert.Error(t, err)
	assert.Equal(t, `tag "existing" is not valid on a plain field (hint: declare a flag, argument, command, config-file, features, or trailing-args container)`, err.Error())
	assert.True(t, spec.IsSchemaError(err))
}

func TestValidateNonContainerFieldTags_DomainTagAllowedByAllowTags(t *testing.T) {
	t.Parallel()

	err := spec.ValidateNonContainerFieldTags(reflect.StructTag(`existing:"true"`), []string{"existing"}, spec.ValidationOptions{AllowedTags: []string{"existing"}})

	assert.NoError(t, err)
}

func TestBuild_PlainFieldRequiredTagRejected(t *testing.T) {
	t.Parallel()

	var cli PlainRequiredCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.Equal(t, `struct PlainRequiredCLI field Host: tag "required" is not supported (hint: use conga.RequiredFlag[T] or conga.RequiredArg[T] instead)`, err.Error())
	assert.True(t, spec.IsSchemaError(err))
}

func TestBuild_PlainFieldRequiredTagIgnoresAllowTags(t *testing.T) {
	t.Parallel()

	var cli PlainRequiredCLI

	_, err := buildSchema(&cli, "app", spec.Options{AllowedTags: []string{"required"}})

	assert.Error(t, err)
	assert.Equal(t, `struct PlainRequiredCLI field Host: tag "required" is not supported (hint: use conga.RequiredFlag[T] or conga.RequiredArg[T] instead)`, err.Error())
	assert.True(t, spec.IsSchemaError(err))
}

func TestBuild_PlainFieldReservedTagRejected(t *testing.T) {
	t.Parallel()

	var cli PlainReservedCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.Equal(t, `struct PlainReservedCLI field Extra: struct tag "~explicit" is reserved for CONGA's app-level configuration settings (hint: declare it via app.ConfigOptions(...) instead)`, err.Error())
	assert.True(t, spec.IsSchemaError(err))
}
