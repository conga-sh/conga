package conga_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type TestVersionRootCLI struct {
	Verbose conga.OptionalFlag[bool]             `help:"Verbose logging"             short:"v"`
	Cluster conga.Command[TestVersionClusterCmd] `summary:"Cluster commands"`
	Version conga.VersionCommand                 `summary:"Show version information"`
}

type TestVersionClusterCmd struct {
	Node conga.Command[TestVersionNodeCmd] `summary:"Node commands"`
}

type TestVersionNodeCmd struct {
	Action conga.OptionalFlag[string] `help:"Node action"`
}

func (n *TestVersionNodeCmd) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type TestVersionJSONCollisionCLI struct {
	JSON    conga.OptionalFlag[bool]            `help:"Output in JSON format"`
	Status  conga.Command[TestVersionStatusCmd] `summary:"Check status"`
	Version conga.VersionCommand                `summary:"Show version information"`
}

type TestVersionStatusCmd struct{}

func (s *TestVersionStatusCmd) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type TestSessionVersionCLI struct{}

func (c *TestSessionVersionCLI) Run(ctx context.Context, session *conga.Session) error {
	session.Version()

	return nil
}

type UserVersionFlagCLI struct {
	Version conga.OptionalFlag[string] `help:"Custom version string" name:"version"`
}

func (u *UserVersionFlagCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestVersionFlag_Root(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("2.1.0").
		Args([]string{"--version"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "testapp has version")
}

func TestVersionFlag_BuildInfoWithoutVersion(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Args([]string{"--version"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "testapp has version")
}

func TestVersionCommand_Root(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("2.1.0").
		Args([]string{"version"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "testapp:\n")
	assert.Contains(t, buffer.String(), " Version:    2.1.0")
	assert.Contains(t, buffer.String(), "Go version:")
}

func TestVersionFlag_SummaryOmitsBuildDetails(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("2.1.0").
		Args([]string{"--version"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.NotContains(t, buffer.String(), "Go version:")
}

func TestVersionFlag_RendersToConfiguredStdout(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("3.0.0").
		Args([]string{"--version"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "testapp has version 3.0.0")
}

func TestVersionCommand_RendersToConfiguredStdout(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("3.0.0").
		Args([]string{"version"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "testapp:\n")
	assert.Contains(t, buffer.String(), " Version:    3.0.0")
	assert.Contains(t, buffer.String(), "Go version:")
}

func TestVersionCommand_JSON(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("3.0.0").
		Args([]string{"version", "--json"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()
	assert.Contains(t, output, `{"version":"3.0.0"`)
	assert.Contains(t, output, `"modified":false`)
	assert.NotContains(t, output, "Go version:")
}

func TestVersionCommand_JSONFalseRendersDetailed(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("3.0.0").
		Args([]string{"version", "--json=false"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "testapp:\n")
	assert.NotContains(t, buffer.String(), `{"version"`)
}

func TestVersionCommand_JSONInvalidValue(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("3.0.0").
		Args([]string{"version", "--json=banana"}).
		Stdout(&buffer)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Equal(t, `invalid boolean value "banana", expected true or false for flag --json`, err.Error())
	assert.Equal(t, "", buffer.String())
}

func TestVersionCommand_JSONEmptyValueTreatedAsTrue(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("3.0.0").
		Args([]string{"version", "--json="}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `{"version":"3.0.0"`)
}

func TestVersionCommand_AncestorJSONFlagDoesNotShadow(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	cli := new(TestVersionJSONCollisionCLI)
	app := conga.New(cli).
		Name("testapp").
		Version("3.0.0").
		Args([]string{"version", "--json"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `{"version":"3.0.0"`)
	assert.True(t, cli.Version.JSON)
	assert.False(t, cli.JSON.Get())
}

func TestVersionCommand_RootJSONFlagUnaffected(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	cli := new(TestVersionJSONCollisionCLI)
	app := conga.New(cli).
		Name("testapp").
		Version("3.0.0").
		Args([]string{"--json", "status"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.JSON.Get())
	assert.False(t, cli.Version.JSON)
}

func TestHelp_ShowsVersionFlagWhenVersionSet(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("2.1.0").
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()
	assert.Contains(t, output, "--version")
	assert.Contains(t, output, "Show version")

	// --help and --version must close the Flags section.
	assert.True(t, strings.Index(output, "--verbose") < strings.Index(output, "--help"))
	assert.True(t, strings.Index(output, "--help") < strings.Index(output, "--version"))
}

func TestHelp_ShowsVersionFlagWithoutVersion(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestHelpRootCLI]().
		Name("testapp").
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "--version")
	assert.Contains(t, buffer.String(), "--help")
}

func TestSessionVersion_BuildInfoWithoutVersion(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestSessionVersionCLI]().
		Name("testapp").
		Args([]string{}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "testapp has version")
}

func TestVersion_CollisionWithoutDisableVersionFlag(t *testing.T) {
	t.Parallel()

	app := conga.New[UserVersionFlagCLI]().
		Name("testapp").
		Version("1.0.0").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
}

func TestVersion_CustomFlagCollisionWithoutDisableVersionFlag(t *testing.T) {
	t.Parallel()

	app := conga.New[UserVersionFlagCLI]().
		Name("testapp").
		Args([]string{"--version", "custom-v1"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
}

func TestVersion_CustomFlagWithDisableVersionFlag(t *testing.T) {
	t.Parallel()

	cli := new(UserVersionFlagCLI)
	app := conga.New(cli).
		Name("testapp").
		DisableVersionFlag().
		Args([]string{"--version", "custom-v1"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "custom-v1", cli.Version.Get())
}
