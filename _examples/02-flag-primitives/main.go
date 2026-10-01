package main

import (
	"context"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates primitive flag types, counters, aliases, groups, and hidden flags.
type CLI struct {
	// Logging Options
	Verbosity conga.Counter              `group:"Logging Options" help:"Increase verbosity level (-v, -vv, -vvv)" short:"v"`
	Debug     conga.OptionalFlag[bool]   `aliases:"dbg"           group:"Logging Options"                         help:"Enable debug logging" short:"d"`
	LogFormat conga.OptionalFlag[string] `default:"text"          group:"Logging Options"`

	// Network Options
	Port    conga.OptionalFlag[int]           `default:"8080" group:"Network Options" help:"Server port"             short:"p"`
	Rate    conga.OptionalFlag[float64]       `default:"1.5"  group:"Network Options" help:"Request rate multiplier"`
	Timeout conga.OptionalFlag[time.Duration] `default:"30s"  group:"Network Options" help:"Request timeout"`

	// Advanced / Hidden
	SecretKey conga.OptionalFlag[string] `help:"Internal secret key" hidden:"true"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("prims").
		Description("Primitive flag types, counters, groups, and aliases").
		Version("1.0.0").
		EnableCompletion()

	app.FatalIfError(app.Run())
}
