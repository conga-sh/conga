package main

import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/yaml"
)

func main() {
	app := conga.New[CLI]().
		Name("watcher-app").
		Description("Daemon CLI demonstrating live configuration watching with path expansion").
		Version("1.0.0").
		RegisterDecoder(".yaml", yaml.RawDecoder).
		RegisterDecoder(".yml", yaml.RawDecoder).
		ConfigOptions(config.Paths(
			"/etc/watcher-app/config.yaml",
			"${XDG_CONFIG_HOME:-~/.config}/watcher-app/config.yaml",
			"./config.yaml",
		), config.Cascading())

	app.FatalIfError(app.Run())
}
