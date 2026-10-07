package conga_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/assertx"
	"github.com/conga-sh/conga/predict"
	"github.com/conga-sh/conga/predict/timezone"
)

func TestRegisterType_ScalarAndPointerFlags(t *testing.T) {
	t.Parallel()

	cli := new(registeredTypeCLI)
	app := conga.New(cli).
		Args([]string{"-u", "uuid-123", "-z", "Europe/Paris"})

	app.RegisterType(parseCustomUUID)
	app.RegisterType(time.LoadLocation, conga.WithPredictor(timezone.Timezones()))

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "uuid-123", cli.ID.Get().raw)
	assert.Equal(t, "Europe/Paris", cli.TZ.Get().String())
	assert.Equal(t, conga.SourceCLI, cli.ID.Source())
	assert.Equal(t, conga.SourceCLI, cli.TZ.Source())
}

func TestRegisterType_DefaultPointerValue(t *testing.T) {
	t.Parallel()

	cli := new(registeredTypeCLI)
	app := conga.New(cli).
		Args([]string{"-u", "uuid-default-test"})

	app.RegisterType(parseCustomUUID)
	app.RegisterType(time.LoadLocation, conga.WithPredictor(timezone.Timezones()))

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "UTC", cli.TZ.Get().String())
	assert.Equal(t, conga.SourceDefault, cli.TZ.Source())
}

func TestRegisterType_SliceFlag(t *testing.T) {
	t.Parallel()

	cli := new(registeredTypeCLI)
	app := conga.New(cli).
		Args([]string{"-u", "uuid-main", "--ids", "uuid-1,uuid-2", "--ids", "uuid-3"})

	app.RegisterType(parseCustomUUID)
	app.RegisterType(time.LoadLocation)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 3, len(cli.IDs.Get()))
	assert.Equal(t, "uuid-1", cli.IDs.Get()[0].raw)
	assert.Equal(t, "uuid-2", cli.IDs.Get()[1].raw)
	assert.Equal(t, "uuid-3", cli.IDs.Get()[2].raw)
}

func TestRegisterType_PositionalArg(t *testing.T) {
	t.Parallel()

	cli := new(registeredTypeCLI)
	app := conga.New(cli).
		Args([]string{"-u", "uuid-flag", "uuid-arg-value"})

	app.RegisterType(parseCustomUUID)
	app.RegisterType(time.LoadLocation)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "uuid-arg-value", cli.ArgID.Get().raw)
	assert.Equal(t, conga.SourceCLI, cli.ArgID.Source())
}

func TestRegisterType_CoercionError_InvalidScalar(t *testing.T) {
	t.Parallel()

	app := conga.New[registeredTypeCLI]().
		Args([]string{"-u", "not-a-valid-uuid"})

	app.RegisterType(parseCustomUUID)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "for flag --id")
}

func TestRegisterType_CoercionError_InvalidPointer(t *testing.T) {
	t.Parallel()

	app := conga.New[registeredTypeCLI]().
		Args([]string{"-u", "uuid-valid", "-z", "Mars/Crater"})

	app.RegisterType(parseCustomUUID)
	app.RegisterType(time.LoadLocation)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "Mars/Crater")
	assert.Contains(t, err.Error(), "for flag --tz")
}

func TestRegisterType_CoercionError_InvalidSliceElement(t *testing.T) {
	t.Parallel()

	app := conga.New[registeredTypeCLI]().
		Args([]string{"-u", "uuid-valid", "--ids", "uuid-1,invalid-elem"})

	app.RegisterType(parseCustomUUID)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "for flag --ids")
}

func TestRegisterType_NilGuards(t *testing.T) {
	t.Parallel()

	app := conga.New[registeredTypeCLI]()

	var nilApp *conga.App[registeredTypeCLI]

	assert.Panics(t, func() {
		nilApp.RegisterType(func(s string) (string, error) { return s, nil })
	})

	assert.Panics(t, func() {
		app.RegisterType[string](nil)
	})
}

func TestRegisterType_LastWriteWinsOverride(t *testing.T) {
	t.Parallel()

	cli := new(registeredTypeCLI)
	app := conga.New(cli).
		Args([]string{"-u", "res-123"})

	app.RegisterType(time.LoadLocation)
	app.RegisterType(func(raw string) (customUUID, error) {
		return customUUID{raw: "initial:" + raw}, nil
	})

	app.RegisterType(func(raw string) (customUUID, error) {
		return customUUID{raw: "overridden:" + raw}, nil
	})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "overridden:res-123", cli.ID.Get().raw)
}

//nolint:paralleltest // serialized: the bash shell iteration drives the completion driver's process-global state
func TestRegisterType_CompletionPredictorInference(t *testing.T) {
	dummyPredictor := predict.Items(
		predict.Item{Value: "Zone/A", Display: "Zone/A", Description: "Alpha"},
		predict.Item{Value: "Zone/B", Display: "Zone/B", Description: "Beta"},
	)

	shells := []string{"bash", "zsh", "fish", "powershell", "elvish", "nushell"}

	//nolint:paralleltest // serialized: the bash iteration drives the completion driver's process-global state
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			app := conga.New[completionTestCLI]()
			app.RegisterType(time.LoadLocation, conga.WithPredictor(dummyPredictor))

			result, _ := app.CompleteForShell(shell, []string{"-t", ""})

			assert.Equal(t, 2, len(result.Items))
			assert.Equal(t, "Zone/A", result.Items[0].Value)
			assert.Equal(t, "Zone/B", result.Items[1].Value)
		})
	}
}

