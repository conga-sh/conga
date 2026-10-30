package analyzer

import (
	"cmp"
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"maps"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/conga-sh/conga/internal/casing"
)

// errUnresolvable marks analyzer setup failures: the compiled conga package
// no longer exposes the runtime objects the analyzer asserts against.
var errUnresolvable = errors.New("unresolvable conga runtime type")

// congaWrapperRoles maps the exported conga container wrapper type names to
// the field role the analyzer assigns them, mirroring the runtime's
// zero-reflection container classifier (internal/container) while
// depending only on the conga package itself. The conga package is always a
// direct import of an anchored package, so it stays fully resolvable under
// export-data dependency loading; transitive dependencies do not.
//
//nolint:gochecknoglobals // the classification contract is inherently a package-level table
var congaWrapperRoles = map[string]string{
	"Command":           "subcommand",
	"HelpCommand":       "help command",
	"VersionCommand":    "version command",
	"CompletionCommand": "completion command",
	"ConfigFile":        "config file flag",
	"Features":          "features",
	"DashArgs":          "dash args",
	"RawArgs":           "raw args",
	"OptionalFlag":      "flag",
	"RequiredFlag":      "flag",
	"Counter":           "flag",
	"OptionalArg":       "positional argument",
	"RequiredArg":       "positional argument",
}

// congaTypes holds the runtime type objects the analyzer asserts against: the
// conga.Runner interface and the conga package object resolving unexported
// wrapper internals, so classification never drifts from the engine's
// zero-reflection contract.
type congaTypes struct {
	packageObject *types.Package
	runner        *types.Interface
	bootstrapper  *types.Interface
}

// groupPair identifies a oneof/and group combination on a single command.
// Runtime group validation is command-scoped, so the analyzer tracks pairs per
// command rather than across the hierarchy.
type groupPair struct {
	oneOf string
	and   string
}

// commandNode is one command struct in the static hierarchy rooted at a
// conga.New anchor.
type commandNode struct {
	named      *types.Named
	structType *types.Struct
	name       string
	inPackage  bool
	reportPos  token.Pos
	// envPrefix is the effective environment variable prefix inherited by the
	// command (its own sentinel tag, else the declaring Command[T] field's
	// envprefix tag, else the parent's, else the app-level prefix), normalized
	// with the trailing underscore the runtime schema builder applies.
	envPrefix string
	// envPrefixKnown reports whether envPrefix was resolved from literal
	// evidence (a local tag or an unambiguous app-level prefix). An unknown
	// prefix suppresses derived-name reasoning, so the analyzer may only miss
	// diagnostics, never invent them.
	envPrefixKnown bool
	// isRoot marks the command struct anchored by conga.New. conga.Features is
	// legal only on the root command struct.
	isRoot bool
	// hasBootstrapper reports whether this command or an ancestor implements
	// conga.Bootstrapper; bootstrap-tagged fields require it.
	hasBootstrapper bool
	// featuresDeclared reports whether the root already declared a
	// conga.Features container; the runtime allows exactly one, and
	// featureFlagName carries the resolved synthesized flag name of that
	// container for the duplicate-container diagnostic.
	featuresDeclared bool
	featureFlagName  string
}

