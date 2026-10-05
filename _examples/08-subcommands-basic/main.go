package main

import "github.com/conga-sh/conga"

func main() {
	app := conga.New[CLI]().
		Name("service-mgr").
		Description("Service management CLI demonstrating subcommands, aliases, and groups").
		Version("1.0.0").
		EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
