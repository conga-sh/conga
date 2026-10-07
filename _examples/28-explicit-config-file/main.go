package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/decoder/yaml"
)

// CLI demonstrates conga.ConfigExplicit(...): one programmatic concrete config file
// with fatal-if-missing semantics. Only an explicit --config-file flag
// outranks it; the <PREFIX>_CONFIG environment variable and the search tier
// are not consulted while the option is declared.
type CLI struct {
	ConfigFile conga.ConfigFile `short:"c"`

	Host conga.OptionalFlag[string] `default:"localhost" help:"Server host"`
	Port conga.OptionalFlag[int]    `default:"8080"      help:"Server port" short:"p"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("explicit-config").
		Description("Programmatic explicit config file (fatal-if-missing)").
		Version("1.0.0").
		EnvPrefix("EXPLICIT").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit("bootstrap.yaml"))

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
