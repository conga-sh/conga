package builderorderinvalid

import "github.com/conga-sh/conga"

// want +4 "builder call order violation: Name .rank 1. called after Version .rank 3.; use -fix to reorder"
func statementInversion() {
	app := conga.New[CLI]()
	app.Version("1.0.0")
	app.Name("bad-statements")

	_ = app
}
