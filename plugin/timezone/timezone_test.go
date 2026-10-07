package timezone_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/plugin/timezone"
)

func TestLocation(t *testing.T) {
	t.Parallel()

	cli := new(timezoneTestCLI)
	app := conga.New(cli).
		Args([]string{"-z", "America/New_York"}).
		Use(timezone.Location)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "America/New_York", cli.Zone.Get().String())
}

func TestLocation_InvalidZone(t *testing.T) {
	t.Parallel()

	app := conga.New[timezoneTestCLI]().
		Args([]string{"-z", "Mars/Crater"}).
		Use(timezone.Location)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "Mars/Crater")
	assert.Contains(t, err.Error(), "for flag --zone")
}

func TestLocation_Completion(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[timezoneTestCLI]().
		EnableCompletion().
		Name("app").
		Stdout(&buffer).
		Use(timezone.Location).
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
