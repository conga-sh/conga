package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

var (
	errInvalidStartDate = errors.New("invalid start-date format")
	errInvalidEndDate   = errors.New("invalid end-date format")
	errInvalidDateRange = errors.New("invalid date range")
	errInvalidBackoff   = errors.New("invalid backoff duration")
)

// CLI demonstrates custom programmatic validation using the conga.Validator interface.
type CLI struct {
	StartDate  conga.OptionalFlag[string]        `default:"2026-01-01" help:"Start date (YYYY-MM-DD)"`
	EndDate    conga.OptionalFlag[string]        `default:"2026-01-31" help:"End date (YYYY-MM-DD)"`
	MaxRetries conga.OptionalFlag[int]           `default:"3"          help:"Maximum retry attempts"`
	Backoff    conga.OptionalFlag[time.Duration] `default:"1s"         help:"Backoff between retries"`
}

// Validate executes pure, side-effect-free cross-field semantic validation before any lifecycle hooks.
func (c *CLI) Validate(ctx context.Context, session *conga.Session) error {
	startDate, err := time.Parse(time.DateOnly, c.StartDate.Get())
	if err != nil {
		return fmt.Errorf("%w %q (expected YYYY-MM-DD)", errInvalidStartDate, c.StartDate.Get())
	}

	endDate, err := time.Parse(time.DateOnly, c.EndDate.Get())
	if err != nil {
		return fmt.Errorf("%w %q (expected YYYY-MM-DD)", errInvalidEndDate, c.EndDate.Get())
	}

	if endDate.Before(startDate) {
		return fmt.Errorf("%w: end-date (%s) cannot be earlier than start-date (%s)",
			errInvalidDateRange, c.EndDate.Get(), c.StartDate.Get())
	}

	if c.MaxRetries.Get() > 0 && c.Backoff.Get() <= 0 {
		return fmt.Errorf("%w: backoff duration must be positive when max-retries > 0", errInvalidBackoff)
	}

	return nil
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("validator-demo").
		Description("Dynamic programmatic validation via conga.Validator interface").
		Version("1.0.0")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
