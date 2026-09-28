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
		Description("Exhaustive type, configuration-mapping, and schema-directive reference").
		Version("1.0.0").
		EnableCompletion().
		RegisterDecoder(".toml", toml.RawDecoder).
		RegisterDecoder(".json", json.RawDecoder).
		RegisterDecoder(".yaml", yaml.RawDecoder).
		RegisterDecoder(".yml", yaml.RawDecoder).
		RegisterDecoder(".hcl", hcl.RawDecoder).
		AllowTags("kong")

	app.FatalIfError(app.Run())
}
