package builderorderinvalid

import "github.com/conga-sh/conga"

// want +4 "builder call order violation: Description .rank 2. called after EnableCompletion .rank 9.; 1 more violation; use -fix to reorder"
func multipleInversions() {
	app := conga.New[CLI]().
		EnableCompletion().
		Description("bad-multiple").
		Name("bad-multiple")

	_ = app
}
