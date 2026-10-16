package main

import (
	"context"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/decoder/yaml"
)

// CLI demonstrates cascading configuration merges (local > user > system precedence).
type CLI struct {
	ConfigFile conga.ConfigFile `multi:"true" short:"c"`

	APIEndpoint conga.OptionalFlag[string]        `default:"https://api.default.io" safe:"true"`
	Timeout     conga.OptionalFlag[time.Duration] `default:"10s"                    help:"Request timeout"             safe:"true"`
	Theme       conga.OptionalFlag[string]        `default:"dark"                   enum:"dark,light,solarized,system" safe:"true"`
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
		EnableWhy().
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigPaths(".", "~/.config/cascading-app", "${SYSCONF_DIR:-/etc/cascading-app}"), conga.ConfigCascading())

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
