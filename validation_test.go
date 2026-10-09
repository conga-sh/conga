package conga_test

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

// --- Custom Type Definitions and Type Aliases ---

// Type definition for int with TextUnmarshaler.
type CustomPort int

func (p *CustomPort) UnmarshalText(text []byte) error {
	value, err := strconv.Atoi(string(text))
	if err != nil {
		return fmt.Errorf("invalid port %q: %w", text, err)
	}

	if value < 1 || value > 65535 {
		return fmt.Errorf("port %d out of range [1..65535]", value)
	}

	*p = CustomPort(value)

	return nil
}

// Type definition for string with TextUnmarshaler.
type CustomUsername string

func (u *CustomUsername) UnmarshalText(text []byte) error {
	rawString := string(text)
	if len(rawString) < 3 || len(rawString) > 16 {
		return fmt.Errorf("username %q length must be between 3 and 16", rawString)
	}

	if strings.Contains(rawString, " ") {
		return fmt.Errorf("username %q cannot contain spaces", rawString)
	}

	*u = CustomUsername(rawString)

	return nil
}

// Type definition for int with SelfValidator interface.
type SelfValidatingScore int

func (s SelfValidatingScore) Validate(_ context.Context) error {
	if s < 0 || s > 100 {
		return fmt.Errorf("score %d must be between 0 and 100", s)
	}

	return nil
}

// Type definition for string with SelfValidator interface.
type SelfValidatingSlug string

func (s SelfValidatingSlug) Validate(_ context.Context) error {
	slug := string(s)
	if strings.ToLower(slug) != slug {
		return fmt.Errorf("slug %q must be all lowercase", slug)
	}

	return nil
}

// Celsius is a signed text dialect: range bounds parse through its own
// UnmarshalText grammar and compare as int64.
type Celsius int64

func (c *Celsius) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseInt(strings.TrimSuffix(string(text), "C"), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid temperature %q: %w", text, err)
	}

	*c = Celsius(parsed)

	return nil
}

// Count is an unsigned text dialect: range bounds compare as uint64 without
// clamping to int64.
type Count uint64

func (c *Count) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseUint(strings.TrimSuffix(string(text), "x"), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid count %q: %w", text, err)
	}

	*c = Count(parsed)

	return nil
}

// Ratio is a float text dialect: range bounds compare as float64.
type Ratio float64

func (r *Ratio) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseFloat(strings.TrimSuffix(string(text), "%"), 64)
	if err != nil {
		return fmt.Errorf("invalid ratio %q: %w", text, err)
	}

	*r = Ratio(parsed)

	return nil
}

// Type aliases.
type AliasInt = int
type AliasString = string

// --- Test CLI Structs ---

type RangeNumericCLI struct {
	Count      conga.OptionalFlag[int]     `name:"count"     range:"1..10"`
	MinOnly    conga.OptionalFlag[int]     `name:"min-only"  range:"5.."`
	MaxOnly    conga.OptionalFlag[int]     `name:"max-only"  range:"..50"`
	Offset     conga.OptionalFlag[int]     `name:"offset"    range:"-20..-5"`
	FloatValue conga.OptionalFlag[float64] `name:"float-val" range:"0.0..1.0"`
}

