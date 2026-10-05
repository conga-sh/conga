package main

import (
	"errors"

	"github.com/conga-sh/conga"
)

var (
	errSimulatedFatal      = errors.New("simulated fatal transaction failure")
	errRegistryUnreachable = errors.New("unable to reach container registry")
)

func main() {
	app := conga.New[CLI]().
		Name("migrator").
		Description("CLI demonstrating full lifecycle hooks (PreRun -> Run -> PostRun -> Finally)").
		Version("1.0.0")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
