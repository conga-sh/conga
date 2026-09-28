package builderorderinvalid

import "github.com/conga-sh/conga"

// want +4 "builder call order violation: Name .rank 1. called after EnableCompletion .rank 9.; use -fix to reorder"
func chainInversion() {
	app := conga.New[CLI]().
		EnableCompletion().
		Name("bad-chain")

	_ = app
}
