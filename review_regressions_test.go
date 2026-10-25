package conga_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

// Counter is a user-defined struct named Counter in a test package.
// It must NOT be hijacked into a CONGA flag counter.
type Counter struct {
	Total int
}

func (u *Counter) UnmarshalText(text []byte) error {
	u.Total = len(text)

	return nil
}

// ConfigFile is a user-defined struct named ConfigFile in a test package.
// It must NOT be hijacked into a CONGA configuration-file container.
type ConfigFile struct {
	Location string
}

func (u *ConfigFile) UnmarshalText(text []byte) error {
	u.Location = string(text)

	return nil
}

type UserTypeCollisionCLI struct {
	MyCounter conga.OptionalFlag[Counter]
	MyConfig  conga.OptionalFlag[ConfigFile]
}

func (c *UserTypeCollisionCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// TestRegression_UserTypesNotHijacked reproduces Finding 2 from REVIEW.md:
// User types named "Counter" or "ConfigFile" in foreign packages must not trigger
// CONGA special-case counter or config-file behavior.
func TestRegression_UserTypesNotHijacked(t *testing.T) {
	t.Parallel()

	cli := new(UserTypeCollisionCLI)
	app := conga.New(cli).
		Args([]string{"--my-counter", "hello", "--my-config", "some-path"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 5, cli.MyCounter.Get().Total)
	assert.Equal(t, "some-path", cli.MyConfig.Get().Location)
}

type InvalidArgOrderWithDefaultCLI struct {
	Opt conga.OptionalArg[string] `default:"fallback"`
	Req conga.RequiredArg[string]
}

func (c *InvalidArgOrderWithDefaultCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// TestRegression_ArgOrderDefaultTagHole reproduces Finding 4 from REVIEW.md:
// An optional argument with a default tag preceding a required argument must be rejected
// at schema construction time with ErrInvalidArgOrder.
func TestRegression_ArgOrderDefaultTagHole(t *testing.T) {
	t.Parallel()

	app := conga.New[InvalidArgOrderWithDefaultCLI]().
		Args([]string{"hello"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, "struct InvalidArgOrderWithDefaultCLI field Req: schema error: invalid positional argument order: optional positional argument cannot precede required positional argument <req>", err.Error())
}

type MapArgConstraintCLI struct {
	Pairs conga.RequiredArg[map[string]string] `items:"2..4"`
}

func (c *MapArgConstraintCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// TestRegression_MapArgConstraintEnforced reproduces Finding 5 from REVIEW.md:
// Map positional arguments must validate entry counts against the items tag.
func TestRegression_MapArgConstraintEnforced(t *testing.T) {
	t.Parallel()

	app := conga.New[MapArgConstraintCLI]().
		Args([]string{"a=1"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Equal(t, "item count 1 is out of range [2..4] for argument <pairs>", err.Error())
}

type ConfigFileFallbackCLI struct {
	Config conga.ConfigFile `short:"c"`
}

func (c *ConfigFileFallbackCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type ConfigFileMultiFallbackCLI struct {
	Config conga.ConfigFile `multi:"true" short:"c"`
}

func (c *ConfigFileMultiFallbackCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// TestRegression_ConfigFileEmptyValueErrors reproduces Finding 6 from REVIEW.md:
// A config-file flag supplied with an empty string names no path. It is
// rejected as a usage error instead of silently falling through to the
// environment variable or the search tier, which would load a different
// configuration than the one the user named.
func TestRegression_ConfigFileEmptyValueErrors(t *testing.T) {
	t.Parallel()

	app := conga.New[ConfigFileFallbackCLI]().
		Args([]string{"-c", ""})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Equal(t, "path cannot be empty for flag --config-file", err.Error())
}

// TestRegression_ConfigFileMultiBlankEntryErrors pins the multi-arity variant:
// supplying the flag with only a blank value names no path and is rejected as
// well, so it cannot fall through to the environment variable or the search
// tier. (Blank slots inside a sep-joined list remain tolerated list syntax.)
func TestRegression_ConfigFileMultiBlankEntryErrors(t *testing.T) {
	t.Parallel()

	app := conga.New[ConfigFileMultiFallbackCLI]().
		Args([]string{"-c", ""})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Equal(t, "path cannot be empty for flag --config-file", err.Error())
}

type TypedMapsCLI struct {
	IntMap      conga.OptionalFlag[map[string]int]           `short:"i"`
	BoolMap     conga.OptionalFlag[map[string]bool]          `short:"b"`
	DurationMap conga.OptionalFlag[map[string]time.Duration] `short:"d"`
	PosWeights  conga.RequiredArg[map[string]float64]
}

func (c *TypedMapsCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// TestTypedMaps_ArbitraryTypes validates that arbitrary map[string]V types are supported
// across flags and positional arguments once registered via app.RegisterMap.
func TestTypedMaps_ArbitraryTypes(t *testing.T) {
	t.Parallel()

	cli := new(TypedMapsCLI)
	app := conga.New(cli).
		Args([]string{"-i", "web=3,worker=5", "-b", "auth=true,billing=false", "-d", "api=5s,db=30s", "lr=0.01,decay=0.001"})

	app.RegisterMap[map[string]int]()
	app.RegisterMap[map[string]bool]()
	app.RegisterMap[map[string]time.Duration]()
	app.RegisterMap[map[string]float64]()

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]int{"web": 3, "worker": 5}, cli.IntMap.Get())
	assert.Equal(t, map[string]bool{"auth": true, "billing": false}, cli.BoolMap.Get())
	assert.Equal(t, map[string]time.Duration{"api": 5 * time.Second, "db": 30 * time.Second}, cli.DurationMap.Get())
	assert.Equal(t, map[string]float64{"lr": 0.01, "decay": 0.001}, cli.PosWeights.Get())
}

type namedStringMap map[string]string

type unregisteredTypedMapCLI struct {
	IntMap conga.OptionalFlag[map[string]int] `short:"i"`
}

func (c *unregisteredTypedMapCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type unregisteredNamedStringMapCLI struct {
	Names conga.OptionalFlag[namedStringMap] `short:"n"`
}

func (c *unregisteredNamedStringMapCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type nonStringKeyMapCLI struct {
	Counts conga.OptionalFlag[map[int]string] `short:"c"`
}

func (c *nonStringKeyMapCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

// TestMapRegistrationRequirements pins the registration boundary: map[string]string and
// named map[string]string types work by default, typed values require RegisterMap, and
// non-string keys panic at schema init.
func TestMapRegistrationRequirements(t *testing.T) {
	t.Parallel()

	t.Run("unregistered typed map fails loudly", func(t *testing.T) {
		t.Parallel()

		app := conga.New[unregisteredTypedMapCLI]().
			Args([]string{"-i", "web=3,worker=5"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
	})

	t.Run("unregistered named map[string]string works", func(t *testing.T) {
		t.Parallel()

		cli := new(unregisteredNamedStringMapCLI)
		app := conga.New(cli).
			Args([]string{"-n", "web=3,worker=5"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, namedStringMap{"web": "3", "worker": "5"}, cli.Names.Get())
	})

	t.Run("non-string map keys rejected at schema init", func(t *testing.T) {
		t.Parallel()

		app := conga.New[nonStringKeyMapCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct nonStringKeyMapCLI field Counts: schema error: map keys must be of type string, got int", err.Error())
	})
}

type MapEnumFlagCLI struct {
	Options conga.OptionalFlag[map[string]string] `enum:"a,b"`
}

func (c *MapEnumFlagCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type MapRangeFlagCLI struct {
	Options conga.OptionalFlag[map[string]int] `range:"1..10"`
}

func (c *MapRangeFlagCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type MapLenFlagCLI struct {
	Options conga.OptionalFlag[map[string]string] `len:"3..5"`
}

func (c *MapLenFlagCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type MapEnumArgCLI struct {
	Options conga.RequiredArg[map[string]string] `enum:"a,b"`
}

func (c *MapEnumArgCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type MapRangeArgCLI struct {
	Options conga.RequiredArg[map[string]int] `range:"1..10"`
}

func (c *MapRangeArgCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type MapLenArgCLI struct {
	Options conga.RequiredArg[map[string]string] `len:"3..5"`
}

func (c *MapLenArgCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

// TestRegression_MapSchemaRejectsScalarConstraints reproduces Finding 2 from REVIEW2.md:
// Scalar constraint tags (enum, range, len) on map flags and arguments are inherently ambiguous
// and must be rejected at schema construction time with a descriptive panic.
func TestRegression_MapSchemaRejectsScalarConstraints(t *testing.T) {
	t.Parallel()

	t.Run("enum tag on map flag rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MapEnumFlagCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct MapEnumFlagCLI field Options: schema error: invalid struct tag: tag "enum" is not supported on map types. Use tag "items" to constrain the number of map entries, or custom domain types with SelfValidator to validate map entries`, err.Error())
	})

	t.Run("range tag on map flag rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MapRangeFlagCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct MapRangeFlagCLI field Options: schema error: invalid struct tag: tag "range" is not supported on map types. Use tag "items" to constrain the number of map entries, or custom domain types with SelfValidator to validate map entries`, err.Error())
	})

	t.Run("len tag on map flag rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MapLenFlagCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct MapLenFlagCLI field Options: schema error: invalid struct tag: tag "len" is not supported on map types. Use tag "items" to constrain the number of map entries, or custom domain types with SelfValidator to validate map entries`, err.Error())
	})

	t.Run("enum tag on map arg rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MapEnumArgCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct MapEnumArgCLI field Options: schema error: invalid struct tag: tag "enum" is not supported on map types. Use tag "items" to constrain the number of map entries, or custom domain types with SelfValidator to validate map entries`, err.Error())
	})

	t.Run("range tag on map arg rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MapRangeArgCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct MapRangeArgCLI field Options: schema error: invalid struct tag: tag "range" is not supported on map types. Use tag "items" to constrain the number of map entries, or custom domain types with SelfValidator to validate map entries`, err.Error())
	})

	t.Run("len tag on map arg rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MapLenArgCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct MapLenArgCLI field Options: schema error: invalid struct tag: tag "len" is not supported on map types. Use tag "items" to constrain the number of map entries, or custom domain types with SelfValidator to validate map entries`, err.Error())
	})
}

type CustomHeaderMap map[string]string

func (m CustomHeaderMap) Validate(_ context.Context) error {
	for k := range m {
		if strings.TrimSpace(k) == "" {
			return errors.New("header name cannot be empty")
		}

		if strings.Contains(k, " ") {
			return errors.New("header name cannot contain spaces")
		}
	}

	return nil
}

type SelfValidatingMapFlagCLI struct {
	Headers conga.OptionalFlag[CustomHeaderMap]
}

func (c *SelfValidatingMapFlagCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

// TestRegression_SelfValidatingMapType ensures that custom map domain types implementing
// conga.SelfValidator are validated during flag binding.
func TestRegression_SelfValidatingMapType(t *testing.T) {
	t.Parallel()

	t.Run("valid custom map", func(t *testing.T) {
		t.Parallel()

		cli := new(SelfValidatingMapFlagCLI)
		app := conga.New(cli).
			Args([]string{"--headers", "Content-Type=application/json,Accept=text/html"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "application/json", cli.Headers.Get()["Content-Type"])
		assert.Equal(t, "text/html", cli.Headers.Get()["Accept"])
	})

	t.Run("invalid custom map fails validation", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SelfValidatingMapFlagCLI]().
			Args([]string{"--headers", "Bad Header=value"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "header name cannot contain spaces for flag --headers", err.Error())
	})
}

type ValidatedPortNumber int

func (p ValidatedPortNumber) Validate(_ context.Context) error {
	if p < 1 || p > 65535 {
		return fmt.Errorf("port %d is out of range 1..65535", p)
	}

	return nil
}

type SelfValidatingMapEntryCLI struct {
	Ports conga.OptionalFlag[map[string]ValidatedPortNumber]
}

func (c *SelfValidatingMapEntryCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

// TestRegression_SelfValidatingMapEntryValues ensures that map entry values implementing
// conga.SelfValidator are individually validated in deterministic sorted key order.
func TestRegression_SelfValidatingMapEntryValues(t *testing.T) {
	t.Parallel()

	t.Run("valid map entry values", func(t *testing.T) {
		t.Parallel()

		cli := new(SelfValidatingMapEntryCLI)
		app := conga.New(cli).
			Args([]string{"--ports", "http=80,https=443"})

		app.RegisterMap[map[string]ValidatedPortNumber]()

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, ValidatedPortNumber(80), cli.Ports.Get()["http"])
		assert.Equal(t, ValidatedPortNumber(443), cli.Ports.Get()["https"])
	})

	t.Run("invalid map entry value fails with entry key in error", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SelfValidatingMapEntryCLI]().
			Args([]string{"--ports", "http=80,bad=99999"})

		app.RegisterMap[map[string]ValidatedPortNumber]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, `port 99999 is out of range 1..65535 for flag --ports entry "bad"`, err.Error())
	})
}

type SelfValidatingMapArgCLI struct {
	Headers conga.RequiredArg[CustomHeaderMap]
}

func (c *SelfValidatingMapArgCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

// TestRegression_SelfValidatingMapArg ensures that positional map arguments implementing
// conga.SelfValidator are validated.
func TestRegression_SelfValidatingMapArg(t *testing.T) {
	t.Parallel()

	t.Run("valid positional map arg", func(t *testing.T) {
		t.Parallel()

		cli := new(SelfValidatingMapArgCLI)
		app := conga.New(cli).
			Args([]string{"Content-Type=application/json"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "application/json", cli.Headers.Get()["Content-Type"])
	})

	t.Run("invalid positional map arg fails validation", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SelfValidatingMapArgCLI]().
			Args([]string{"Invalid Key=val"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "header name cannot contain spaces for argument <headers>", err.Error())
	})
}
