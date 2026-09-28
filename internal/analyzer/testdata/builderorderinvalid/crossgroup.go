package builderorderinvalid

import "github.com/conga-sh/conga"

// want +3 "builder call order violation: Name .rank 1. called after Version .rank 3.; reorder manually .not auto-fixable."
func crossGroupInversion() {
	app := conga.New[CLI]().Version("1.0.0")
	app.Name("bad-crossgroup")

	_ = app
}
