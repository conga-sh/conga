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

	// Conventional root flags, all bound from top-level config keys
	Verbosity conga.Counter              `short:"v"`
	Format    conga.OptionalFlag[string] `default:"json"      enum:"json,yaml,toml,hcl"                    safe:"true"`
	Cache     conga.OptionalFlag[bool]   `default:"true"      help:"Enable cache"                          negatable:"true"`
	SecretKey conga.OptionalFlag[string] `config:"api_secret" help:"API secret (config key remapped)"      safe:"true"`
	TempFlag  conga.OptionalFlag[string] `config:"-"          help:"CLI only (excluded from config files)" safe:"true"`

	// app.AllowTags("kong") permits this foreign tag schema-wide
	Metadata conga.OptionalFlag[string] `help:"Arbitrary deployment metadata" kong:"default='none'" safe:"true"`

	// The conga:"allow-unknown-tags" directive on any root field permits the
	// foreign tag on this field; recurse widens the permission into every
	// nested command (the targeted alternative to a blanket AllowTags list)
	Storm conga.OptionalFlag[bool] `conga:"allow-unknown-tags,recurse" help:"Enable storm mode" storm:"enabled"`

	// conga:"ignore" removes the field from the schema entirely: the command
	// is not runnable, listed in help, or completed
	Session string `conga:"ignore"` // plain fields are inert either way; the directive targets schema elements

	Internal conga.Command[PrimitivesCmd] `conga:"ignore"`

	Types    conga.Command[TypesCmd]    `summary:"Type system matrix (primitives, filesystem, value types)"`
	Configs  conga.Command[ConfigCmd]   `name:"config"                                                      summary:"Configuration mapping patterns"`
	Advanced conga.Command[AdvancedCmd] `summary:"Required flags, arguments, trailing args, provenance"`

	Help       conga.HelpCommand `summary:"Show help for any command"`
	Version    conga.VersionCommand
	Completion conga.CompletionCommand
}

func (c *KitchenSinkCLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}