type customUUID struct {
	raw string
}

type registeredTypeCLI struct {
	ID    conga.OptionalFlag[customUUID]     `help:"Custom resource ID"          short:"u"`
	IDs   conga.OptionalFlag[[]customUUID]   `help:"List of custom resource IDs" name:"ids"`
	TZ    conga.OptionalFlag[*time.Location] `default:"UTC"                      help:"Timezone location" short:"z"`
	ArgID conga.OptionalArg[customUUID]      `help:"Positional custom ID"`
}

func (c *registeredTypeCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type completionTestCLI struct {
	Target conga.OptionalFlag[*time.Location] `short:"t"`
}

func parseCustomUUID(s string) (customUUID, error) {
	if !strings.HasPrefix(s, "uuid-") {
		return customUUID{}, errors.New("must start with uuid- prefix")
	}

	return customUUID{raw: s}, nil
}

type domainWorkerMap map[string]int

type registeredMapCLI struct {
	Counts    conga.OptionalFlag[map[string]int]           `short:"c"`
	Durations conga.OptionalFlag[map[string]time.Duration] `short:"d"`
	ArgMap    conga.OptionalArg[map[string]int]
}

func (c *registeredMapCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type registeredDomainMapCLI struct {
	Workers conga.OptionalFlag[domainWorkerMap] `short:"w"`
}

func (c *registeredDomainMapCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type registeredSemicolonMapCLI struct {
	Counts conga.OptionalFlag[map[string]int] `sep:";" short:"c"`
}

func (c *registeredSemicolonMapCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type registeredNoSepMapCLI struct {
	Counts conga.OptionalFlag[map[string]string] `sep:"none" short:"c"`
}

func (c *registeredNoSepMapCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

func TestRegisterMap_FlagsAndArgs(t *testing.T) {
	t.Parallel()

	t.Run("valid flag and positional argument", func(t *testing.T) {
		t.Parallel()

		cli := new(registeredMapCLI)
		app := conga.New(cli).
			Args([]string{"-c", "web=3,worker=5", "-d", "timeout=10s,idle=1m", "db=2,cache=4"})

		app.RegisterMap(map[string]int{})
		app.RegisterMap(map[string]time.Duration{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[string]int{"web": 3, "worker": 5}, cli.Counts.Get())
		assert.Equal(t, map[string]time.Duration{"timeout": 10 * time.Second, "idle": time.Minute}, cli.Durations.Get())
		assert.Equal(t, map[string]int{"db": 2, "cache": 4}, cli.ArgMap.Get())
	})

	t.Run("repeatable flag occurrences", func(t *testing.T) {
		t.Parallel()

		cli := new(registeredMapCLI)
		app := conga.New(cli).
			Args([]string{"-c", "web=3", "-c", "worker=5,db=2"})

		app.RegisterMap(map[string]int{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[string]int{"web": 3, "worker": 5, "db": 2}, cli.Counts.Get())
	})

	t.Run("custom domain map type", func(t *testing.T) {
		t.Parallel()

		cli := new(registeredDomainMapCLI)
		app := conga.New(cli).
			Args([]string{"-w", "frontend=4,backend=8"})

		app.RegisterMap(domainWorkerMap{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, domainWorkerMap{"frontend": 4, "backend": 8}, cli.Workers.Get())
	})

	t.Run("invalid key-value format returns coercion error", func(t *testing.T) {
		t.Parallel()

		app := conga.New[registeredMapCLI]().
			Args([]string{"-c", "bad_entry"})

		app.RegisterMap(map[string]int{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
	})

	t.Run("invalid value coercion returns coercion error", func(t *testing.T) {
		t.Parallel()

		app := conga.New[registeredMapCLI]().
			Args([]string{"-c", "web=not_a_number"})

		app.RegisterMap(map[string]int{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, `value for key "web": invalid integer value "not_a_number" for flag --counts`, err.Error())
	})

	t.Run("custom separator sep:\";\"", func(t *testing.T) {
		t.Parallel()

		cli := new(registeredSemicolonMapCLI)
		app := conga.New(cli).
			Args([]string{"-c", "web=3;worker=5"})

		app.RegisterMap(map[string]int{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[string]int{"web": 3, "worker": 5}, cli.Counts.Get())
	})

	t.Run("sep:\"none\" preserves embedded commas", func(t *testing.T) {
		t.Parallel()

		cli := new(registeredNoSepMapCLI)
		app := conga.New(cli).
			Args([]string{"-c", "hosts=a,b"})

		app.RegisterMap(map[string]string{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"hosts": "a,b"}, cli.Counts.Get())
	})

	t.Run("nil app panics", func(t *testing.T) {
		t.Parallel()

		var nilApp *conga.App[registeredMapCLI]

		assertx.PanicsWithValue(t, "conga: RegisterMap requires a non-nil app", func() {
			nilApp.RegisterMap(map[string]int(nil))
		})
	})
}
