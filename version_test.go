package conga_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type TestVersionRootCLI struct {
	Verbose conga.OptionalFlag[bool]             `help:"Verbose logging"     short:"v"`
	Cluster conga.Command[TestVersionClusterCmd] `summary:"Cluster commands"`
	Version conga.VersionCommand
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
	Version conga.VersionCommand
}

type TestVersionStatusCmd struct{}

func (s *TestVersionStatusCmd) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type TestRootJSONCLI struct {
	JSON conga.OptionalFlag[bool] `help:"Output in JSON format"`
}

func (c *TestRootJSONCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type TestSessionVersionCLI struct{}

func (c *TestSessionVersionCLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), session.Version())

	return nil
}

type UserVersionFlagCLI struct {
	Version conga.OptionalFlag[string] `help:"Custom version string"`
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
	assert.Contains(t, buffer.String(), "testapp has version 2.1.0 built with ")
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
	assert.Contains(t, buffer.String(), "testapp has version dev built with ")
}

type TestVersionAliasCLI struct {
	Version conga.VersionCommand `aliases:"v"`
}

func TestVersionCommand_AliasRouting(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionAliasCLI]().
		Name("testapp").
		Version("2.1.0").
		Args([]string{"v"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "testapp:\n")
	assert.Contains(t, buffer.String(), " Version:    2.1.0")
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
		Version("2.1.0").
		Args([]string{"--version"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.NotContains(t, buffer.String(), "Go version:")
}

// TestVersion_NamePrecedence pins the display name: a name configured with
// App.Name always wins over the invoked binary, while an unnamed app falls
// back to the invoked binary name.
//
//nolint:paralleltest // Mutates global os.Args for invocation name testing.
func TestVersion_NamePrecedence(t *testing.T) {
	oldArgs := os.Args

	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{"/usr/local/bin/invoked-binary"}

	t.Run("configured name wins over the invoked binary", func(t *testing.T) {
		var buffer bytes.Buffer

		app := conga.New[TestVersionRootCLI]().
			Name("tool").
			Version("1.0.0").
			Args([]string{"--version"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "tool has version 1.0.0")
		assert.NotContains(t, buffer.String(), "invoked-binary")
	})

	t.Run("unnamed app uses the invoked binary", func(t *testing.T) {
		var buffer bytes.Buffer

		app := conga.New[TestVersionRootCLI]().
			Version("1.0.0").
			Args([]string{"--version"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "invoked-binary has version 1.0.0")
	})

	t.Run("runtime error prefix uses the configured name", func(t *testing.T) {
		var buffer bytes.Buffer

		app := conga.New[TestVersionRootCLI]().
			Name("tool").
			Args([]string{}).
			Stderr(&buffer).
			ExitFunc(func(int) {})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Contains(t, buffer.String(), "tool: error: ")
		assert.NotContains(t, buffer.String(), "invoked-binary")
	})
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
	assert.Contains(t, buffer.String(), "testapp has version 3.0.0 built with ")
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
		Version("3.0.0").
		Args([]string{"version", "--json=banana"}).
		Stdout(&buffer)

	err := app.Run()

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Equal(t, `invalid boolean value "banana", expected true or false (or 1 or 0) for flag --json`, err.Error())
	assert.Equal(t, "", buffer.String())
}

func TestVersionCommand_JSONEmptyValueRejected(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Version("3.0.0").
		Args([]string{"version", "--json="}).
		Stdout(&buffer)

	err := app.Run()

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Equal(t, `invalid boolean value "", expected true or false (or 1 or 0) for flag --json`, err.Error())
	assert.Equal(t, "", buffer.String())
}

func TestVersionCommand_AncestorJSONFlagCollides(t *testing.T) {
	t.Parallel()

	app := conga.New[TestVersionJSONCollisionCLI]().
		Name("tool").
		Version("3.0.0")

	err := app.Run()

	assert.Error(t, err)
	assert.Equal(t, `struct TestVersionJSONCollisionCLI field Version: duplicate flag --json (note: already declared by struct TestVersionJSONCollisionCLI field JSON on an ancestor command; the built-in version command owns --json; hint: rename the ancestor flag or remove the VersionCommand field)`, err.Error())
}

func TestRootJSONFlagWithoutVersionCommandBuilds(t *testing.T) {
	t.Parallel()

	var cli TestRootJSONCLI

	app := conga.New(&cli).
		Name("tool").
		Args([]string{"--json"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.JSON.Get())
}

func TestHelp_ShowsVersionFlagWhenVersionSet(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
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
	assert.Contains(t, buffer.String(), "testapp has version dev built with ")
}

func TestVersion_CollisionWithoutDisableVersionFlag(t *testing.T) {
	t.Parallel()

	app := conga.New[UserVersionFlagCLI]().
		Version("1.0.0").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.Equal(t, "struct UserVersionFlagCLI field Version: duplicate flag --version (note: already declared by the built-in --version flag; hint: use app.DisableVersionFlag() to override)", err.Error())
}

func TestVersion_CustomFlagCollisionWithoutDisableVersionFlag(t *testing.T) {
	t.Parallel()

	app := conga.New[UserVersionFlagCLI]().
		Args([]string{"--version", "custom-v1"})

	err := app.Run()

	assert.Error(t, err)
	assert.Equal(t, "struct UserVersionFlagCLI field Version: duplicate flag --version (note: already declared by the built-in --version flag; hint: use app.DisableVersionFlag() to override)", err.Error())
}

func TestVersion_CustomFlagWithDisableVersionFlag(t *testing.T) {
	t.Parallel()

	cli := new(UserVersionFlagCLI)
	app := conga.New(cli).
		DisableVersionFlag().
		Args([]string{"--version", "custom-v1"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "custom-v1", cli.Version.Get())
}

// TestVersionFlag_PrecedenceOverParseErrors pins that a built-in --version
// renders the version line even when another token is an unknown flag.
func TestVersionFlag_PrecedenceOverParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag after version", args: []string{"--version", "--bogus"}},
		{name: "unknown flag before version", args: []string{"--bogus", "--version"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer

			app := conga.New[TestVersionRootCLI]().
				Name("testapp").
				Version("2.1.0").
				Args(test.args).
				Stdout(&buffer)

			err := app.Run()

			assert.NoError(t, err)
			assert.True(t, strings.HasPrefix(buffer.String(), "testapp has version 2.1.0 built with "))
		})
	}
}

// TestVersionFlag_ShortPrecedenceOverParseErrors pins that the configured -V
// rune outranks a parse error like --version does.
func TestVersionFlag_ShortPrecedenceOverParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"-V", "--bogus"}},
		{name: "missing flag value", args: []string{"-V", "--name"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer

			app := conga.New[versionPrecedenceCLI]().
				Name("testapp").
				Version("2.1.0").
				VersionShortFlag('V').
				Args(test.args).
				Stdout(&buffer)

			err := app.Run()

			assert.NoError(t, err)
			assert.True(t, strings.HasPrefix(buffer.String(), "testapp has version 2.1.0 built with "))
		})
	}
}

