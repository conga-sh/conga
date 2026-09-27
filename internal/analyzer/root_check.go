package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// checkThrowawayRoots flags inferred conga.New(root) calls whose root instance
// is never referenced outside the app: the instance exists only to satisfy the
// variadic root parameter, so the engine-allocating conga.New[T]() form is the
// intended spelling. Fresh allocations (new(T), &T{}) and local variables
// whose sole use is the conga.New argument are reported; any read, seeding,
// closure capture, or comparison suppresses the diagnostic.
func checkThrowawayRoots(pass *analysis.Pass) {
	for _, file := range pass.Files {
		for _, call := range anchorCallNodes(file) {
			found := anchorFromCall(pass, call)
			if found == nil || !isThrowawayRootCall(pass, call) {
				continue
			}

			pass.Reportf(call.Args[0].Pos(), "conga.New root instance is not referenced outside the app (use conga.New[%s]() to let the engine allocate the root)", found.rootType.Obj().Name())
		}
	}
}

// isThrowawayRootCall reports whether an inferred conga.New(root) call supplies
// an unreachable root instance: the explicit conga.New[T](...) form carries no
// root argument, and variadic calls are multi-root panic guards, so both are
// left alone.
func isThrowawayRootCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	switch call.Fun.(type) {
	case *ast.IndexExpr, *ast.IndexListExpr:
		return false
	}

	if len(call.Args) != 1 {
		return false
	}

	return isThrowawayRootInstance(pass, call.Args[0])
}

// isThrowawayRootInstance classifies the root argument expression: fresh
// allocations construct an instance nothing else can reach, while any other
// expression shape is treated as caller-owned and never reported.
func isThrowawayRootInstance(pass *analysis.Pass, rootArgument ast.Expr) bool {
	switch argument := rootArgument.(type) {
	case *ast.CallExpr:
		return isBuiltinNewCall(pass, argument)
	case *ast.UnaryExpr:
		_, isComposite := argument.X.(*ast.CompositeLit)

		return argument.Op == token.AND && isComposite
	case *ast.Ident:
		variable, ok := pass.TypesInfo.Uses[argument].(*types.Var)
		if !ok {
			return false
		}

		return isSingleUseObject(pass, variable)
	default:
		return false
	}
}

// isBuiltinNewCall reports whether the expression calls the universe new
// builtin; a shadowing user-defined new is left alone.
func isBuiltinNewCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	builtin, ok := pass.TypesInfo.Uses[identifier].(*types.Builtin)

	return ok && builtin.Name() == "new"
}

// isSingleUseObject reports whether the pass observes exactly one reference to
// the object, which is the conga.New argument currently under inspection.
func isSingleUseObject(pass *analysis.Pass, object types.Object) bool {
	uses := 0

	for _, used := range pass.TypesInfo.Uses {
		if used == object {
			uses++
		}
	}

	return uses == 1
}
