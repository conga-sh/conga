package main

import "github.com/conga-sh/conga"

func main() {
	app := conga.New[CLI]().
		EnableCompletion().
		Name("runner").
		Description("Argument forwarding workflows (DashArgs vs RawArgs)").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
