package builderorderfix

import "github.com/conga-sh/conga"

// want +4 "builder call order violation: Name .rank 1. called after Version .rank 3.; use -fix to reorder"
func commentInsideArgumentsFix() {
	app := conga.New[CLI]().
		Version("1.0.0").
		Name("fix-comment" /* travels with this call */)

	_ = app
}
