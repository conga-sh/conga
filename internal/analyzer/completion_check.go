package analyzer

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// optInState is the per-anchor verdict for an opt-in builder method
// (EnableCompletion, EnableFeatures). Unlike the app-wide appOptions facts, the
// enabled fact belongs to one conga.New anchor: the same command struct is
// valid under an app that calls the method and invalid under one that does not.
const (
	// optInUnknown leaves the fact undecided: multiple anchors, an escaping
	// app value, or a shape the analyzer does not attribute. The guard is
	// suppressed rather than guessed.
	optInUnknown optInState = iota
	// optInEnabled records a statically identified call to the builder method
	// on the anchored app value.
	optInEnabled
	// optInDisabled records an anchored app value that never calls the builder
	// method in its straight-line construction run.
	optInDisabled
)

type optInState int

// detectCompletionState attributes the completion-enabled fact to the single
// conga.New anchor of the package.
func detectCompletionState(pass *analysis.Pass, anchors []*anchor) optInState {
	return detectOptInState(pass, anchors, "EnableCompletion")
}

// detectFeatureState attributes the feature-enabled fact to the single
// conga.New anchor of the package.
func detectFeatureState(pass *analysis.Pass, anchors []*anchor) optInState {
	return detectOptInState(pass, anchors, "EnableFeatures")
}

// detectOptInState attributes an opt-in builder-method fact to the single
// conga.New anchor of the package. The construction chain is inspected first,
// then the straight-line statements on the app variable it binds, stopping at
// the first reference that is not a pure builder call on that variable.
// Multiple anchors or any escape of the app value leave the fact undecided, so
// the analyzer only ever misses a diagnostic, never invents one.
func detectOptInState(pass *analysis.Pass, anchors []*anchor, method string) optInState {
	if len(anchors) != 1 {
		return optInUnknown
	}

	found := anchors[0]
	if found.file == nil || found.call == nil {
		return optInUnknown
	}

	parents := nodeParents(found.file)
	chainEnd := chainEndNode(parents, found.call)

	if appChainCallsMethod(pass, chainEnd, method) {
		return optInEnabled
	}

	// A bare expression statement discards the app value, so nothing can
	// escape and the chain alone decides the fact.
	if _, ok := parents[chainEnd].(*ast.ExprStmt); ok {
		return optInDisabled
	}

	variable, block, index := boundAppVariable(pass, parents, chainEnd)
	if variable == nil {
		// The app value is returned, passed, stored, or otherwise escapes the
		// straight-line run, so its builder calls cannot be attributed.
		return optInUnknown
	}

	for _, statement := range block.List[index+1:] {
		if expression := statementExpression(statement); expression != nil {
			slots, root := fluentChainSegments(pass, expression, nil)
			if len(slots) > 0 && isAppVariableExpression(pass, root, variable) && variableReferenceCount(pass, statement, variable) == 1 {
				if slices.ContainsFunc(slots, func(slot builderOrderSlot) bool { return slot.name == method }) {
					return optInEnabled
				}

				continue
			}
		}

		if statementReferencesVariable(pass, statement, variable) {
			return optInUnknown
		}
	}

	return optInDisabled
}

// appChainCallsMethod reports whether a fluent construction chain rooted at a
// conga.New call invokes method on the app.
func appChainCallsMethod(pass *analysis.Pass, outermost ast.Expr, method string) bool {
	slots, _ := fluentChainSegments(pass, outermost, nil)

	return slices.ContainsFunc(slots, func(slot builderOrderSlot) bool { return slot.name == method })
}

// variableReferenceCount counts the references to the app variable inside one
// statement, so a builder call whose arguments also mention the app (for
// example app.Name(label(app))) is classified as an escape rather than a pure
// builder run.
func variableReferenceCount(pass *analysis.Pass, statement ast.Stmt, variable *types.Var) int {
	count := 0

	ast.Inspect(statement, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && pass.TypesInfo.Uses[identifier] == variable {
			count++
		}

		return true
	})

	return count
}
