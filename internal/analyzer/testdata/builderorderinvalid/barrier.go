package builderorderinvalid

import "github.com/conga-sh/conga"

// want +4 "builder call order violation: Name .rank 1. called after Use .rank 21.; reorder manually .not auto-fixable."
func barrierInversion() {
	app := conga.New[CLI]().
		Use().
		Name("bad-barrier")

	_ = app
}
