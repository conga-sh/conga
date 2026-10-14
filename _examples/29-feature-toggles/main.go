package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/decoder/yaml"
)

// CLI demonstrates feature toggles: the feature:"..." tag gates flags and whole
// subcommands behind named features. Each gate here hides a capability that is
// experimental or is a legacy escape hatch: an experimental HTTP/3 listener,
// the experimental admin API, and a plaintext escape hatch retired by strict
// TLS. Every feature is carried by one key=value map surface — the
// --feature-gates flag, the SERVE_FEATURES environment variable, or the
// features configuration section — with field defaults as the base. Inactive
// members are hidden from --help, --why, and shell completion, and invoking
// them is a usage error (exit 2).
type CLI struct {
	Config conga.ConfigFile `short:"c"`

	Features conga.Features[experiments] `config:"features" env:"SERVE_FEATURES" flag:"feature-gates"`

	Listen        conga.OptionalFlag[string] `default:":8080"       help:"Listen address"`
	HTTP3         conga.OptionalFlag[bool]   `feature:"http3"       help:"Enable the QUIC/HTTP3 listener (experimental)"`
	AllowInsecure conga.OptionalFlag[bool]   `feature:"!strict-tls" help:"Serve plaintext (development only)"`
	Cache         conga.OptionalFlag[int]    `default:"128"         help:"Response cache size in MB"`

	Admin conga.Command[AdminCmd] `feature:"admin-api" summary:"Experimental admin API"`
}

// Run reports the resolved server configuration and feature state, then starts
// the metrics collector when the metrics gate is active.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	fmt.Fprintln(session.Stdout())

	fmt.Fprintf(session.Stdout(), "listen=%s cache=%d http3=%v insecure=%v\n",
		c.Listen.Get(), c.Cache.Get(), c.HTTP3.Get(), c.AllowInsecure.Get())

	experiments := c.Features.Get()

	fmt.Fprintf(session.Stdout(), "features: http3=%v admin-api=%v strict-tls=%v metrics=%v\n",
		experiments.HTTP3, experiments.AdminAPI, experiments.StrictTLS, experiments.Metrics)

	if experiments.Metrics {
		fmt.Fprintf(session.Stdout(), "metrics: collecting on %s\n", c.Listen.Get())
	}

	return nil
}

// AdminCmd is the experimental admin API gated behind the admin-api feature.
type AdminCmd struct {
	Port conga.OptionalFlag[int] `default:"9090" help:"Admin API port"`
}

// Run executes the admin API command.
func (a *AdminCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), a)

	fmt.Fprintf(session.Stdout(), "admin: API listening on :%d\n", a.Port.Get())

	return nil
}

// experiments declares the application's feature toggles. Each field is one
// feature: the field name becomes the feature name and a bool field is an
// on/off toggle. The default tag seeds the base value.
type experiments struct {
	HTTP3     bool `help:"Experimental HTTP/3 (QUIC) listener"`
	AdminAPI  bool `help:"Experimental admin API"`
	StrictTLS bool `default:"true"                             help:"Require TLS and reject weak ciphers"`
	Metrics   bool `default:"true"                             help:"Expose the metrics endpoint"`
}

func main() {
	app := conga.New[CLI]().
		Name("serve").
		Description("HTTP API server gating experimental features behind code, env, and config gates").
		Version("1.0.0").
		EnableCompletion().
		EnableFeatures().
		EnableWhy().
		EnvPrefix("SERVE").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