// walker drives the static hierarchy traversal and emits diagnostics.
type walker struct {
	pass         *analysis.Pass
	conga        *congaTypes
	options      *appOptions
	reported     map[string]bool
	visitPath    map[types.Object]bool
	singleAnchor bool
	// completionState is the per-anchor completion verdict: the completion
	// command guard reports only when the anchored app definitely never calls
	// EnableCompletion.
	completionState optInState
	// featureState is the per-anchor feature verdict: the Features and
	// feature-tag guards report only when the anchored app definitely never
	// calls EnableFeatures.
	featureState optInState
	// configFileMulti tracks the root command's config-file flag arity
	// (multi tag) for the cascading guard: nil when no field is declared.
	// The state is reset per walked root.
	configFileMulti       *bool
	configFileFlagName    string
	configFileFieldObject types.Object
	configFileNode        *commandNode
	// featureEnvBindings records the environment variables the root Features
	// container binds — the combined map variable and per-feature fields —
	// with owner descriptions, for the env collision check. The state is
	// reset per walked root.
	featureEnvBindings []envBinding
	// flagEnvBindings records every flag's environment variable bindings in
	// declaration order across the walked tree. Feature-vs-flag collisions
	// iterate it in flag order, mirroring the runtime walk.
	flagEnvBindings []envBinding
	// featureEnvNode and featureEnvFieldObject locate the Features container
	// field for collision diagnostics.
	featureEnvNode        *commandNode
	featureEnvFieldObject types.Object
}

func newWalker(pass *analysis.Pass, congaTypes *congaTypes, options *appOptions, completionState, featureState optInState, singleAnchor bool) *walker {
	return &walker{
		pass:            pass,
		conga:           congaTypes,
		options:         options,
		reported:        make(map[string]bool),
		visitPath:       make(map[types.Object]bool),
		singleAnchor:    singleAnchor,
		completionState: completionState,
		featureState:    featureState,
	}
}

func (w *walker) walkRoot(root *anchor) {
	structType, ok := root.rootType.Underlying().(*types.Struct)
	if !ok {
		return
	}

	w.configFileMulti = nil
	w.configFileFlagName = ""
	w.configFileFieldObject = nil
	w.configFileNode = nil
	w.featureEnvBindings = nil
	w.featureEnvNode = nil
	w.featureEnvFieldObject = nil
	w.flagEnvBindings = nil

	rootEnvPrefix := nodeEnvPrefix(structType)

	node := &commandNode{
		named:           root.rootType,
		structType:      structType,
		name:            casing.Kebab(root.rootType.Obj().Name()),
		inPackage:       w.isCurrentPackage(root.rootType.Obj()),
		reportPos:       root.position,
		envPrefix:       normalizedEnvPrefix(cmp.Or(rootEnvPrefix, w.options.envPrefix)),
		envPrefixKnown:  rootEnvPrefix != "" || !w.options.envPrefixAmbiguous,
		isRoot:          true,
		hasBootstrapper: w.implements(types.NewPointer(root.rootType), w.conga.bootstrapper),
	}

	w.walkCommand(node, map[string]string{}, map[rune]string{})
	w.checkCascadingArity()
	w.checkFeatureEnvCollisions()
}

// walkCommand mirrors the runtime schema walk: struct tag validation per
// field, ancestor flag shadowing against the accumulated maps, and the leaf
// command Runner contract.
func (w *walker) walkCommand(node *commandNode, ancestorNames map[string]string, ancestorShorts map[rune]string) {
	typeObject := node.named.Obj()
	if w.visitPath[typeObject] {
		return
	}

	w.visitPath[typeObject] = true
	defer delete(w.visitPath, typeObject)

	w.checkBuiltinShortPair(node)

	branchNames := maps.Clone(ancestorNames)
	branchShorts := maps.Clone(ancestorShorts)
	branchGroups := make(map[groupPair]string)

	subcommandCount := 0

	var childNodes []*commandNode

	for index := range node.structType.NumFields() {
		fieldObject := node.structType.Field(index)
		tag := reflect.StructTag(node.structType.Tag(index))

		childNode := w.checkField(node, fieldObject, tag, branchNames, branchShorts, branchGroups)
		if childNode != nil {
			subcommandCount++

			childNodes = append(childNodes, childNode)
		}
	}

	w.checkRunner(node, subcommandCount)

	for _, childNode := range childNodes {
		w.walkCommand(childNode, branchNames, branchShorts)
	}
}

