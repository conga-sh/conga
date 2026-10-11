package main

import (
	"github.com/conga-sh/conga"
)

// CLI is the root of a node administration CLI. The system and network packages
// provide self-completing modular domain types, so the commands below need no
// predictor wiring at all.
type CLI struct {
	System  conga.Command[SystemCmd]  `summary:"Operating system and process operations"`
	Network conga.Command[NetworkCmd] `summary:"Network and web operations"`

	Completion conga.CompletionCommand
	Help       conga.HelpCommand `summary:"Show help for any command"`
	Version    conga.VersionCommand
}
