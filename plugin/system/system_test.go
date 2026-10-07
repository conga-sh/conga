package system_test

import (
	"bytes"
	"context"
	"os/user"
	"syscall"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/plugin/system"
)

func TestSignal(t *testing.T) {
	t.Parallel()

	cli := new(sysTestCLI)
	app := conga.New(cli).
		Args([]string{"-s", "SIGHUP"}).
		Use(system.Signal)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, syscall.SIGHUP, cli.Sig.Get())
}

func TestSignal_InvalidName(t *testing.T) {
	t.Parallel()

	app := conga.New[sysTestCLI]().
		Args([]string{"-s", "NOTASIGNAL"}).
		Use(system.Signal)

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
		Name("app").
		Stdout(&buffer).
		Use(system.Signal).
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
		Use(system.User)

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, curr.Username, cli.Usr.Get().Username)
	assert.Equal(t, curr.Uid, cli.Usr.Get().Uid)
}

func TestUser_UnknownUser(t *testing.T) {
	t.Parallel()

	app := conga.New[sysTestCLI]().
		Args([]string{"-u", "no-such-user-conga-test"}).
		Use(system.User)

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
		Use(system.Group)

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, grp.Name, cli.Grp.Get().Name)
	assert.Equal(t, grp.Gid, cli.Grp.Get().Gid)
}

func TestGroup_UnknownGroup(t *testing.T) {
	t.Parallel()

	app := conga.New[sysTestCLI]().
		Args([]string{"-g", "no-such-group-conga-test"}).
		Use(system.Group)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "no-such-group-conga-test")
}

type sysTestCLI struct {
	Sig conga.OptionalFlag[syscall.Signal] `short:"s"`
	Usr conga.OptionalFlag[*user.User]     `short:"u"`
	Grp conga.OptionalFlag[*user.Group]    `short:"g"`
}

func (c *sysTestCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}
