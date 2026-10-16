package main

import (
	"context"
	"syscall"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates pluggable type parsers: bindings for standard library
// types (*time.Location, syscall.Signal) mounted via app.Use, and a custom
// foreign type wired via App.RegisterType.
type CLI struct {
	TZ       conga.OptionalFlag[*time.Location]   `default:"UTC"                                        help:"Primary deployment timezone"          short:"z"`
	Replicas conga.OptionalFlag[[]*time.Location] `help:"Replica timezones for multi-region deployment" name:"replica-tz"`
	Signal   conga.OptionalFlag[syscall.Signal]   `default:"TERM"                                       help:"Shutdown signal to forward to daemon" short:"s"`
	Load     conga.OptionalFlag[Percent]          `help:"Current load percentage (e.g. 75%)"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}
