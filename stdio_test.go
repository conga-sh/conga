package conga_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
)

type stdioWriterCLI struct {
	Output conga.OptionalFlag[conga.File] `stdout:"true"`
}

func (c *stdioWriterCLI) Run(_ context.Context, _ *conga.Session) error {
	writer, err := c.Output.Get().Writer()
	if err != nil {
		return err
	}

	_, err = io.WriteString(writer, "stdout payload")

	return err
}

type stdioReaderCLI struct {
	Input conga.OptionalFlag[conga.File] `stdin:"true"`

	payload string
}

func (c *stdioReaderCLI) Run(_ context.Context, _ *conga.Session) error {
	reader, err := c.Input.Get().Reader()
	if err != nil {
		return err
	}

	data, err := io.ReadAll(reader)
	c.payload = string(data)

	return err
}

type stdioSliceCLI struct {
	Inputs conga.OptionalFlag[[]conga.File] `name:"input" stdin:"true"`

	payload string
}

func (c *stdioSliceCLI) Run(_ context.Context, _ *conga.Session) error {
	inputs := c.Inputs.Get()

	reader, err := inputs[0].Reader()
	if err != nil {
		return err
	}

	data, err := io.ReadAll(reader)
	c.payload = string(data)

	return err
}

type stdioContentCLI struct {
	Payload conga.OptionalFlag[conga.FileContent]
}

func (c *stdioContentCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type stdioEnvCLI struct {
	Input conga.OptionalFlag[conga.File] `env:"CONGA_TEST_STDIO_TEST_INPUT" stdin:"true"`

	payload string
}

func (c *stdioEnvCLI) Run(_ context.Context, _ *conga.Session) error {
	reader, err := c.Input.Get().Reader()
	if err != nil {
		return err
	}

	data, err := io.ReadAll(reader)
	c.payload = string(data)

	return err
}

type stdioDefaultCLI struct {
	Input conga.OptionalFlag[conga.File] `default:"-" stdin:"true"`

	payload string
}

func (c *stdioDefaultCLI) Run(_ context.Context, _ *conga.Session) error {
	reader, err := c.Input.Get().Reader()
	if err != nil {
		return err
	}

	data, err := io.ReadAll(reader)
	c.payload = string(data)

	return err
}

type stdioConfigCLI struct {
	Input conga.OptionalFlag[conga.File] `config:"stdio.input" stdin:"true"`

	payload string
}

func (c *stdioConfigCLI) Run(_ context.Context, _ *conga.Session) error {
	reader, err := c.Input.Get().Reader()
	if err != nil {
		return err
	}

	data, err := io.ReadAll(reader)
	c.payload = string(data)

	return err
}

type stdioSessionCaptureCLI struct {
	session *conga.Session
}

func (c *stdioSessionCaptureCLI) Run(_ context.Context, session *conga.Session) error {
	c.session = session

	return nil
}

func TestStdio_FileDashWriterHonorsAppStdout(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[stdioWriterCLI]().
		Stdout(&buffer).
		Args([]string{"--output", "-"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "stdout payload", buffer.String())
}

func TestStdio_FileDashReaderHonorsAppStdin(t *testing.T) {
	t.Parallel()

	cli := new(stdioReaderCLI)
	app := conga.New(cli).
		Stdin(strings.NewReader("stdin payload")).
		Args([]string{"--input", "-"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "stdin payload", cli.payload)
}

func TestStdio_FileSliceElementsHonorStreams(t *testing.T) {
	t.Parallel()

	cli := new(stdioSliceCLI)
	app := conga.New(cli).
		Stdin(strings.NewReader("slice payload")).
		Args([]string{"--input", "-"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "slice payload", cli.payload)
}

func TestStdio_FileContentDashHonorsAppStdin(t *testing.T) {
	t.Parallel()

	cli := new(stdioContentCLI)
	app := conga.New(cli).
		Stdin(strings.NewReader("content payload")).
		Args([]string{"--payload", "-"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "content payload", cli.Payload.Get().String())
}

func TestStdio_EnvTierHonorsStreams(t *testing.T) {
	t.Setenv("CONGA_TEST_STDIO_TEST_INPUT", "-")

	cli := new(stdioEnvCLI)
	app := conga.New(cli).
		Stdin(strings.NewReader("env payload")).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "env payload", cli.payload)
}

func TestStdio_DefaultTierHonorsStreams(t *testing.T) {
	t.Parallel()

	cli := new(stdioDefaultCLI)
	app := conga.New(cli).
		Stdin(strings.NewReader("default payload")).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "default payload", cli.payload)
}

func TestStdio_ConfigTierHonorsStreams(t *testing.T) {
	t.Parallel()

	cli := new(stdioConfigCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"stdio": {"input": "-"}}`))).
		Stdin(strings.NewReader("config payload")).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "config payload", cli.payload)
}

func TestStdio_SessionStreamsFrozenAfterAppMutation(t *testing.T) {
	t.Parallel()

	original := strings.NewReader("first")
	replacement := strings.NewReader("second")

	cli := new(stdioSessionCaptureCLI)
	app := conga.New(cli).
		Stdin(original).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.NotZero(t, cli.session)

	app.Stdin(replacement)

	assert.True(t, cli.session.Stdin() == io.Reader(original))
}

func TestStdio_ExitUsesCurrentStderr(t *testing.T) {
	t.Parallel()

	var (
		initialBuffer bytes.Buffer
		updatedBuffer bytes.Buffer
	)

	app := conga.New[SimpleAppCLI]().
		Stderr(&initialBuffer).
		ExitFunc(func(int) {}).
		Args([]string{"--nope"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)

	app.Stderr(&updatedBuffer)

	app.Exit(err)

	assert.NotEqual(t, "", updatedBuffer.String())
	assert.Equal(t, "", initialBuffer.String())
}
