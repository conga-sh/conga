package system_test

import (
	"bytes"
	"context"
	"errors"
	"os/user"
	"strconv"
	"syscall"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/errlines"
	"github.com/conga-sh/conga/system"
)

func TestSignal(t *testing.T) {
	t.Parallel()

	cli := new(sysTestCLI)
	app := conga.New(cli).
		Args([]string{"-s", "SIGHUP"}).
		Use(system.RegisterSignal, system.RegisterUser, system.RegisterGroup)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, syscall.SIGHUP, cli.Sig.Get())
}

func TestSignal_InvalidName(t *testing.T) {
	t.Parallel()

	app := conga.New[sysTestCLI]().
		Args([]string{"-s", "NOTASIGNAL"}).
		Use(system.RegisterSignal, system.RegisterUser, system.RegisterGroup)

	err := app.Run()

	assert.Error(t, err)
	_, isUsage := errors.AsType[*conga.UsageError](err)
	assert.True(t, isUsage)
	assert.Contains(t, err.Error(), "NOTASIGNAL")
}

func TestSignal_Completion(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[sysTestCLI]().
		EnableCompletion().
		Stdout(&buffer).
		Use(system.RegisterSignal, system.RegisterUser, system.RegisterGroup).
		Args([]string{"__complete", "export", "--sig", ""})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `"value":"SIGTERM"`)
}

func TestSignal_EnumNarrowsCompletion(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[enumSignalCLI]().
		EnableCompletion().
		Stdout(&buffer).
		Args([]string{"__complete", "export", "--signal", ""})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, `{"candidates":[`+
		`{"value":"HUP","display":"HUP","description":"Hangup detected on controlling terminal","style":"yellow","tag":"signals"},`+
		`{"value":"INT","display":"INT","description":"Interrupt from keyboard (Ctrl+C)","style":"yellow","tag":"signals"},`+
		`{"value":"TERM","display":"TERM","description":"Termination signal (graceful shutdown request)","style":"yellow","tag":"signals"}],`+
		`"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},`+
		`"usage":"Signal name or number (e.g. SIGTERM, 9)"}`, buffer.String())
}

func TestSignal_WithoutEnumCompletesEverySignal(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[enumSignalCLI]().
		EnableCompletion().
		Stdout(&buffer).
		Args([]string{"__complete", "export", "--any-signal", ""})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `{"value":"SIGTERM","display":"SIGTERM","description":"Termination signal (graceful shutdown request)","style":"yellow","tag":"signals"}`)
	assert.Contains(t, buffer.String(), `{"value":"KILL","display":"KILL",`)
}

func TestUser(t *testing.T) {
	t.Parallel()

	curr, err := user.Current()
	assert.NoError(t, err)

	cli := new(sysTestCLI)
	app := conga.New(cli).
		Args([]string{"-u", curr.Username}).
		Use(system.RegisterSignal, system.RegisterUser, system.RegisterGroup)

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, curr.Username, cli.Usr.Get().Username)
	assert.Equal(t, curr.Uid, cli.Usr.Get().Uid)
}

func TestUser_UnknownUser(t *testing.T) {
	t.Parallel()

	app := conga.New[sysTestCLI]().
		Args([]string{"-u", "no-such-user-conga-test"}).
		Use(system.RegisterSignal, system.RegisterUser, system.RegisterGroup)

	err := app.Run()

	assert.Error(t, err)
	_, isUsage := errors.AsType[*conga.UsageError](err)
	assert.True(t, isUsage)
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
		Use(system.RegisterSignal, system.RegisterUser, system.RegisterGroup)

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, grp.Name, cli.Grp.Get().Name)
	assert.Equal(t, grp.Gid, cli.Grp.Get().Gid)
}

func TestGroup_UnknownGroup(t *testing.T) {
	t.Parallel()

	app := conga.New[sysTestCLI]().
		Args([]string{"-g", "no-such-group-conga-test"}).
		Use(system.RegisterSignal, system.RegisterUser, system.RegisterGroup)

	err := app.Run()

	assert.Error(t, err)
	_, isUsage := errors.AsType[*conga.UsageError](err)
	assert.True(t, isUsage)
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

type enumSignalCLI struct {
	Signal    conga.OptionalFlag[system.Signal] `enum:"TERM,INT,HUP"`
	AnySignal conga.OptionalFlag[system.Signal]
}

func (c *enumSignalCLI) Run(_ context.Context, _ *conga.Session) error {
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

func TestPID_UnmarshalText_Errors(t *testing.T) {
	t.Parallel()

	t.Run("not an integer keeps the strconv failure as its cause", func(t *testing.T) {
		t.Parallel()

		var pid system.PID

		err := pid.UnmarshalText([]byte("abc"))

		assert.Equal(t, `invalid PID "abc"`, err.Error())
		assert.Equal(t, "must be an integer", errlines.Note(err))

		numberError, ok := errors.AsType[*strconv.NumError](err)

		assert.True(t, ok)
		assert.Equal(t, "Atoi", numberError.Func)
	})

	t.Run("not positive is a root error", func(t *testing.T) {
		t.Parallel()

		var pid system.PID

		err := pid.UnmarshalText([]byte("-4"))

		assert.Equal(t, "invalid PID -4", err.Error())
		assert.Equal(t, "must be a positive integer", errlines.Note(err))
		assert.Equal(t, nil, errors.Unwrap(err))
	})
}