// versionPrecedenceCLI is a runnable root with a value flag, so a version
// request can meet a missing value and a false version value runs the root.
type versionPrecedenceCLI struct {
	Name conga.OptionalFlag[string] `help:"Service name"`
	Ran  bool
}

func (c *versionPrecedenceCLI) Run(context.Context, *conga.Session) error {
	c.Ran = true

	return nil
}

// TestVersionFlag_UserVersionFlagGetsNoPrecedence pins that a user flag
// reusing the freed version name is an ordinary flag, so an unknown flag still
// fails.
func TestVersionFlag_UserVersionFlagGetsNoPrecedence(t *testing.T) {
	t.Parallel()

	app := conga.New[UserVersionFlagCLI]().
		DisableVersionFlag().
		Args([]string{"--bogus", "--version", "custom-v1"})

	err := app.Run()

	assertUsageError(t, err)
	assert.Equal(t, "unknown flag: --bogus", err.Error())
}

// TestVersionFlag_FalseValueIsNoRequest pins that the built-in version flag
// honors its bool value: a false value runs the command.
func TestVersionFlag_FalseValueIsNoRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "long flag zero", args: []string{"--version=0"}},
		{name: "short flag false", args: []string{"-V=false"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer

			cli := new(versionPrecedenceCLI)
			app := conga.New(cli).
				Name("testapp").
				Version("2.1.0").
				VersionShortFlag('V').
				Args(test.args).
				Stdout(&buffer)

			err := app.Run()

			assert.NoError(t, err)
			assert.True(t, cli.Ran)
			assert.Equal(t, "", buffer.String())
		})
	}
}

func TestVersionFlag_InvalidValueFails(t *testing.T) {
	t.Parallel()

	app := conga.New[versionPrecedenceCLI]().
		Name("testapp").
		Args([]string{"--version=maybe"})

	err := app.Run()

	assertUsageError(t, err)
	assert.Equal(t, `invalid boolean value "maybe", expected true or false (or 1 or 0) for flag --version`, err.Error())
}

// TestVersionCommand_HelpWinsOverInvalidJSON pins that --help on the version
// command renders its help screen before --json is coerced.
func TestVersionCommand_HelpWinsOverInvalidJSON(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestVersionRootCLI]().
		Name("testapp").
		Version("2.1.0").
		Args([]string{"version", "--json=bad", "--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "Show version information\n\nUsage:\n  testapp version [flags]\n\nFlags:\n      --json  Render machine-readable JSON output\n      --help  Show help\n", buffer.String())
}
