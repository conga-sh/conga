package builderorderinvalid

import "github.com/conga-sh/conga"

// want +5 "builder call order violation: Name .rank 1. called after EnableCompletion .rank 9.; reorder manually .not auto-fixable."
func commentPinned() {
	app := conga.New[CLI]()
	app.EnableCompletion()
	// the gap comment pins the statement group in place
	app.Name("bad-comment")

	_ = app
}
