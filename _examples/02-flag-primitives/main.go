package main

import (
	"context"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates primitive flag types, counters, aliases, sections, and hidden flags.
type CLI struct {
	// Logging Options
	Verbosity conga.Counter              `help:"Increase verbosity level (-v, -vv, -vvv)" section:"Logging Options"   short:"v"`
	Debug     conga.OptionalFlag[bool]   `aliases:"dbg"                                   help:"Enable debug logging" section:"Logging Options" short:"d"`
	LogFormat conga.OptionalFlag[string] `default:"text"                                  section:"Logging Options"`

	// Network Options
	Port    conga.OptionalFlag[int]           `default:"8080" help:"Server port"             section:"Network Options" short:"p"`
	Rate    conga.OptionalFlag[float64]       `default:"1.5"  help:"Request rate multiplier" section:"Network Options"`
	Timeout conga.OptionalFlag[time.Duration] `default:"30s"  help:"Request timeout"         section:"Network Options"`

	// Advanced / Hidden
	SecretKey conga.OptionalFlag[string] `help:"Internal secret key" hidden:"true"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("flag-primitives").
		Description("Primitive flag types, counters, sections, and aliases").
		Version("1.0.0").
		EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
