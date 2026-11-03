package main

import "github.com/conga-sh/conga"

func main() {
	app := conga.New[CLI]().
		Name("bunlike").
		Description("Bun-style implicit file execution").
		Version("1.0.0")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