// checkField validates the tags of a single field and returns the child
// command node for user subcommand fields. Flag fields are folded into the
// shadow maps.
func (w *walker) checkField(node *commandNode, fieldObject types.Object, tag reflect.StructTag, branchNames map[string]string, branchShorts map[rune]string, branchGroups map[groupPair]string) *commandNode {
	if !fieldObject.Exported() {
		if fieldObject.Name() == "_" {
			w.validateNodeFieldTags(node, fieldObject, tag, "sentinel field", nil)
		}

		return nil
	}

	role, known := congaWrapperRoles[w.congaWrapperName(fieldObject.Type())]
	if !known {
		return nil
	}

	switch role {
	case "subcommand":
		return w.checkSubcommandField(node, fieldObject, tag)
	case "help command":
		w.checkBuiltinCommandField(node, fieldObject, tag, role)

		return nil
	case "version command":
		w.checkBuiltinCommandField(node, fieldObject, tag, role)

		return nil
	case "completion command":
		w.checkCompletionCommandField(node, fieldObject, tag)

		return nil
	case "config file flag":
		w.checkConfigFileField(node, fieldObject, tag, branchNames, branchShorts)

		return nil
	case "features":
		w.checkFeaturesField(node, fieldObject, tag, branchNames)

		return nil
	case "dash args", "raw args":
		w.checkTrailingArgsField(node, fieldObject, tag, role)

		return nil
	case "flag":
		w.checkFlagField(node, fieldObject, tag, branchNames, branchShorts, branchGroups)

		return nil
	case "positional argument":
		w.checkArgField(node, fieldObject, tag)

		return nil
	default:
		return nil
	}
}

// checkTrailingArgsField validates a DashArgs/RawArgs field and mirrors the
// bootstrap-tag guards.
func (w *walker) checkTrailingArgsField(node *commandNode, fieldObject types.Object, tag reflect.StructTag, role string) {
	if w.validateNodeFieldTags(node, fieldObject, tag, role, nil) {
		w.checkBootstrapField(node, fieldObject, tag, nil)
	}
}

// congaWrapperName returns the name of the conga container wrapper type
// referenced by a field type, or "" when the field is not a conga container.
// Wrapper structs declare their marker methods on pointer receivers, so a
// value-typed field is classified after dereferencing to mirror the runtime's
// addressable interface assertions.
func (w *walker) congaWrapperName(fieldType types.Type) string {
	containerType := fieldType
	if pointer, ok := containerType.(*types.Pointer); ok {
		containerType = pointer.Elem()
	}

	named, ok := containerType.(*types.Named)
	if !ok {
		return ""
	}

	object := named.Obj()
	if object.Pkg() == nil || object.Pkg().Path() != congaPackagePath {
		return ""
	}

	return object.Name()
}

// buildChildNode resolves the command struct referenced by a Command[T]
// field into a child command node.
func (w *walker) buildChildNode(node *commandNode, fieldObject types.Object, tag reflect.StructTag) *commandNode {
	named, ok := w.containerInnerType(fieldObject.Type()).(*types.Named)
	if !ok || named.TypeArgs().Len() > 0 {
		return nil
	}

	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	localEnvPrefix := cmp.Or(nodeEnvPrefix(structType), tag.Get("envprefix"))

	childNode := &commandNode{
		named:           named,
		structType:      structType,
		name:            cmp.Or(tag.Get("name"), casing.Kebab(fieldObject.Name())),
		inPackage:       w.isCurrentPackage(named.Obj()),
		reportPos:       w.fieldReportPos(node, fieldObject),
		hasBootstrapper: node.hasBootstrapper || w.implements(types.NewPointer(named), w.conga.bootstrapper),
	}

	if localEnvPrefix != "" {
		childNode.envPrefix = normalizedEnvPrefix(localEnvPrefix)
		childNode.envPrefixKnown = true
	} else {
		childNode.envPrefix = node.envPrefix
		childNode.envPrefixKnown = node.envPrefixKnown
	}

	return childNode
}

