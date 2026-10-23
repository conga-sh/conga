package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// KitchenSinkCLI is the exhaustive reference application: every supported
// flag type, configuration-mapping pattern, and schema directive lives here.
type KitchenSinkCLI struct {
	Config conga.ConfigFile `name:"config-file" short:"c"`

	// Feature toggles declared once on the root. envprefix derives one
	// environment variable per feature (KITCHEN_SINK_FEATURE_<NAME>); each
	// inner field may override that name with its own env tag or opt out with
	// env:"-". The flag tag names the CLI map surface, and config names the
	// config-file section. See feature_commands.go.
	Features conga.Features[kitchenFeatures] `config:"features" envprefix:"KITCHEN_SINK_FEATURE" flag:"feature-gates"`

	// Conventional root flags, all bound from top-level config keys.
	// EnvPrefix("KITCHEN_SINK") (main.go) derives KITCHEN_SINK_<NAME> for every
	// flag without an env tag; an explicit env tag overrides the prefix and
	// env:"-" removes the environment tier entirely.
	Verbosity conga.Counter              `short:"v"`
	Format    conga.OptionalFlag[string] `default:"json"      enum:"json,yaml,toml,hcl" safe:"true"`
	Cache     conga.OptionalFlag[bool]   `default:"true"      help:"Enable cache"       negatable:"true"`
	SecretKey conga.OptionalFlag[string] `config:"api_secret" env:"KITCHEN_SECRET"      help:"API secret (config key remapped, explicit env name)"`
	TempFlag  conga.OptionalFlag[string] `config:"-"          env:"-"                   help:"CLI only (excluded from config files and environment)" safe:"true"`

	// Feature-gated flag: pruned from help, completion, and --why unless the
	// http3 feature is active. Invoking it while inactive is a usage error.
	HTTP3Port conga.OptionalFlag[int] `default:"8443" feature:"http3" help:"QUIC/HTTP3 listener port (experimental)" safe:"true"`

	// app.AllowTags("kong") permits this foreign tag schema-wide; registered
	// keys are matched exactly, so a misspelling still fails schema build
	Metadata conga.OptionalFlag[string] `help:"Arbitrary deployment metadata" kong:"default='none'"`

	Storm conga.OptionalFlag[bool] `help:"Enable storm mode"`

	// conga:"ignore" removes the field from the schema entirely: the command
	// is not runnable, listed in help, or completed
	Session string `conga:"ignore"` // plain fields are inert either way; the directive targets schema elements

	Internal conga.Command[PrimitivesCmd] `conga:"ignore"`

	Types        conga.Command[TypesCmd]        `summary:"Type system matrix (primitives, filesystem, value types)"`
	Configs      conga.Command[ConfigCmd]       `name:"config"                                                             summary:"Configuration mapping patterns"`
	Advanced     conga.Command[AdvancedCmd]     `summary:"Required flags, arguments, trailing args, provenance"`
	FeatureState conga.Command[FeatureStateCmd] `summary:"Resolved feature state, per-key provenance, and lifecycle hooks"`

	// Feature-gated subcommand: the whole command tree below it is pruned from
	// help, completion, and --why unless the http3 feature is active.
	Experimental conga.Command[ExperimentalCmd] `feature:"http3" summary:"Commands gated behind the http3 feature"`

	Help       conga.HelpCommand `summary:"Show help for any command"`
	Version    conga.VersionCommand
	Completion conga.CompletionCommand
}

func (c *KitchenSinkCLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}
