// Package analyzer implements conga-lint, a static analysis pass that
// enforces CONGA CLI struct contracts at lint time. It mirrors the runtime
// schema validations performed by internal/spec (leaf command Runner
// enforcement, struct tag linting, and ancestor flag shadowing) and flags
// throwaway inferred conga.New(root) instances directly on type information,
// so contract violations surface before any binary runs.
package analyzer

import (
	"flag"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"

	"github.com/conga-sh/conga/internal/spec"
)

// Analyzer is the conga-lint analysis pass. Run it via
// `go tool conga-lint ./...` or `task lint:schema`.
//
//nolint:gochecknoglobals // the analysis framework requires a package-level Analyzer value
var Analyzer = &analysis.Analyzer{
	Name: "congalint",
	Doc:  "checks CONGA CLI struct definitions for contract compliance (leaf command Runner enforcement, struct tag validation, ancestor flag shadowing, throwaway root detection, completion command opt-in enforcement) plus opt-in fluent builder call order checking",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
	Flags: analyzerFlags,
}

const (
	// congaPackagePath is the import path of the CONGA runtime package.
	congaPackagePath = "github.com/conga-sh/conga"

	// configPackagePath is the import path of the CONGA runtime package,
	// which hosts the app.ConfigOptions option constructors. User-defined
	// constructors live in other packages and are never classified.
	configPackagePath = "github.com/conga-sh/conga"
)

// analyzerFlags registers the conga-lint command-line flags.
//
//nolint:gochecknoglobals // the analysis framework requires flag registration on a package-level FlagSet
var analyzerFlags = newAnalyzerFlags()

// anchor describes a conga.New instantiation discovered in the analyzed
// package, covering both the explicit conga.New[T] and the inferred
// conga.New(root) call forms. The root command struct type is T; the call
// position anchors diagnostics for root structs declared outside the package.
type anchor struct {
	packageObject *types.Package
	rootType      *types.Named
	position      token.Pos
	// file and call locate the construction site, so per-anchor builder facts
	// can be attributed by tracking the app value through the surrounding
	// statements.
	file *ast.File
	call *ast.CallExpr
}

// appOptions holds the statically detected builder options that affect tag
// validation: app.AllowTags and app.Predictor registrations. Non-literal
// arguments leave the evidence ambiguous; every consumer then falls back to
// the conservative default (checks suppressed), so the analyzer can only ever
// miss diagnostics, never invent them.
type appOptions struct {
	allowedTags              []string
	allowedTagsAmbiguous     bool
	registeredNames          []string
	registeredNamesAmbiguous bool
	// envPrefix records a statically identified app.EnvPrefix("...") call;
	// envPrefixAmbiguous marks non-literal or conflicting arguments that leave
	// the app-level prefix undecided. Ambiguous evidence leaves {{env}}
	// available in help tags, so the analyzer skips that check rather than
	// inventing a diagnostic, and suppresses derived-name reasoning in the
	// feature env collision check.
	envPrefix          string
	envPrefixAmbiguous bool
	// configCascading records a statically identified app.ConfigOptions(conga.ConfigCascading())
	// call; configCascadingAmbiguous marks unclassifiable ConfigOptions arguments
	// that could hide one. Ambiguous evidence skips the cascading guard.
	configCascading          bool
	configCascadingAmbiguous bool
}

func (o *appOptions) absorbCall(pass *analysis.Pass, node ast.Node) {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	method, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok || !isCongaAppMethod(method) {
		return
	}

	switch method.Name() {
	case "AllowTags":
		o.absorbAllowTagsCall(pass, call)
	case "Predictor":
		if len(call.Args) == 0 {
			return
		}

		if value, known := stringLiteralValue(pass, call.Args[0]); known {
			o.registeredNames = append(o.registeredNames, value)
		} else {
			o.registeredNamesAmbiguous = true
		}
	case "ConfigOptions":
		o.absorbConfigCall(pass, call)
	case "EnvPrefix":
		o.absorbEnvPrefixCall(pass, call)
	}
}

// absorbAllowTagsCall records the app.AllowTags literals; a non-literal
// argument marks the allow-list ambiguous.
func (o *appOptions) absorbAllowTagsCall(pass *analysis.Pass, call *ast.CallExpr) {
	for _, argument := range call.Args {
		if value, known := stringLiteralValue(pass, argument); known {
			o.allowedTags = append(o.allowedTags, value)
		} else {
			o.allowedTagsAmbiguous = true
		}
	}
}

