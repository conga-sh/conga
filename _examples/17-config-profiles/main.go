package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/yaml"
)

// CLI merges multiple config files: a base profile plus environment overrides.
// The multi:"true" config-file flag is a slice container: repeated -c flags and
// one comma-separated list all merge with later files winning. Cascading()
// makes the fallback tier below merge every existing file (lowest precedence
// first) instead of stopping at the first match.
type CLI struct {
	ConfigFiles conga.ConfigFile `multi:"true" short:"c"`

	Host     conga.OptionalFlag[string] `default:"localhost" help:"Server host"`
	Port     conga.OptionalFlag[int]    `default:"8080"      help:"Server port"           short:"p"`
	LogLevel conga.OptionalFlag[string] `default:"info"      enum:"debug,info,warn,error"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("profiles").
		Description("Multi-file config merging via conga.ConfigFile with multi:\"true\"").
		Version("1.0.0").
		RegisterDecoder(".yaml", yaml.RawDecoder).
		RegisterDecoder(".yml", yaml.RawDecoder).
		ConfigOptions(config.Paths("deploy/prod.yaml", "deploy/base.yaml"), config.Cascading())

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
