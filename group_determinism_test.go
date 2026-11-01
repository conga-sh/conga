package conga_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type TwoXorGroupsCLI struct {
	noopRunner

	FirstA  conga.OptionalFlag[bool] `name:"first-a"  xor:"first"`
	FirstB  conga.OptionalFlag[bool] `name:"first-b"  xor:"first"`
	SecondA conga.OptionalFlag[bool] `name:"second-a" xor:"second"`
	SecondB conga.OptionalFlag[bool] `name:"second-b" xor:"second"`
}

type TwoOneOfGroupsCLI struct {
	noopRunner

	FirstA  conga.OptionalFlag[bool] `name:"first-a"  oneof:"first"`
	FirstB  conga.OptionalFlag[bool] `name:"first-b"  oneof:"first"`
	SecondA conga.OptionalFlag[bool] `name:"second-a" oneof:"second"`
	SecondB conga.OptionalFlag[bool] `name:"second-b" oneof:"second"`
}

type TwoAndGroupsCLI struct {
	noopRunner

	FirstA  conga.OptionalFlag[bool] `and:"first"  name:"first-a"`
	FirstB  conga.OptionalFlag[bool] `and:"first"  name:"first-b"`
	SecondA conga.OptionalFlag[bool] `and:"second" name:"second-a"`
	SecondB conga.OptionalFlag[bool] `and:"second" name:"second-b"`
}

type CrossKindGroupsCLI struct {
	noopRunner

	XorA conga.OptionalFlag[bool] `name:"xor-a"    xor:"xor-group"`
	XorB conga.OptionalFlag[bool] `name:"xor-b"    xor:"xor-group"`
	OneA conga.OptionalFlag[bool] `name:"one-a"    oneof:"oneof-group"`
	OneB conga.OptionalFlag[bool] `name:"one-b"    oneof:"oneof-group"`
	AndA conga.OptionalFlag[bool] `and:"and-group" name:"and-a"`
	AndB conga.OptionalFlag[bool] `and:"and-group" name:"and-b"`
}

func TestGroupValidation_DeterministicXorOrder(t *testing.T) {
	t.Parallel()

	app := conga.New[TwoXorGroupsCLI]().Args([]string{"--first-a", "--first-b", "--second-a", "--second-b"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --first-a and --first-b are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestGroupValidation_DeterministicOneOfOrder(t *testing.T) {
	t.Parallel()

	app := conga.New[TwoOneOfGroupsCLI]().Args([]string{"--first-a", "--first-b", "--second-a", "--second-b"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --first-a and --first-b are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestGroupValidation_DeterministicAndOrder(t *testing.T) {
	t.Parallel()

	app := conga.New[TwoAndGroupsCLI]().Args([]string{"--first-a", "--second-a"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --first-a must be provided together with --first-b", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestGroupValidation_PinsCrossKindOrder(t *testing.T) {
	t.Parallel()

	app := conga.New[CrossKindGroupsCLI]().Args([]string{"--xor-a", "--xor-b", "--one-a", "--and-a"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --xor-a and --xor-b are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}
