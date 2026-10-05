package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates opt-in built-in subcommands (HelpCommand, VersionCommand)
// alongside custom commands that render through the session primitives.
type CLI struct {
	Status  conga.Command[StatusCmd] `summary:"Check system status"`
	About   conga.Command[AboutCmd]  `summary:"Print the version summary via the session renderer"`
	Guide   conga.Command[GuideCmd]  `summary:"Render this command's help screen via the session renderer"`
	Syntax  conga.Command[SyntaxCmd] `summary:"Render the syntax usage summary via the session renderer"`
	Help    conga.HelpCommand        `summary:"Show help for any command"`
	Version conga.VersionCommand     `summary:"Display detailed version and build information"`
}

type StatusCmd struct {
	Detailed conga.OptionalFlag[bool] `help:"Show detailed subsystem diagnostics" short:"d"`
}

// Description enriches the status help screen with a multi-line overview; the
// summary tag stays the one-line synopsis in the parent's command list.
func (s *StatusCmd) Description() string {
	return "Reports the health of every subsystem infomgr manages.\n\nPass --detailed for per-subsystem diagnostics."
}

// Examples renders usage examples under the Examples: heading.
func (s *StatusCmd) Examples() string {
	return "infomgr status\ninfomgr status --detailed"
}

func (s *StatusCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	return nil
}

type AboutCmd struct{}

// Run prints the version summary through the same renderer the built-in
// --version flag uses, without touching the configured streams directly.
func (a *AboutCmd) Run(_ context.Context, session *conga.Session) error {
	session.Version()

	return nil
}

type GuideCmd struct{}

// Run renders the active command's help screen to stdout.
func (g *GuideCmd) Run(_ context.Context, session *conga.Session) error {
	session.Help()

	return nil
}

type SyntaxCmd struct{}

// Run renders the syntax usage summary to stderr.
func (s *SyntaxCmd) Run(_ context.Context, session *conga.Session) error {
	session.Usage()

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("infomgr").
		Description("CLI demonstrating built-in HelpCommand and VersionCommand").
		Version("2.4.1").
		Examples("infomgr status\ninfomgr help status\ninfomgr version").
		Footer("infomgr home: https://example.com/infomgr").
		UsageOnError(conga.UsageHint)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
