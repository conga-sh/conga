package builderorderspec

import "github.com/conga-sh/conga"

// want +4 "builder call order violation: Version .rank 1. called after Name .rank 2.; use -fix to reorder"
func specOverrideViolation() {
	app := conga.New[CLI]().
		Name("spec").
		Version("1.0.0")

	_ = app
}