func (w *walker) implements(target types.Type, interfaceType *types.Interface) bool {
	return interfaceType != nil && types.Implements(target, interfaceType)
}

func (w *walker) isCurrentPackage(object types.Object) bool {
	return object.Pkg() != nil && object.Pkg() == w.pass.Pkg
}

// report emits a diagnostic once per position and message.
func (w *walker) report(position token.Pos, message string) {
	key := fmt.Sprintf("%d|%s", position, message)
	if w.reported[key] {
		return
	}

	w.reported[key] = true

	w.pass.Report(analysis.Diagnostic{Pos: position, Message: message})
}

func (w *walker) reportFieldError(node *commandNode, fieldObject types.Object, err error) {
	if err == nil {
		return
	}

	w.report(w.fieldReportPos(node, fieldObject), fmt.Sprintf("struct %s field %s: %s", node.named.Obj().Name(), fieldObject.Name(), err.Error()))
}

// fieldReportPos resolves the diagnostic position for a field of the command
// struct: the field declaration when it lives in the analyzed package, the
// referencing subcommand field (or the conga.New anchor) otherwise.
func (w *walker) fieldReportPos(node *commandNode, fieldObject types.Object) token.Pos {
	if node.inPackage && w.isCurrentPackage(fieldObject) {
		return fieldObject.Pos()
	}

	return node.reportPos
}

// envBinding is one environment variable bound by a feature or a flag, with
// the owner description the collision diagnostic names.
type envBinding struct {
	envVar string
	owner  string
}

// normalizedEnvPrefix mirrors the runtime schema builder's trailing-underscore
// normalization of environment prefixes.
func normalizedEnvPrefix(prefix string) string {
	if prefix == "" || strings.HasSuffix(prefix, "_") {
		return prefix
	}

	return prefix + "_"
}

// nodeEnvPrefix mirrors the runtime scanSentinelEnvPrefix scan: the
// last sentinel field declaring envprefix wins.
func nodeEnvPrefix(structType *types.Struct) string {
	var envPrefix string

	for index := range structType.NumFields() {
		if structType.Field(index).Name() != "_" {
			continue
		}

		if prefix, ok := reflect.StructTag(structType.Tag(index)).Lookup("envprefix"); ok {
			envPrefix = prefix
		}
	}

	return envPrefix
}

// resolveCongaTypes resolves the runtime conga package objects the analyzer
// asserts against. The conga package is always a direct import of an anchored
// package, so its exported scope is complete under both source and export-data
// dependency loading; transitive dependencies cannot be relied on. Failure
// here signals a version skew between the analyzer and the compiled conga
// package and must surface as an analyzer error rather than a silent pass.
func resolveCongaTypes(congaPackage *types.Package) (*congaTypes, error) {
	runner := lookupInterface(congaPackage, "Runner")
	if runner == nil {
		return nil, fmt.Errorf("%w: conga.Runner missing from package %q", errUnresolvable, congaPackage.Path())
	}

	bootstrapper := lookupInterface(congaPackage, "Bootstrapper")
	if bootstrapper == nil {
		return nil, fmt.Errorf("%w: conga.Bootstrapper missing from package %q", errUnresolvable, congaPackage.Path())
	}

	for wrapperName := range congaWrapperRoles {
		if congaPackage.Scope().Lookup(wrapperName) == nil {
			return nil, fmt.Errorf("%w: conga.%s missing from package %q", errUnresolvable, wrapperName, congaPackage.Path())
		}
	}

	return &congaTypes{packageObject: congaPackage, runner: runner, bootstrapper: bootstrapper}, nil
}

func lookupInterface(packageObject *types.Package, name string) *types.Interface {
	object := packageObject.Scope().Lookup(name)
	if object == nil {
		return nil
	}

	interfaceType, ok := object.Type().Underlying().(*types.Interface)
	if !ok {
		return nil
	}

	return interfaceType
}
