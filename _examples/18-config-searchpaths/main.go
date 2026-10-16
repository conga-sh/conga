package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/decoder/hcl"
)

// CLI probes search paths in order and loads only the first existing file.
type CLI struct {
	ConfigFile conga.ConfigFile `short:"c"`

	Region conga.OptionalFlag[string] `default:"us-east-1" help:"Deployment region"                safe:"true"`
	Mode   conga.OptionalFlag[string] `default:"standby"   enum:"local,user,system,standby,active" help:"Operating mode" safe:"true"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("firstfound").
		Description("Single-file config mode: first existing search path wins").
		Version("1.0.0").
		EnableWhy().
		RegisterDecoder(".hcl", hcl.Decode).
		ConfigOptions(conga.ConfigPaths(".", "~/.config/firstfound", "${SYSCONF_DIR:-/etc/firstfound}"))

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
