package main

import "github.com/conga-sh/conga"

func main() {
	app := conga.New[CLI]().
		EnableCompletion().
		Name("service-mgr").
		Description("Service management CLI demonstrating subcommands, aliases, and groups").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
