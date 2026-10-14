package timezone_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/timezone"
)

//nolint:paralleltest // timezone package tests mutate the process-global ZONEINFO environment variable.
func TestLocation(t *testing.T) {
	cli := new(timezoneTestCLI)
	app := conga.New(cli).
		Args([]string{"-z", "America/New_York"}).
		Use(timezone.RegisterLocation)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "America/New_York", cli.Zone.Get().String())
}

//nolint:paralleltest // timezone package tests mutate the process-global ZONEINFO environment variable.
func TestLocation_InvalidZone(t *testing.T) {
	app := conga.New[timezoneTestCLI]().
		Args([]string{"-z", "Mars/Crater"}).
		Use(timezone.RegisterLocation)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "Mars/Crater")
	assert.Contains(t, err.Error(), "for flag --zone")
}

//nolint:paralleltest // timezone package tests mutate the process-global ZONEINFO environment variable.
func TestLocation_Completion(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[timezoneTestCLI]().
		EnableCompletion().
		Stdout(&buffer).
		Use(timezone.RegisterLocation).
		Args([]string{"__complete", "export", "--zone", ""})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `"value":"America/New_York"`)
}

type timezoneTestCLI struct {
	Zone conga.OptionalFlag[*time.Location] `short:"z"`
}

func (c *timezoneTestCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}
