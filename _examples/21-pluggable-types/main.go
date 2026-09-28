package main

import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/plugin/system"
	"github.com/conga-sh/conga/plugin/timezone"
	"github.com/conga-sh/conga/predict"
)

func main() {
	app := conga.New[CLI]().
		EnableCompletion().
		Name("pluggable-demo").
		Description("Pluggable type parsers, standard library types, and modular plugins").
		Version("1.0.0").
		Use(
			timezone.Location,
			system.Signal,
		)

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

	app.FatalIfError(app.Run())
}
