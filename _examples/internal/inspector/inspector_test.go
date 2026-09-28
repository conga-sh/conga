package inspector_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

type SampleCLI struct {
	Name    conga.OptionalFlag[string] `default:"world"     help:"User name"   short:"n"`
	Verbose conga.OptionalFlag[bool]   `help:"Verbose mode" short:"v"`
	Port    conga.OptionalFlag[int]    `default:"8080"      help:"Server port" short:"p"`
}

func (s *SampleCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type ChainCLI struct {
	Region conga.OptionalFlag[string]    `default:"eu-west"          help:"Deploy region"`
	Token  conga.OptionalFlag[string]    `help:"Auth token"`
	Create conga.Command[ChainCreateCmd] `summary:"Create a service"`
}

func (c *ChainCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type ChainCreateCmd struct {
	Name  conga.RequiredArg[string]  `help:"Service name"`
	Image conga.OptionalFlag[string] `default:"nginx:latest" help:"Container image" short:"i"`
}

func (c *ChainCreateCmd) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestInspectorFormatting(t *testing.T) {
	t.Parallel()

	cli := new(SampleCLI)
	app := conga.New(cli).
		Name("sample").
		Args([]string{"-n", "Antigravity", "-v"})

	err := app.RunContext(t.Context())
	assert.NoError(t, err)

	var buffer bytes.Buffer
	inspector.Fprint(nil, &buffer, cli)

	output := buffer.String()
	assert.Contains(t, output, "⚙  app")
	assert.Contains(t, output, "FIELD")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "VALUE")
	assert.Contains(t, output, "SOURCE")
	assert.Contains(t, output, "-n, --name")
	assert.Contains(t, output, "Antigravity")
	assert.Contains(t, output, "● cli --name")
	assert.Contains(t, output, "8080")
	assert.Contains(t, output, "· default")
}

func TestInspectorChainView(t *testing.T) {
	t.Parallel()

	cli := new(ChainCLI)
	app := conga.New(cli).
		Name("chainctl").
		Args([]string{"create", "web-api", "-i", "redis:7"})

	err := app.RunContext(t.Context())
	assert.NoError(t, err)

	var buffer bytes.Buffer
	inspector.Fprint(nil, &buffer, cli)

	output := buffer.String()
	assert.Contains(t, output, "⚙  app")
	assert.Contains(t, output, "app · root")
	assert.Contains(t, output, "create · active")
	assert.Contains(t, output, "--region")
	assert.Contains(t, output, "eu-west")
	assert.Contains(t, output, "--token")
	assert.Contains(t, output, "○ unset")
	assert.Contains(t, output, "▸ selected")
	assert.Contains(t, output, "<name>")
	assert.Contains(t, output, "web-api")
	assert.Contains(t, output, "● cli --image")
	assert.Contains(t, output, "redis:7")
	assert.Contains(t, output, "· default")
}

func TestInspectorColorForced(t *testing.T) {
	t.Setenv("CONGA_COLOR", "1")

	cli := new(SampleCLI)
	app := conga.New(cli).
		Name("sample").
		Args([]string{"-n", "Antigravity", "-v"})

	err := app.RunContext(t.Context())
	assert.NoError(t, err)

	var buffer bytes.Buffer
	inspector.Fprint(nil, &buffer, cli)

	output := buffer.String()
	assert.Contains(t, output, "\033[")
	assert.Contains(t, output, "\033[1;36m  ⚙")
	assert.Contains(t, output, "\033[1;36m● cli")
	assert.Contains(t, output, "\033[2m· default")
}

func TestInspectorColorDisabled(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	cli := new(SampleCLI)
	app := conga.New(cli).
		Name("sample").
		Args([]string{"-n", "Antigravity", "-v"})

	err := app.RunContext(t.Context())
	assert.NoError(t, err)

	var buffer bytes.Buffer
	inspector.Fprint(nil, &buffer, cli)

	output := buffer.String()
	assert.False(t, strings.Contains(output, "\033["))
	assert.Contains(t, output, "⚙  app")
	assert.Contains(t, output, "● cli --name")
}
