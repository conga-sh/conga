package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/decoder/hcl"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
)

// CLI demonstrates multi-format config decoding (JSON, TOML, YAML, HCL) and .env file loading.
type CLI struct {
	ConfigFile conga.ConfigFile `short:"c"`

	ServiceName conga.OptionalFlag[string] `default:"api-svc"  env:"SERVICE_NAME"`
	Port        conga.OptionalFlag[int]    `default:"8080"     env:"PORT"                            range:"1..65535"           short:"p"`
	DatabaseURL conga.OptionalFlag[string] `env:"DATABASE_URL" help:"Database connection"            short:"d"`
	MaxWorkers  conga.OptionalFlag[int]    `default:"4"        env:"MAX_WORKERS"                     help:"Worker thread count" range:"1.."`
	APIKey      conga.OptionalFlag[string] `env:"API_KEY"      help:"API key loaded from .env.local"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("multiformat-app").
		Description("Multi-format configuration decoders (JSON, TOML, YAML, HCL) and .env loading").
		Version("1.0.0").
		Dotenv().
		RegisterDecoder(".json", json.Decode).
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		RegisterDecoder(".hcl", hcl.Decode)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
