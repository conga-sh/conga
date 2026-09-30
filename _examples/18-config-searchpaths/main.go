package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/hcl"
)

// CLI probes search paths in order and loads only the first existing file.
type CLI struct {
	ConfigFile conga.ConfigFile `short:"c"`

	Region conga.OptionalFlag[string] `default:"us-east-1" help:"Deployment region"`
	Mode   conga.OptionalFlag[string] `default:"standby"   enum:"local,user,system,standby,active" help:"Operating mode"`
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
		RegisterDecoder(".hcl", hcl.RawDecoder).
		ConfigOptions(config.Paths(".", "~/.config/firstfound", "${SYSCONF_DIR:-/etc/firstfound}"))

	app.FatalIfError(app.Run())
}
