package conga_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
)

type GroupXorEnvCLI struct {
	noopRunner

	Gzip conga.OptionalFlag[bool] `env:"CONGA_TEST_GROUP_GZIP" name:"gzip" xor:"compression"`
	Zstd conga.OptionalFlag[bool] `env:"CONGA_TEST_GROUP_ZSTD" name:"zstd" xor:"compression"`
}

type GroupOneOfConfigCLI struct {
	noopRunner

	ToFile conga.OptionalFlag[string] `env:"CONGA_TEST_GROUP_TO_FILE" name:"to-file" oneof:"target"`
	ToURL  conga.OptionalFlag[string] `name:"to-url"                  oneof:"target"`
}

type GroupAndCLI struct {
	noopRunner

	User     conga.OptionalFlag[string] `and:"auth" env:"CONGA_TEST_GROUP_USER" name:"user" short:"u"`
	Password conga.OptionalFlag[string] `and:"auth" env:"CONGA_TEST_GROUP_PASS" name:"password"`
	Host     conga.OptionalFlag[string] `and:"auth" name:"host"`
}

type GroupSpellingCLI struct {
	noopRunner

	JSON conga.OptionalFlag[bool] `aliases:"as-json" name:"json" short:"j" xor:"format"`
	YAML conga.OptionalFlag[bool] `name:"yaml"       short:"y" xor:"format"`
}

func TestGroupErrors_XorBothFromEnv(t *testing.T) {
	t.Setenv("CONGA_TEST_GROUP_GZIP", "true")
	t.Setenv("CONGA_TEST_GROUP_ZSTD", "true")

	app := conga.New[GroupXorEnvCLI]().Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --gzip (from environment variable CONGA_TEST_GROUP_GZIP) and --zstd (from environment variable CONGA_TEST_GROUP_ZSTD) are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrConfig)
	assert.False(t, conga.IsUsageError(err))
}

func TestGroupErrors_XorBothFromConfig(t *testing.T) {
	t.Parallel()

	app := conga.New[GroupXorEnvCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"gzip": true, "zstd": true}`))).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `flags --gzip (from config key "gzip") and --zstd (from config key "zstd") are mutually exclusive`, err.Error())
	assert.IsError(t, err, conga.ErrConfig)
	assert.False(t, conga.IsUsageError(err))
}

func TestGroupErrors_OneOfEnvAndConfig(t *testing.T) {
	t.Setenv("CONGA_TEST_GROUP_TO_FILE", "out.json")

	app := conga.New[GroupOneOfConfigCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"to-url": "https://example.com"}`))).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `flags --to-file (from environment variable CONGA_TEST_GROUP_TO_FILE) and --to-url (from config key "to-url") are mutually exclusive`, err.Error())
	assert.IsError(t, err, conga.ErrConfig)
	assert.False(t, conga.IsUsageError(err))
}

func TestGroupErrors_OneOfConfigAndCLI(t *testing.T) {
	t.Parallel()

	app := conga.New[GroupOneOfConfigCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"to-url": "https://example.com"}`))).
		Args([]string{"--to-file", "out.json"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `flags --to-file and --to-url (from config key "to-url") are mutually exclusive`, err.Error())
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
}

func TestGroupErrors_AndPartialFromConfig(t *testing.T) {
	t.Parallel()

	app := conga.New[GroupAndCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"user": "alice"}`))).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `flag --user (from config key "user") must be given together with --password and --host`, err.Error())
	assert.IsError(t, err, conga.ErrConfig)
	assert.False(t, conga.IsUsageError(err))
}

func TestGroupErrors_AndCLIWithEnvPartner(t *testing.T) {
	t.Setenv("CONGA_TEST_GROUP_PASS", "s3cret")

	app := conga.New[GroupAndCLI]().Args([]string{"-u", "alice"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags -u and --password (from environment variable CONGA_TEST_GROUP_PASS) must be given together with --host", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
}

func TestGroupErrors_AndSatisfiedAcrossSources(t *testing.T) {
	t.Setenv("CONGA_TEST_GROUP_USER", "alice")
	t.Setenv("CONGA_TEST_GROUP_PASS", "s3cret")

	app := conga.New[GroupAndCLI]().Args([]string{"--host", "example.com"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
}

func TestGroupErrors_ShortSpelling(t *testing.T) {
	t.Parallel()

	app := conga.New[GroupSpellingCLI]().Args([]string{"-jy"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags -j and -y are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestGroupErrors_AliasSpelling(t *testing.T) {
	t.Parallel()

	app := conga.New[GroupSpellingCLI]().Args([]string{"--as-json", "--yaml"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --as-json and --yaml are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestGroupErrors_LastSpellingWins(t *testing.T) {
	t.Parallel()

	app := conga.New[GroupSpellingCLI]().Args([]string{"--json", "-j", "--yaml"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags -j and --yaml are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestGroupErrors_WhyRendersOnEnvConflict(t *testing.T) {
	t.Setenv("CONGA_TEST_GROUP_GZIP", "true")
	t.Setenv("CONGA_TEST_GROUP_ZSTD", "true")

	var buffer bytes.Buffer

	app := conga.New[GroupXorEnvCLI]().
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--why"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Value provenance")
	assert.Contains(t, buffer.String(), "CONGA_TEST_GROUP_GZIP")
	assert.Contains(t, buffer.String(), "CONGA_TEST_GROUP_ZSTD")
}

func TestGroupErrors_WhyStillFailsOnValueConstraint(t *testing.T) {
	t.Parallel()

	type WhyRangeCLI struct {
		noopRunner

		Gzip  conga.OptionalFlag[bool] `name:"gzip"  xor:"compression"`
		Zstd  conga.OptionalFlag[bool] `name:"zstd"  xor:"compression"`
		Level conga.OptionalFlag[int]  `name:"level" range:"1..9"`
	}

	app := conga.New[WhyRangeCLI]().
		EnableWhy().
		Stdout(new(bytes.Buffer)).
		Args([]string{"--why", "--gzip", "--zstd", "--level", "12"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "value 12 is out of range [1..9] for flag --level", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}
