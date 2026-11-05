package main

import "github.com/conga-sh/conga"

func main() {
	app := conga.New[CLI]().
		Name("deep-hierarchy-context").
		Description("Multi-tier nested command tree demonstrating context hierarchy navigation").
		Version("1.0.0").
		EnvPrefix("FLEET")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