// absorbEnvPrefixCall records the app-level environment prefix; a non-literal
// or conflicting argument leaves it ambiguous.
func (o *appOptions) absorbEnvPrefixCall(pass *analysis.Pass, call *ast.CallExpr) {
	if len(call.Args) == 0 {
		return
	}

	value, known := stringLiteralValue(pass, call.Args[0])
	if !known {
		o.envPrefixAmbiguous = true

		return
	}

	if o.envPrefix != "" && o.envPrefix != value {
		o.envPrefixAmbiguous = true
	}

	o.envPrefix = value
}

// absorbConfigCall classifies the option-constructor arguments of an
// app.ConfigOptions(...) call: a statically identified conga.ConfigCascading()
// enables the cascading guard, and anything else is ambiguous evidence that
// could hide a Cascading option. Conflicts between ConfigExplicit and
// ConfigPaths/ConfigCascading are detected per call and reported at the
// argument that completes the conflict: evidence never leaks across distinct
// apps in one package. Conflicts spanning multiple ConfigOptions calls on a
// single app are intentionally missed here — the analyzer may only miss
// diagnostics, never invent them, and the runtime foldConfigOptions guard
// remains the source of truth for that case.
func (o *appOptions) absorbConfigCall(pass *analysis.Pass, call *ast.CallExpr) {
	var hasExplicit, hasPaths, hasCascading bool

	reported := false

	for _, argument := range call.Args {
		switch configOptionName(pass, argument) {
		case "ConfigCascading":
			hasCascading = true
			o.configCascading = true
		case "ConfigPaths":
			hasPaths = true
		case "ConfigExplicit":
			hasExplicit = true
		default:
			o.configCascadingAmbiguous = true
		}

		if !reported && hasExplicit && (hasPaths || hasCascading) {
			pass.Reportf(argument.Pos(), "%s", spec.ConfigExplicitConflictError().Error())

			reported = true
		}
	}
}

// validationOptionsFor renders the detected options for a command node,
// including the environment prefix threaded through the hierarchy.
func (o *appOptions) validationOptionsFor(node *commandNode) spec.ValidationOptions {
	return spec.ValidationOptions{
		AllowedTags:            o.allowedTags,
		SkipUnknownTagKeyCheck: o.allowedTagsAmbiguous,
		EnvPrefix:              node.envPrefix,
		EnvPrefixKnown:         !o.envPrefixAmbiguous,
	}
}

