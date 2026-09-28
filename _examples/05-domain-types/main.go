package main

import (
	"github.com/conga-sh/conga"
)

func main() {
	app := conga.New[CLI]().
		EnableCompletion().
		Name("domain-demo").
		Description("Strongly-typed domain types (File with tags, Dir, ByteSize, Custom)").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
