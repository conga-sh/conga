package system_test

import (
	"bytes"
	"context"
	"os/user"
	"syscall"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/system"
)

func TestSignal(t *testing.T) {
	t.Parallel()

	cli := new(sysTestCLI)
	app := conga.New(cli).
		Args([]string{"-s", "SIGHUP"}).
		Use(system.RegisterSignal)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, syscall.SIGHUP, cli.Sig.Get())
}

func TestSignal_InvalidName(t *testing.T) {
	t.Parallel()

	app := conga.New[sysTestCLI]().
		Args([]string{"-s", "NOTASIGNAL"}).
		Use(system.RegisterSignal)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "NOTASIGNAL")
}

func TestSignal_Completion(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[sysTestCLI]().
		EnableCompletion().
		Stdout(&buffer).
		Use(system.RegisterSignal).
		Args([]string{"__complete", "export", "--sig", ""})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `"value":"SIGTERM"`)
}

func TestUser(t *testing.T) {
	t.Parallel()

	curr, err := user.Current()
	assert.NoError(t, err)

	cli := new(sysTestCLI)
	app := conga.New(cli).
		Args([]string{"-u", curr.Username}).
		Use(system.RegisterUser)

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, curr.Username, cli.Usr.Get().Username)
	assert.Equal(t, curr.Uid, cli.Usr.Get().Uid)
}

func TestUser_UnknownUser(t *testing.T) {
	t.Parallel()

	app := conga.New[sysTestCLI]().
		Args([]string{"-u", "no-such-user-conga-test"}).
		Use(system.RegisterUser)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "no-such-user-conga-test")
}

func TestGroup(t *testing.T) {
	t.Parallel()

	curr, err := user.Current()
	assert.NoError(t, err)

	grp, err := user.LookupGroupId(curr.Gid)
	assert.NoError(t, err)

	cli := new(sysTestCLI)
	app := conga.New(cli).
		Args([]string{"-g", grp.Name}).
		Use(system.RegisterGroup)

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, grp.Name, cli.Grp.Get().Name)
	assert.Equal(t, grp.Gid, cli.Grp.Get().Gid)
}

func TestGroup_UnknownGroup(t *testing.T) {
	t.Parallel()

	app := conga.New[sysTestCLI]().
		Args([]string{"-g", "no-such-group-conga-test"}).
		Use(system.RegisterGroup)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "no-such-group-conga-test")
}

func TestDefaultTypeHelp(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[sysHelpCLI]().
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()
	assert.NoError(t, err)

	output := buffer.String()

	for _, testCase := range []struct {
		name string
		want string
	}{
		{"EnvVar", "Environment variable name (e.g. PATH)"},
		{"Group", "System group name"},
		{"Locale", "Locale identifier (e.g. en_US.UTF-8)"},
		{"PID", "Process ID (a positive integer)"},
		{"Process", "Process executable name (e.g. postgres)"},
		{"Signal", "Signal name or number (e.g. SIGTERM, 9)"},
		{"User", "System username"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, output, testCase.want)
		})
	}
}

type sysHelpCLI struct {
	Env     conga.OptionalFlag[system.EnvVar]
	Group   conga.OptionalFlag[system.Group]
	Locale  conga.OptionalFlag[system.Locale]
	PID     conga.OptionalFlag[system.PID]
	Process conga.OptionalFlag[system.Process]
	Signal  conga.OptionalFlag[system.Signal]
	User    conga.OptionalFlag[system.User]
}

func (c *sysHelpCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type sysTestCLI struct {
	Sig conga.OptionalFlag[syscall.Signal] `short:"s"`
	Usr conga.OptionalFlag[*user.User]     `short:"u"`
	Grp conga.OptionalFlag[*user.Group]    `short:"g"`
}

func (c *sysTestCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}
