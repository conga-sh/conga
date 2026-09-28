package main

import "github.com/conga-sh/conga"

func main() {
	app := conga.New[CLI]().
		Name("cloudctl").
		Description("Multi-tier nested command tree demonstrating context hierarchy navigation").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
