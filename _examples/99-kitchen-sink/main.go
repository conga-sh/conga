package main

import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/hcl"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
)

func main() {
	app := conga.New[KitchenSinkCLI]().
		Name("kitchen-sink").
		Description("Exhaustive type, configuration-mapping, and value-provenance reference").
		Version("1.0.0").
		EnableCompletion().
		EnableFeatures().
		EnableWhy(conga.WhyOptionJSON()).
		EnvPrefix("KITCHEN_SINK").
		Dotenv().
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".json", json.Decode).
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		RegisterDecoder(".hcl", hcl.Decode).
		AllowTags("kong")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
