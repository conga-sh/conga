package main

import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
	"github.com/conga-sh/conga/system"
	"github.com/conga-sh/conga/timezone"
)

func main() {
	app := conga.New[CLI]().
		Name("pluggable-demo").
		Description("Pluggable type parsers, standard library types, and modular bindings").
		Version("1.0.0").
		EnableCompletion()

	// RegisterType wires a parser for a foreign type that CONGA cannot parse
	// natively and that does not implement TextUnmarshaler; WithPredictor
	// attaches shell completion to it.
	app.RegisterType(parsePercent, conga.WithPredictor(
		predict.Items(
			predict.NewItem("25%", "Low load"),
			predict.NewItem("50%", "Medium load"),
			predict.NewItem("75%", "High load"),
			predict.NewItem("90%", "Critical load"),
		),
	))

	app.Use(
		timezone.RegisterLocation,
		system.RegisterSignal,
	)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
