package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/decoder/toml"
)

// CLI demonstrates the 4-tier 12-Factor precedence: CLI Flag > Env Var > Config File > Struct Default.
type CLI struct {
	ConfigFile conga.ConfigFile `short:"c"`

	Port        conga.OptionalFlag[int]    `default:"8080"                    env:"PORT"                                                     help:"Server port"        placeholder:"PORT" safe:"true" short:"p"`
	Host        conga.OptionalFlag[string] `default:"127.0.0.1"               help:"Server host"                                             safe:"true"`
	Environment conga.OptionalFlag[string] `default:"dev"                     enum:"dev,staging,prod"                                        help:"Target environment" safe:"true"        short:"e"`
	Debug       conga.OptionalFlag[bool]   `default:"false"                   help:"Enable debug logging"                                    short:"d"`
	WorkerID    conga.OptionalFlag[string] `help:"Worker instance identifier" safe:"true"`
	Trace       conga.OptionalFlag[bool]   `env:"-"                           help:"Enable trace output (never environment-bound)"`
	AuthToken   conga.OptionalFlag[string] `config:"-"                        help:"Bearer token (CLI/env only, excluded from config files)"`
	SessionID   string                     // plain field, not a schema element
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("server-precedence").
		Description("12-Factor configuration resolution and 4-tier precedence").
		Version("1.0.0").
		EnableWhy().
		EnvPrefix("SERVER").
		RegisterDecoder(".toml", toml.Decode)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
