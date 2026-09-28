package main

import "github.com/conga-sh/conga"

// CLI demonstrates DashArgs (explicit '--') and RawArgs (bare trailing unparsed args).
type CLI struct {
	Verbose conga.OptionalFlag[bool] `help:"Enable verbose output"                                short:"v"`
	Test    conga.Command[TestCmd]   `summary:"Run test suite with flags forwarded after '--'"`
	Exec    conga.Command[ExecCmd]   `summary:"Execute bare container command (no '--' required)"`
}
