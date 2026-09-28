package main

import (
	"github.com/conga-sh/conga"
)

func main() {
	app := conga.New[CLI]().
		EnableCompletion().
		Name("nodectl").
		Description("Node administration CLI with modular system and network completion").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
