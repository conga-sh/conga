package conga_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/hcl"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
)

var (
	// Offset, local, date, and time values in TOML and YAML, bound to
	// time.Time and string flags.
	//go:embed testdata/fixtures/decoded_datetimes.toml
	fixtureDecodedDatetimesTOML []byte

	//go:embed testdata/fixtures/decoded_datetimes.yaml
	fixtureDecodedDatetimesYAML []byte

	//go:embed testdata/fixtures/decoded_integers.json
	fixtureDecodedIntegers []byte
)

type decodedIntegersCLI struct {
	noopRunner

	MillionInt    conga.OptionalFlag[int]    `name:"million-int"`
	MillionInt64  conga.OptionalFlag[int64]  `name:"million-int64"`
	MillionUint64 conga.OptionalFlag[uint64] `name:"million-uint64"`
	ExactInt      conga.OptionalFlag[int]    `name:"exact-int"`
	ExactInt64    conga.OptionalFlag[int64]  `name:"exact-int64"`
	ExactUint64   conga.OptionalFlag[uint64] `name:"exact-uint64"`
	MaxUint64     conga.OptionalFlag[uint64] `name:"max-uint64"`
}

type decodedDatetimesCLI struct {
	noopRunner

	When      conga.OptionalFlag[time.Time] `name:"when"`
	WhenText  conga.OptionalFlag[string]    `name:"when-text"`
	LocalText conga.OptionalFlag[string]    `name:"local-text"`
	DateText  conga.OptionalFlag[string]    `name:"date-text"`
	TimeText  conga.OptionalFlag[string]    `name:"time-text"`
}

type decodedWhenCLI struct {
	noopRunner

	When conga.OptionalFlag[time.Time] `name:"when"`
}

type decodedCountCLI struct {
	noopRunner

	Count conga.OptionalFlag[string] `name:"count"`
}

func TestConfigValues_JSONIntegersBindExactly(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFixture(t, "config.json", fixtureDecodedIntegers)
	cli := new(decodedIntegersCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 1000000, cli.MillionInt.Get())
	assert.Equal(t, int64(1000000), cli.MillionInt64.Get())
	assert.Equal(t, uint64(1000000), cli.MillionUint64.Get())
	assert.Equal(t, 9007199254740993, cli.ExactInt.Get())
	assert.Equal(t, int64(9007199254740993), cli.ExactInt64.Get())
	assert.Equal(t, uint64(9007199254740993), cli.ExactUint64.Get())
	assert.Equal(t, uint64(18446744073709551615), cli.MaxUint64.Get())
}

func TestConfigValues_JSONIntegerBeyond64BitsRejected(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFixture(t, "config.json", []byte(`{"count": 18446744073709551616}`))
	app := conga.New(new(decodedCountCLI)).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err := app.Run()

	assert.IsError(t, err, conga.ErrConfig)
	assert.IsError(t, err, json.ErrParse)
	assert.Equal(t, fmt.Sprintf(`cannot parse config file %q: json parse error: /count: integer 18446744073709551616 out of range`, cfgPath), err.Error())
}

func TestConfigValues_YAMLIntegerBeyond64BitsRejected(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFixture(t, "config.yaml", []byte("count: 999999999999999999999999999999\n"))
	app := conga.New(new(decodedCountCLI)).
		RegisterDecoder(".yaml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err := app.Run()

	assert.IsError(t, err, conga.ErrConfig)
	assert.IsError(t, err, yaml.ErrParse)
	assert.Equal(t, fmt.Sprintf(`cannot parse config file %q: yaml parse error: line 1: integer 999999999999999999999999999999 out of range`, cfgPath), err.Error())
}

func TestConfigValues_HCLIntegerBeyond64BitsRejected(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFixture(t, "config.hcl", []byte("count = 999999999999999999999999999999\n"))
	app := conga.New(new(decodedCountCLI)).
		RegisterDecoder(".hcl", hcl.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err := app.Run()

	assert.IsError(t, err, conga.ErrConfig)
	assert.IsError(t, err, hcl.ErrParse)
	assert.Equal(t, fmt.Sprintf(`cannot parse config file %q: hcl parse error: config.hcl:1,1-39: attribute "count": number 999999999999999999999999999999 out of range`, cfgPath), err.Error())
}

func TestConfigValues_TOMLDatetimesBind(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFixture(t, "config.toml", fixtureDecodedDatetimesTOML)
	cli := new(decodedDatetimesCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, time.Date(1979, 5, 27, 7, 32, 0, 0, time.UTC), cli.When.Get().UTC())
	assert.Equal(t, "1979-05-27T07:32:00-07:00", cli.WhenText.Get())
	assert.Equal(t, "1979-05-27T07:32:00", cli.LocalText.Get())
	assert.Equal(t, "1979-05-27", cli.DateText.Get())
	assert.Equal(t, "07:32:00", cli.TimeText.Get())
}

func TestConfigValues_TOMLLocalDatetimeRejectedForTime(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFixture(t, "config.toml", []byte("when = 1979-05-27T07:32:00\n"))
	app := conga.New(new(decodedWhenCLI)).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err := app.Run()

	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, `invalid time "1979-05-27T07:32:00" (expected RFC 3339, e.g. 2006-01-02T15:04:05Z) for flag --when (from config key "when")`, err.Error())
}

func TestConfigValues_TOMLLocalDateRejectedForTime(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFixture(t, "config.toml", []byte("when = 1979-05-27\n"))
	app := conga.New(new(decodedWhenCLI)).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err := app.Run()

	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, `invalid time "1979-05-27" (expected RFC 3339, e.g. 2006-01-02T15:04:05Z) for flag --when (from config key "when")`, err.Error())
}

func TestConfigValues_YAMLTimestampsBind(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFixture(t, "config.yaml", fixtureDecodedDatetimesYAML)
	cli := new(decodedDatetimesCLI)
	app := conga.New(cli).
		RegisterDecoder(".yaml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, time.Date(1979, 5, 27, 7, 32, 0, 0, time.UTC), cli.When.Get().UTC())
	assert.Equal(t, "1979-05-27T07:32:00-07:00", cli.WhenText.Get())
}

func writeConfigFixture(t *testing.T, name string, content []byte) string {
	t.Helper()

	cfgPath := filepath.Join(t.TempDir(), name)
	assert.NoError(t, os.WriteFile(cfgPath, content, 0o600))

	return cfgPath
}
