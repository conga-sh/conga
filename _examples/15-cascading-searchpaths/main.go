package main

import (
	"context"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/yaml"
)

// CLI demonstrates cascading configuration merges (local > user > system precedence).
type CLI struct {
	ConfigFile conga.ConfigFile `multi:"true" short:"c"`

	APIEndpoint conga.OptionalFlag[string]        `default:"https://api.default.io" name:"api-endpoint"`
	Timeout     conga.OptionalFlag[time.Duration] `default:"10s"                    help:"Request timeout"`
	Theme       conga.OptionalFlag[string]        `default:"dark"                   enum:"dark,light,solarized,system"`
	Telemetry   conga.OptionalFlag[bool]          `default:"true"                   help:"Enable telemetry reporting"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("cascading-app").
		Description("Cascading multi-tier configuration search paths (local > user > system)").
		Version("1.0.0").
		RegisterDecoder(".yaml", yaml.RawDecoder).
		RegisterDecoder(".yml", yaml.RawDecoder).
		ConfigOptions(config.Paths(".", "~/.config/cascading-app", "${SYSCONF_DIR:-/etc/cascading-app}"), config.Cascading())

	app.FatalIfError(app.Run())
}
