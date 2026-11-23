package analyzer

import (
	"go/types"

	"github.com/conga-sh/conga/internal/spec"
)

// checkRunner enforces the leaf command contract: a command struct without
// user subcommand fields must implement conga.Runner, mirroring the runtime
// bind-time check (noRunnerError) that otherwise only surfaces on first
// execution.
func (w *walker) checkRunner(node *commandNode, subcommandCount int) {
	if subcommandCount > 0 || node.named == nil {
		return
	}

	if w.implements(types.NewPointer(node.named), w.conga.runner) {
		return
	}

	position := node.reportPos
	if node.inPackage {
		position = node.named.Obj().Pos()
	}

	w.reportError(position, spec.NoRunnerError(node.named.Obj().Name()))
}