// configOptionName reports the name of the conga option constructor an
// app.ConfigOptions argument calls (e.g. "ConfigCascading", "ConfigPaths",
// "ConfigExplicit"), or "" when the argument is not a statically identifiable
// config option call.
func configOptionName(pass *analysis.Pass, argument ast.Expr) string {
	call, ok := argument.(*ast.CallExpr)
	if !ok {
		return ""
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	optionFunc, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok || optionFunc.Pkg() == nil {
		return ""
	}

	if optionFunc.Pkg().Path() != configPackagePath {
		return ""
	}

	return optionFunc.Name()
}

// newAnalyzerFlags builds the flag set the conga-lint binary accepts.
func newAnalyzerFlags() flag.FlagSet {
	flags := flag.FlagSet{}
	flags.Bool("builder-order", false, "check that builder calls at conga.New construction sites follow the canonical method order")
	flags.Var(&builderOrderSpec{}, "builder-order-spec", "canonical builder call order: comma- or newline-separated App method names, or @path to read one per line (defaults to the built-in order)")

	return flags
}

func run(pass *analysis.Pass) (any, error) {
	checkThrowawayRoots(pass)

	if err := checkBuilderCallOrder(pass); err != nil {
		return nil, err
	}

	anchors := discoverAnchors(pass)
	if len(anchors) == 0 {
		return nil, nil //nolint:nilnil // the analysis driver contract expects a nil result on success
	}

	if len(anchors) == 1 {
		checkSessionRootCalls(pass, anchors[0])
	}

	loaderState := detectLoaderState(pass, anchors)
	checkLoaderConflicts(pass, anchors, loaderState)

	congaTypes, err := resolveCongaTypes(anchors[0].packageObject)
	if err != nil {
		return nil, err
	}

	options := collectAppOptions(pass)

	walker := newWalker(pass, congaTypes, options, detectCompletionState(pass, anchors), detectFeatureState(pass, anchors), loaderState, len(anchors) == 1)
	for _, anchor := range anchors {
		walker.walkRoot(anchor)
	}

	return nil, nil //nolint:nilnil // the analysis driver contract expects a nil result on success
}

func isFlagEnabled(pass *analysis.Pass, name string) bool {
	registered := pass.Analyzer.Flags.Lookup(name)

	return registered != nil && registered.Value.String() == "true"
}

func isTestFile(fileSet *token.FileSet, file *ast.File) bool {
	return strings.HasSuffix(fileSet.Position(file.Pos()).Filename, "_test.go")
}

// discoverAnchors finds every conga.New call in the analyzed package's
// non-test files — both the explicit conga.New[T] and the inferred
// conga.New(root) forms — deduplicating repeated instantiations of the same
// root type.
func discoverAnchors(pass *analysis.Pass) []*anchor {
	var anchors []*anchor

	seenRootTypes := make(map[types.Object]bool)

	for _, file := range pass.Files {
		if isTestFile(pass.Fset, file) {
			continue
		}

		for _, node := range anchorCallNodes(file) {
			found := anchorFromCall(pass, node)
			if found == nil {
				continue
			}

			if seenRootTypes[found.rootType.Obj()] {
				continue
			}

			seenRootTypes[found.rootType.Obj()] = true

			found.file = file
			found.call = node

			anchors = append(anchors, found)
		}
	}

	return anchors
}

func anchorCallNodes(file *ast.File) []*ast.CallExpr {
	var calls []*ast.CallExpr

	ast.Inspect(file, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}

		return true
	})

	return calls
}

func anchorFromCall(pass *analysis.Pass, call *ast.CallExpr) *anchor {
	selector, ok := newCallSelector(call)
	if !ok {
		return nil
	}

	newFunc, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok || newFunc.Name() != "New" || newFunc.Pkg() == nil || newFunc.Pkg().Path() != congaPackagePath {
		return nil
	}

	signature, signatureOk := newFunc.Type().(*types.Signature)
	if !signatureOk || signature.Recv() != nil {
		return nil
	}

	rootType, ok := newCallRootType(pass, call)
	if !ok {
		return nil
	}

	return &anchor{packageObject: newFunc.Pkg(), rootType: rootType, position: call.Pos()}
}

// newCallSelector unwraps the callee selector of both supported conga.New call
// forms: the explicit instantiation conga.New[T](...) and the inferred
// conga.New(root) that derives T from the root instance's pointer type.
func newCallSelector(call *ast.CallExpr) (*ast.SelectorExpr, bool) {
	switch function := call.Fun.(type) {
	case *ast.IndexExpr:
		selector, ok := function.X.(*ast.SelectorExpr)

		return selector, ok
	case *ast.SelectorExpr:
		return function, true
	default:
		return nil, false
	}
}

// newCallRootType resolves the root command struct type for a conga.New call.
// The explicit form carries T as the type argument (conga.New[T]); the
// inferred form derives T from the pointer type of the supplied root instance
// (conga.New(root)). Explicit instantiations keep their strict shape checks;
// inferred calls unwrap exactly one pointer level to reach the struct.
func newCallRootType(pass *analysis.Pass, call *ast.CallExpr) (*types.Named, bool) {
	if indexExpression, ok := call.Fun.(*ast.IndexExpr); ok {
		rootType, ok := pass.TypesInfo.TypeOf(indexExpression.Index).(*types.Named)
		if !ok || !isRootStructType(rootType) {
			return nil, false
		}

		return rootType, true
	}

	if len(call.Args) == 0 {
		return nil, false
	}

	pointer, ok := pass.TypesInfo.TypeOf(call.Args[0]).(*types.Pointer)
	if !ok {
		return nil, false
	}

	rootType, ok := pointer.Elem().(*types.Named)
	if !ok || !isRootStructType(rootType) {
		return nil, false
	}

	return rootType, true
}

// isRootStructType reports whether the named type is a non-generic struct
// eligible to act as a conga.New root command.
func isRootStructType(rootType *types.Named) bool {
	if rootType.TypeArgs().Len() > 0 {
		return false
	}

	_, ok := rootType.Underlying().(*types.Struct)

	return ok
}

