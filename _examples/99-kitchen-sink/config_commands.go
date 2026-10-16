package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// ConfigCmd groups the configuration-mapping patterns. The Config root
// flag keeps the --config-file name; this subcommand tree maps to [config]*
// sections in config files.
type ConfigCmd struct {
	Context conga.OptionalFlag[string] `default:"default" help:"Active configuration context" safe:"true"`

	Maps   conga.Command[MapConfigCmd]    `summary:"Native map vs array-of-strings encodings"`
	Nested conga.Command[NestedConfigCmd] `summary:"Deeply nested config sections"`
}

func (c *ConfigCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[KitchenSinkCLI](), c)

	return nil
}

// MapConfigCmd demonstrates the two supported map encodings in config files:
// a native map value and an array of KEY=VALUE strings.
type MapConfigCmd struct {
	Vars   conga.OptionalFlag[map[string]string] `help:"Environment variables (native map encoding)"`
	Labels conga.OptionalFlag[map[string]string] `help:"Resource labels (array-of-strings encoding)" safe:"true"`
}

func (c *MapConfigCmd) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[KitchenSinkCLI]()
	configCmd := root.Configs.Get()
	inspector.Print(session, root, configCmd, c)

	return nil
}

// NestedConfigCmd demonstrates deeply nested config sections ([config.nested]).
type NestedConfigCmd struct {
	Zone  conga.OptionalFlag[string] `default:"us-east-1a" help:"Availability zone" safe:"true"`
	Count conga.OptionalFlag[int]    `default:"3"          help:"Node count"        safe:"true"`
}

func (c *NestedConfigCmd) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[KitchenSinkCLI]()
	configCmd := root.Configs.Get()
	inspector.Print(session, root, configCmd, c)

	return nil
}