func (r *RangeNumericCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type RangeUnitCLI struct {
	Timeout conga.OptionalFlag[time.Duration]  `name:"timeout" range:"100ms..5m"`
	Memory  conga.OptionalFlag[conga.ByteSize] `name:"memory"  range:"1MB..10GB"`
}

func (r *RangeUnitCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type ByteSizeUnitsRangeMismatchCLI struct {
	Memory conga.OptionalFlag[conga.ByteSize] `name:"memory" range:"1MB..10MB" units:"binary"`
}

func (b *ByteSizeUnitsRangeMismatchCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type RangeTextDialectCLI struct {
	Temperature conga.OptionalFlag[Celsius] `name:"temperature" range:"10C..30C"`
}

func (r *RangeTextDialectCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type BadRangeTextDialectCLI struct {
	Temperature conga.OptionalFlag[Celsius] `name:"temperature" range:"bananaC..30C"`
}

func (b *BadRangeTextDialectCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type RangeUnsignedDialectCLI struct {
	Threshold conga.OptionalFlag[Count] `name:"threshold" range:"9223372036854775808x..9223372036854775809x"`
}

func (r *RangeUnsignedDialectCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type RangeFloatDialectCLI struct {
	Ratio conga.OptionalFlag[Ratio] `name:"ratio" range:"10%..90%"`
}

func (r *RangeFloatDialectCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type RangeSliceDialectCLI struct {
	Temperatures conga.OptionalFlag[[]Celsius] `name:"temperatures" range:"10C..30C"`
}

func (r *RangeSliceDialectCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type RangeUintCLI struct {
	Port   conga.OptionalFlag[uint]     `name:"port"   range:"1..65535"`
	Worker conga.OptionalFlag[uint8]    `name:"worker" range:"..8"`
	Level  conga.OptionalFlag[uint64]   `name:"level"  range:"10.."`
	Octets conga.OptionalFlag[[]uint16] `name:"octets" range:"0..255"`
}

func (r *RangeUintCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type LengthCLI struct {
	Username conga.OptionalFlag[string] `len:"3..16" name:"user"`
	Pin      conga.OptionalFlag[string] `len:"4"     name:"pin"`
	MinPass  conga.OptionalFlag[string] `len:"8.."   name:"pass"`
	MaxBio   conga.OptionalFlag[string] `len:"..50"  name:"bio"`
}

func (l *LengthCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type SliceConstraintsCLI struct {
	Ports conga.OptionalFlag[[]int]    `items:"1..3" name:"ports" range:"1..10"`
	Tags  conga.OptionalFlag[[]string] `items:"2..4" len:"2..6"   name:"tags"`
}

func (s *SliceConstraintsCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type CustomTypesCLI struct {
	Port     conga.OptionalFlag[CustomPort]          `name:"port"`
	User     conga.OptionalFlag[CustomUsername]      `name:"user"`
	Score    conga.OptionalFlag[SelfValidatingScore] `name:"score"`
	Slug     conga.OptionalFlag[SelfValidatingSlug]  `name:"slug"`
	AliasNum conga.OptionalFlag[AliasInt]            `name:"alias-num" range:"1..100"`
	AliasStr conga.OptionalFlag[AliasString]         `len:"3..10"      name:"alias-str"`
	Negative conga.OptionalFlag[AliasInt]            `name:"neg-alias" range:"-50..-1"`
}

func (c *CustomTypesCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type PositionalRangeCLI struct {
	Retries conga.RequiredArg[int]    `name:"retries" range:"1..5"`
	Name    conga.RequiredArg[string] `len:"3..10"    name:"name"`
}

func (p *PositionalRangeCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

type PositionalSliceCLI struct {
	Items conga.RequiredArg[[]int] `items:"2..4" name:"items" range:"1..10"`
}

func (p *PositionalSliceCLI) Run(ctx context.Context, session *conga.Session) error { return nil }

// --- Tests ---

func TestNumericRangeValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid closed range", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeNumericCLI)
		app := conga.New(cli).Args([]string{"--count", "5"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, 5, cli.Count.Get())
	})

	t.Run("out of range below min", func(t *testing.T) {
		t.Parallel()

		app := conga.New[RangeNumericCLI]().Args([]string{"--count", "0"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("out of range above max", func(t *testing.T) {
		t.Parallel()

		app := conga.New[RangeNumericCLI]().Args([]string{"--count", "11"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("min only range valid at lower bound", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--min-only", "5"}).Run()

		assert.NoError(t, err)
	})

	t.Run("min only range valid above lower bound", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--min-only", "100"}).Run()

		assert.NoError(t, err)
	})

	t.Run("min only range invalid below min", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--min-only", "4"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("max only range valid", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--max-only", "50"}).Run()

		assert.NoError(t, err)
	})

	t.Run("max only range invalid above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--max-only", "51"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("negative numbers in range valid", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeNumericCLI)
		app := conga.New(cli).Args([]string{"--offset", "-10"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, -10, cli.Offset.Get())
	})

	t.Run("negative numbers in range invalid below min", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--offset", "-30"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("negative numbers in range invalid above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--offset", "0"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("float range valid", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--float-val", "0.75"}).Run()

		assert.NoError(t, err)
	})

	t.Run("float range invalid above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--float-val", "1.5"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("float range invalid below min", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeNumericCLI]().Args([]string{"--float-val", "-0.1"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestUnitsRangeValidation(t *testing.T) {
	t.Parallel()

	t.Run("duration range valid", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeUnitCLI)
		app := conga.New(cli).Args([]string{"--timeout", "2m"})
		err := app.Run()
		assert.NoError(t, err)
		assert.Equal(t, 2*time.Minute, cli.Timeout.Get())
	})

	t.Run("duration range below min", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUnitCLI]().Args([]string{"--timeout", "50ms"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("duration range above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUnitCLI]().Args([]string{"--timeout", "10m"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("bytesize range valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[RangeUnitCLI]().Args([]string{"--memory", "500MB"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("bytesize range below min", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUnitCLI]().Args([]string{"--memory", "500kB"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("bytesize range above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUnitCLI]().Args([]string{"--memory", "20GB"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("bytesize bound violating units tag rejected at schema build", func(t *testing.T) {
		t.Parallel()

		err := conga.New[ByteSizeUnitsRangeMismatchCLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct ByteSizeUnitsRangeMismatchCLI field Memory: schema error: invalid range syntax: invalid byte size min bound "1MB"`, err.Error())
	})
}

func TestTextDialectRangeValidation(t *testing.T) {
	t.Parallel()

	t.Run("value within dialect bounds", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeTextDialectCLI)

		err := conga.New(cli).Args([]string{"--temperature", "20C"}).Run()

		assert.NoError(t, err)
		assert.Equal(t, Celsius(20), cli.Temperature.Get())
	})

	t.Run("value below dialect min", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeTextDialectCLI]().Args([]string{"--temperature", "5C"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("value above dialect max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeTextDialectCLI]().Args([]string{"--temperature", "50C"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("unparseable dialect bound rejected at schema build", func(t *testing.T) {
		t.Parallel()

		err := conga.New[BadRangeTextDialectCLI]().Args([]string{"--temperature", "20C"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
	})

	t.Run("unsigned dialect value above int64 max accepted", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeUnsignedDialectCLI)

		err := conga.New(cli).Args([]string{"--threshold", "9223372036854775808x"}).Run()

		assert.NoError(t, err)
		assert.Equal(t, Count(9223372036854775808), cli.Threshold.Get())
	})

	t.Run("unsigned dialect value below min rejected", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUnsignedDialectCLI]().Args([]string{"--threshold", "5x"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("float dialect value within bounds", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeFloatDialectCLI]().Args([]string{"--ratio", "50%"}).Run()

		assert.NoError(t, err)
	})

	t.Run("float dialect value above max rejected", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeFloatDialectCLI]().Args([]string{"--ratio", "95%"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("slice dialect validates each element", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeSliceDialectCLI]().Args([]string{"--temperatures", "20C,25C"}).Run()

		assert.NoError(t, err)
	})

	t.Run("slice dialect rejects out-of-range element", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeSliceDialectCLI]().Args([]string{"--temperatures", "20C,50C"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestUintRangeValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid closed range", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeUintCLI)
		app := conga.New(cli).Args([]string{"--port", "8080"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, uint(8080), cli.Port.Get())
	})

	t.Run("out of range below min", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUintCLI]().Args([]string{"--port", "0"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "value 0 is out of range [1..65535] for flag --port", err.Error())
	})

	t.Run("out of range above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUintCLI]().Args([]string{"--port", "70000"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "value 70000 is out of range [1..65535] for flag --port", err.Error())
	})

	t.Run("closed range boundary min accepted", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeUintCLI)
		app := conga.New(cli).Args([]string{"--port", "1"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, uint(1), cli.Port.Get())
	})

	t.Run("closed range boundary max accepted", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeUintCLI)
		app := conga.New(cli).Args([]string{"--port", "65535"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, uint(65535), cli.Port.Get())
	})

	t.Run("max only range violated", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUintCLI]().Args([]string{"--worker", "9"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "value 9 must be <= 8 for flag --worker", err.Error())
	})

	t.Run("max only range boundary accepted", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeUintCLI)
		app := conga.New(cli).Args([]string{"--worker", "8"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, uint8(8), cli.Worker.Get())
	})

	t.Run("min only range violated", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUintCLI]().Args([]string{"--level", "9"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "value 9 must be >= 10 for flag --level", err.Error())
	})

	t.Run("min only range boundary accepted", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeUintCLI)
		app := conga.New(cli).Args([]string{"--level", "10"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, uint64(10), cli.Level.Get())
	})

	t.Run("slice element out of range", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUintCLI]().Args([]string{"--octets", "80,300"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "value 300 is out of range [0..255] for flag --octets", err.Error())
	})

	t.Run("slice element boundaries accepted", func(t *testing.T) {
		t.Parallel()

		cli := new(RangeUintCLI)
		app := conga.New(cli).Args([]string{"--octets", "0,255"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, uint16(0), cli.Octets.Get()[0])
		assert.Equal(t, uint16(255), cli.Octets.Get()[1])
	})

	t.Run("negative input fails coercion before range check", func(t *testing.T) {
		t.Parallel()

		err := conga.New[RangeUintCLI]().Args([]string{"--port", "-1"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
	})
}

func TestStringLengthValidation(t *testing.T) {
	t.Parallel()

	t.Run("closed string length range valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[LengthCLI]().Args([]string{"--user", "alice"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("closed string length below min", func(t *testing.T) {
		t.Parallel()

		err := conga.New[LengthCLI]().Args([]string{"--user", "ab"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("closed string length above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[LengthCLI]().Args([]string{"--user", "thisusernameiswaytoolongtobeallowed"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("exact string length valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[LengthCLI]().Args([]string{"--pin", "1234"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("exact string length invalid", func(t *testing.T) {
		t.Parallel()

		err := conga.New[LengthCLI]().Args([]string{"--pin", "123"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("min only string length valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[LengthCLI]().Args([]string{"--pass", "supersecretpassword"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("min only string length invalid", func(t *testing.T) {
		t.Parallel()

		err := conga.New[LengthCLI]().Args([]string{"--pass", "short"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("max only string length valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[LengthCLI]().Args([]string{"--bio", "Hello world"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("max only string length invalid", func(t *testing.T) {
		t.Parallel()

		err := conga.New[LengthCLI]().Args([]string{"--bio", strings.Repeat("x", 55)}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestSliceConstraints(t *testing.T) {
	t.Parallel()

	t.Run("slice items count and element range valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceConstraintsCLI]().Args([]string{"--ports", "1,2,3"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("slice item count violation above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[SliceConstraintsCLI]().Args([]string{"--ports", "1,2,3,4"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("slice element range violation above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[SliceConstraintsCLI]().Args([]string{"--ports", "1,15"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("slice string length validation valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceConstraintsCLI]().Args([]string{"--tags", "web,api,db"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("slice string item too short", func(t *testing.T) {
		t.Parallel()

		err := conga.New[SliceConstraintsCLI]().Args([]string{"--tags", "a,api"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("slice too few items", func(t *testing.T) {
		t.Parallel()

		err := conga.New[SliceConstraintsCLI]().Args([]string{"--tags", "web"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestCustomTypesAndAliases(t *testing.T) {
	t.Parallel()

	t.Run("CustomPort with TextUnmarshaler valid", func(t *testing.T) {
		t.Parallel()

		cli := new(CustomTypesCLI)
		app := conga.New(cli).Args([]string{"--port", "8080"})
		err := app.Run()
		assert.NoError(t, err)
		assert.Equal(t, 8080, int(cli.Port.Get()))
	})

	t.Run("CustomPort invalid out of range", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--port", "70000"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, "port 70000 out of range [1..65535] for flag --port", err.Error())
	})

	t.Run("CustomUsername with TextUnmarshaler valid", func(t *testing.T) {
		t.Parallel()

		cli := new(CustomTypesCLI)
		app := conga.New(cli).Args([]string{"--user", "john_doe"})
		err := app.Run()
		assert.NoError(t, err)
		assert.Equal(t, "john_doe", string(cli.User.Get()))
	})

	t.Run("CustomUsername invalid short", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--user", "ab"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, `username "ab" length must be between 3 and 16 for flag --user`, err.Error())
	})

	t.Run("CustomUsername invalid with space", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--user", "john doe"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, `username "john doe" cannot contain spaces for flag --user`, err.Error())
	})

	t.Run("SelfValidatingScore with SelfValidator interface valid", func(t *testing.T) {
		t.Parallel()

		cli := new(CustomTypesCLI)
		app := conga.New(cli).Args([]string{"--score", "85"})
		err := app.Run()
		assert.NoError(t, err)
		assert.Equal(t, 85, int(cli.Score.Get()))
	})

	t.Run("SelfValidatingScore invalid above 100", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--score", "150"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "score 150 must be between 0 and 100 for flag --score", err.Error())
	})

	t.Run("SelfValidatingScore invalid below 0", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--score", "-5"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "score -5 must be between 0 and 100 for flag --score", err.Error())
	})

	t.Run("SelfValidatingSlug with SelfValidator interface valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CustomTypesCLI]().Args([]string{"--slug", "my-cluster-01"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("SelfValidatingSlug invalid uppercase", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--slug", "My-Cluster-01"}).Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, `slug "My-Cluster-01" must be all lowercase for flag --slug`, err.Error())
	})

	t.Run("Type Alias with range tag valid", func(t *testing.T) {
		t.Parallel()

		cli := new(CustomTypesCLI)
		app := conga.New(cli).Args([]string{"--alias-num", "42"})
		err := app.Run()
		assert.NoError(t, err)
		assert.Equal(t, 42, cli.AliasNum.Get())
	})

	t.Run("Type Alias with range tag invalid above 100", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--alias-num", "150"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("Type Alias with len tag valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CustomTypesCLI]().Args([]string{"--alias-str", "foobar"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("Type Alias with len tag invalid too short", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--alias-str", "hi"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("Type Alias with negative number parsing valid", func(t *testing.T) {
		t.Parallel()

		cli := new(CustomTypesCLI)
		app := conga.New(cli).Args([]string{"--neg-alias", "-25"})
		err := app.Run()
		assert.NoError(t, err)
		assert.Equal(t, -25, cli.Negative.Get())
	})

	t.Run("Type Alias with negative number above max", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--neg-alias", "10"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("Type Alias with negative number below min", func(t *testing.T) {
		t.Parallel()

		err := conga.New[CustomTypesCLI]().Args([]string{"--neg-alias", "-60"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestPositionalValidation(t *testing.T) {
	t.Parallel()

	t.Run("scalar positional valid", func(t *testing.T) {
		t.Parallel()

		cli := new(PositionalRangeCLI)
		app := conga.New(cli).Args([]string{"3", "deploy"})
		err := app.Run()
		assert.NoError(t, err)
		assert.Equal(t, 3, cli.Retries.Get())
		assert.Equal(t, "deploy", cli.Name.Get())
	})

	t.Run("scalar positional retries out of range", func(t *testing.T) {
		t.Parallel()

		err := conga.New[PositionalRangeCLI]().Args([]string{"10", "deploy"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("scalar positional name length violation", func(t *testing.T) {
		t.Parallel()

		err := conga.New[PositionalRangeCLI]().Args([]string{"3", "ab"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("slice positional valid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[PositionalSliceCLI]().Args([]string{"1", "5", "10"})
		err := app.Run()
		assert.NoError(t, err)
	})

	t.Run("slice positional too few items", func(t *testing.T) {
		t.Parallel()

		err := conga.New[PositionalSliceCLI]().Args([]string{"5"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("slice positional item out of range", func(t *testing.T) {
		t.Parallel()

		err := conga.New[PositionalSliceCLI]().Args([]string{"5", "20"}).Run()
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestHelpConstraintHints(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[RangeNumericCLI]().
		Name("test").
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)

	helpText := buffer.String()
	assert.Contains(t, helpText, "[1..10]")
	assert.Contains(t, helpText, "[>= 5]")
	assert.Contains(t, helpText, "[<= 50]")
	assert.Contains(t, helpText, "[-20..-5]")
}
