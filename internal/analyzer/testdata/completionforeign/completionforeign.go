// Package completionforeign anchors a CLI declared in another package: the
// completion guard attributes the fact to this anchor and reports at the
// conga.New call because the offending field is foreign.
package completionforeign

import (
	"github.com/conga-sh/conga"

	"congalint.test/fixtures/completionlib"
)

func run() {
	app := conga.New[completionlib.Root]() // want `completion command declared without completion enabled; call app\.EnableCompletion\(\) when constructing the app, or remove the field`
	app.Run()
}
