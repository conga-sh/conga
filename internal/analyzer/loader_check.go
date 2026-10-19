package analyzer

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/analysis"

	"github.com/conga-sh/conga/internal/spec"
)

// loaderState records the statically identified configuration loader
// declaration of one conga.New anchor and the file-based mechanisms declared
// on the same app chain. Ambiguous evidence — multiple anchors, an escaping
// app value, or a non-classifiable ConfigOptions argument — leaves the state
// unknown so the analyzer only ever misses a diagnostic, never invents one.
type loaderState struct {
	// declared records a statically identified conga.ConfigLoader(...)
	// argument on the anchored app.
	declared bool
	// unknown suppresses every loader diagnostic for the anchor.
	unknown bool
	// fileOptionPos is the ConfigPaths, ConfigExplicit, or ConfigCascading
	// argument declared alongside a loader.
	fileOptionPos token.Pos
	// registerDecoderPos is the app.RegisterDecoder call declared alongside a
	// loader.
	registerDecoderPos token.Pos
}

// absorbExpression records the loader facts of every conga app builder call
// inside one fluent expression and reports whether any evidence was ambiguous.
func (s *loaderState) absorbExpression(pass *analysis.Pass, expression ast.Expr) bool {
	ambiguous := false

	ast.Inspect(expression, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		method, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
		if !ok || !isCongaAppMethod(method) {
			return true
		}

		switch method.Name() {
		case "ConfigOptions":
			for _, argument := range call.Args {
				switch configOptionName(pass, argument) {
				case "ConfigLoader":
					s.declared = true
				case "ConfigPaths", "ConfigExplicit", "ConfigCascading":
					if s.fileOptionPos == token.NoPos {
						s.fileOptionPos = argument.Pos()
					}
				default:
					ambiguous = true
				}
			}
		case "RegisterDecoder":
			if s.registerDecoderPos == token.NoPos {
				s.registerDecoderPos = selector.Sel.Pos()
			}
		}

		return true
	})

	return ambiguous
}

// checkLoaderConflicts reports combining conga.ConfigLoader with a file-based
// configuration mechanism on the same app: file resolution options, decoder
// registrations, or a root ConfigFile field. Runtime schema build
// (spec.validateLoaderConfigSettings) remains the source of truth; the
// analyzer mirrors the conflict messages for statically visible cases and
// stays silent when the evidence is ambiguous.
func checkLoaderConflicts(pass *analysis.Pass, anchors []*anchor, state loaderState) {
	if state.unknown || !state.declared {
		return
	}

	if state.fileOptionPos != token.NoPos {
		pass.Reportf(state.fileOptionPos, "%s", spec.ConfigLoaderOptionsConflictError().Error())

		return
	}

	if state.registerDecoderPos != token.NoPos {
		pass.Reportf(state.registerDecoderPos, "%s", spec.ConfigLoaderDecoderConflictError().Error())

		return
	}

	fieldPos, flagName := rootConfigFileField(pass, anchors[0])
	if fieldPos == token.NoPos {
		return
	}

	pass.Reportf(fieldPos, "%s", spec.ConfigLoaderConfigFileConflictError(flagName).Error())
}

// detectLoaderState attributes the configuration loader facts to the single
// conga.New anchor of the package. The construction chain is inspected first,
// then the straight-line statements on the app variable it binds, stopping at
// the first reference that is not a pure builder call on that variable.
func detectLoaderState(pass *analysis.Pass, anchors []*anchor) loaderState {
	if len(anchors) != 1 {
		return loaderState{unknown: true}
	}

	found := anchors[0]
	if found.file == nil || found.call == nil {
		return loaderState{unknown: true}
	}

	var state loaderState

	parents := nodeParents(found.file)
	chainEnd := chainEndNode(parents, found.call)

	if state.absorbExpression(pass, chainEnd) {
		return loaderState{unknown: true}
	}

	// A discarded app value (a bare expression statement or a blank
	// assignment) cannot be referenced elsewhere, so the chain alone decides
	// the facts.
	if isDiscardedExpression(parents, chainEnd) {
		return state
	}

	variable, block, index := boundAppVariable(pass, parents, chainEnd)
	if variable == nil {
		// The app value escapes the straight-line run, so its builder calls
		// cannot be attributed.
		return loaderState{unknown: true}
	}

	for _, statement := range block.List[index+1:] {
		if expression := statementExpression(statement); expression != nil {
			slots, root := fluentChainSegments(pass, expression, nil)
			if len(slots) > 0 && isAppVariableExpression(pass, root, variable) && variableReferenceCount(pass, statement, variable) == 1 {
				if state.absorbExpression(pass, expression) {
					return loaderState{unknown: true}
				}

				continue
			}
		}

		if statementReferencesVariable(pass, statement, variable) {
			return loaderState{unknown: true}
		}
	}

	return state
}

// isDiscardedExpression reports whether the expression's value is dropped in
// place: a bare expression statement, a blank assignment, or a package-level
// declaration binding only the blank identifier. The app value cannot be
// referenced elsewhere in these shapes, so its builder chain is the complete
// evidence.
func isDiscardedExpression(parents map[ast.Node]ast.Node, expression ast.Expr) bool {
	switch parent := parents[expression].(type) {
	case *ast.ExprStmt:
		return true
	case *ast.AssignStmt:
		if len(parent.Lhs) == 0 {
			return false
		}

		for _, lhs := range parent.Lhs {
			identifier, ok := lhs.(*ast.Ident)
			if !ok || identifier.Name != "_" {
				return false
			}
		}

		return true
	case *ast.ValueSpec:
		if len(parent.Names) == 0 {
			return false
		}

		for _, name := range parent.Names {
			if name.Name != "_" {
				return false
			}
		}

		return true
	default:
		return false
	}
}

// rootConfigFileField returns the diagnostic position and resolved flag name
// of the root command's ConfigFile field, or token.NoPos when the root
// declares none. The resolved name mirrors the runtime schema build: the
// field's name tag literal, defaulting to "config-file".
func rootConfigFileField(pass *analysis.Pass, found *anchor) (token.Pos, string) {
	structType, ok := found.rootType.Underlying().(*types.Struct)
	if !ok {
		return token.NoPos, ""
	}

	for index := range structType.NumFields() {
		field := structType.Field(index)
		if !isConfigFileType(field.Type()) {
			continue
		}

		position := found.position
		if field.Pkg() == pass.Pkg {
			position = field.Pos()
		}

		return position, cmp.Or(reflect.StructTag(structType.Tag(index)).Get("name"), "config-file")
	}

	return token.NoPos, ""
}

// isConfigFileType reports whether a field type is the conga.ConfigFile
// container, by value or by pointer.
func isConfigFileType(fieldType types.Type) bool {
	if pointer, ok := fieldType.(*types.Pointer); ok {
		fieldType = pointer.Elem()
	}

	named, ok := fieldType.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}

	return named.Obj().Pkg().Path() == congaPackagePath && named.Obj().Name() == "ConfigFile"
}
