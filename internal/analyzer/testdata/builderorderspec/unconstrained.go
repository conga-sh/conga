package builderorderspec

import "github.com/conga-sh/conga"

// unconstrainedMethods inverts methods the active spec leaves unranked; the
// check must stay silent because no constrained pair is out of order.
func unconstrainedMethods() {
	app := conga.New[CLI]().
		EnableCompletion().
		Name("spec-unconstrained")

	_ = app
}