func collectAppOptions(pass *analysis.Pass) *appOptions {
	options := &appOptions{}

	for _, file := range pass.Files {
		if isTestFile(pass.Fset, file) {
			continue
		}

		ast.Inspect(file, func(node ast.Node) bool {
			options.absorbCall(pass, node)

			return true
		})
	}

	return options
}

func isCongaAppMethod(method *types.Func) bool {
	signature, ok := method.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return false
	}

	receiver := signature.Recv().Type()
	if pointer, isPointer := receiver.(*types.Pointer); isPointer {
		receiver = pointer.Elem()
	}

	named, ok := receiver.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}

	return named.Obj().Pkg().Path() == congaPackagePath && named.Obj().Name() == "App"
}

func stringLiteralValue(pass *analysis.Pass, expression ast.Expr) (string, bool) {
	switch literal := expression.(type) {
	case *ast.BasicLit:
		if literal.Kind != token.STRING {
			return "", false
		}

		unquoted, err := strconv.Unquote(literal.Value)
		if err != nil {
			return "", false
		}

		return unquoted, true
	case *ast.Ident:
		constantObject, ok := pass.TypesInfo.Uses[literal].(*types.Const)
		if !ok || constantObject.Val().Kind() != constant.String {
			return "", false
		}

		return constant.StringVal(constantObject.Val()), true
	default:
		return "", false
	}
}

// checkSessionRootCalls inspects calls to (*conga.Session).Root[T]() in packages
// with a single conga.New anchor, reporting a diagnostic if the type argument T
// does not match the anchored application root command struct.
func checkSessionRootCalls(pass *analysis.Pass, singleAnchor *anchor) {
	for _, file := range pass.Files {
		if isTestFile(pass.Fset, file) {
			continue
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			typeArg, indexPos, ok := sessionRootCallTypeArg(pass, call)
			if !ok {
				return true
			}

			if !types.Identical(typeArg, singleAnchor.rootType) {
				qualifier := func(pkg *types.Package) string {
					if pkg == pass.Pkg || pkg == nil {
						return ""
					}

					return pkg.Name()
				}

				pass.Reportf(indexPos, "Session.Root[T] type %s does not match application root command %s",
					types.TypeString(typeArg, qualifier),
					types.TypeString(singleAnchor.rootType, qualifier),
				)
			}

			return true
		})
	}
}

func sessionRootCallTypeArg(pass *analysis.Pass, call *ast.CallExpr) (types.Type, token.Pos, bool) {
	var (
		selector  *ast.SelectorExpr
		indexNode ast.Expr
	)

	switch function := call.Fun.(type) {
	case *ast.IndexExpr:
		sel, ok := function.X.(*ast.SelectorExpr)
		if !ok {
			return nil, token.NoPos, false
		}

		selector = sel
		indexNode = function.Index
	case *ast.IndexListExpr:
		sel, ok := function.X.(*ast.SelectorExpr)
		if !ok || len(function.Indices) != 1 {
			return nil, token.NoPos, false
		}

		selector = sel
		indexNode = function.Indices[0]
	default:
		return nil, token.NoPos, false
	}

	if !isSessionRootMethod(pass, selector) {
		return nil, token.NoPos, false
	}

	typeArg := pass.TypesInfo.TypeOf(indexNode)
	if typeArg == nil {
		return nil, token.NoPos, false
	}

	return typeArg, indexNode.Pos(), true
}

func isSessionRootMethod(pass *analysis.Pass, selector *ast.SelectorExpr) bool {
	var method *types.Func

	if selection := pass.TypesInfo.Selections[selector]; selection != nil {
		if fn, ok := selection.Obj().(*types.Func); ok {
			method = fn
		}
	}

	if method == nil {
		if fn, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func); ok {
			method = fn
		}
	}

	if method == nil || method.Name() != "Root" {
		return false
	}

	signature, ok := method.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return false
	}

	receiver := signature.Recv().Type()
	if pointer, isPointer := receiver.(*types.Pointer); isPointer {
		receiver = pointer.Elem()
	}

	named, ok := receiver.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}

	return named.Obj().Pkg().Path() == congaPackagePath && named.Obj().Name() == "Session"
}
