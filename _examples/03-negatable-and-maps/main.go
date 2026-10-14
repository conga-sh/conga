package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates negatable booleans (--[no-]color), key-value maps, and slice delimiter controls.
type CLI struct {
	// Negatable booleans (last-flag-wins precedence)
	Color    conga.OptionalFlag[bool] `default:"true"           help:"Enable or disable colorized output" negatable:"true" short:"C"`
	Progress conga.OptionalFlag[bool] `help:"Show progress bar" negatable:"true"`

	// Key-value pairs flags: built-in map[string]string and registered custom map[string]int
	EnvVars  conga.OptionalFlag[map[string]string] `help:"Environment variables"             short:"e"`
	Replicas conga.OptionalFlag[map[string]int]    `help:"Service replica counts"            items:"1..4" short:"r"`
	Shards   conga.OptionalFlag[map[string]int]    `help:"Shard counts, semicolon-separated" sep:";"      short:"z"`

	// Slices: preserving literal commas vs comma-splitting vs custom separators
	Headers conga.OptionalFlag[[]string] `help:"HTTP headers (preserves embedded commas)" sep:"none" short:"H"`
	Tags    conga.OptionalFlag[[]string] `short:"t"`
	Hosts   conga.OptionalFlag[[]string] `help:"Semicolon-separated upstream hosts"       sep:";"    short:"o"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("negmap").
		Description("Negatable booleans, maps, and slice delimiter controls").
		Version("1.0.0")

	app.RegisterMap[map[string]int]()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
