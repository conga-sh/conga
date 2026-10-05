package main

import (
	"github.com/conga-sh/conga"
)

func main() {
	app := conga.New[CLI]().
		Name("domain-demo").
		Description("Strongly-typed domain types (File with tags, Dir, ByteSize, Custom)").
		Version("1.0.0").
		EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
