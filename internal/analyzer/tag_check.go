package analyzer

import (
	"cmp"
	"fmt"
	"go/token"
	"go/types"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/spec"
)

// congaBuiltinTaggedTypes maps the conga package's built-in tagged domain
// types to their live TagSpecProvider implementations. Static analysis cannot
// evaluate the analyzed code's TagSpecs() methods, so the built-in vocabulary
// is consulted through the real conga types compiled into the analyzer,
// keeping the type methods the single source of truth.
//
//nolint:gochecknoglobals // static dispatch table from analyzed type names to the real conga built-in implementations
var congaBuiltinTaggedTypes = map[string]spec.TagSpecProvider{
	"File":     new(conga.File),
	"Dir":      new(conga.Dir),
	"ByteSize": new(conga.ByteSize),
}

// validateNodeFieldTags runs the shared tag-validation facade for a field of
// the command node and reports whether role-specific checks should proceed.
// tagSpecs are the field's domain-type tag registrations; their keys join the
// strict tag-key candidates.
func (w *walker) validateNodeFieldTags(node *commandNode, fieldObject types.Object, tag reflect.StructTag, role string, tagSpecs []spec.TagSpec) bool {
	candidates, roleKnown := spec.TagCandidatesForRole(role)
	if !roleKnown {
		return false
	}

	if len(tagSpecs) > 0 {
		candidates = slices.Concat(candidates, spec.FieldTagKeys(tagSpecs))
	}

	dirs, err := spec.ValidateFieldTags(tag, role, candidates, w.options.validationOptionsFor(node))
	w.reportFieldError(node, fieldObject, err)

	return err == nil && !dirs.Ignore
}

// congaFieldTagSpecs resolves the field-tag specs of a flag or positional
// argument element type. Only named types declared by the conga package
// resolve through the built-in dispatch: foreign types that happen to share a
// builtin name are not honored statically, mirroring the runtime's
// TagSpecProvider interface assertion and keeping the analyzer conservative
// (it may miss diagnostics, never invent them).
func (w *walker) congaFieldTagSpecs(targetType types.Type) []spec.TagSpec {
	if targetType == nil {
		return nil
	}

	if slice, ok := targetType.Underlying().(*types.Slice); ok {
		targetType = slice.Elem()
	}

	if mapping, ok := targetType.Underlying().(*types.Map); ok {
		targetType = mapping.Elem()
	}

	named, ok := targetType.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != congaPackagePath {
		return nil
	}

	provider, ok := congaBuiltinTaggedTypes[named.Obj().Name()]
	if !ok {
		return nil
	}

	return provider.TagSpecs()
}

// checkSubcommandField validates a user subcommand field and returns the
// child command node to recurse into.
func (w *walker) checkSubcommandField(node *commandNode, fieldObject types.Object, tag reflect.StructTag) *commandNode {
	if !w.validateNodeFieldTags(node, fieldObject, tag, "subcommand", nil) {
		return nil
	}

	w.reportFieldError(node, fieldObject, summaryTagError(tag))
	w.checkFeatureExpression(node, fieldObject, tag)

	childAllowUnknown, childAllowRecurse := threadDirectiveOptions(node, parseNodeDirectives(tag))

	return w.buildChildNode(node, fieldObject, tag, childAllowUnknown, childAllowRecurse)
}

// checkFeatureExpression mirrors the runtime feature-tag syntax check: a
// declared feature expression must parse. The runtime schema builder validates
// subcommand, flag, and config-file flag feature tags through the same shared
// validator, so the analyzer reports the identical diagnostic.
func (w *walker) checkFeatureExpression(node *commandNode, fieldObject types.Object, tag reflect.StructTag) {
	featureExpr := strings.TrimSpace(tag.Get("feature"))
	if featureExpr == "" {
		return
	}

	w.reportFieldError(node, fieldObject, spec.ValidateFeatureExpression(featureExpr))
}

// checkBuiltinCommandField validates a built-in help/version/completion
// command field.
func (w *walker) checkBuiltinCommandField(node *commandNode, fieldObject types.Object, tag reflect.StructTag, role string) {
	if !w.validateNodeFieldTags(node, fieldObject, tag, role, nil) {
		return
	}

	w.reportFieldError(node, fieldObject, summaryTagError(tag))
}

// checkCompletionCommandField validates a built-in completion command field
// and mirrors the runtime opt-in guard: a declared CompletionCommand under an
// app that never calls EnableCompletion is rejected at schema build. Tag
// validation runs first (an ignored or invalid field short-circuits the guard,
// exactly like the runtime), and undecided per-anchor evidence leaves the
// diagnostic suppressed.
func (w *walker) checkCompletionCommandField(node *commandNode, fieldObject types.Object, tag reflect.StructTag) {
	if !w.validateNodeFieldTags(node, fieldObject, tag, "completion command", nil) {
		return
	}

	w.reportFieldError(node, fieldObject, summaryTagError(tag))

	if w.completionState == completionDisabled {
		w.reportFieldError(node, fieldObject, spec.CompletionRequiresEnableError())
	}
}

// checkConfigFileField validates a config file flag field and folds it into
// the ancestor shadow maps (runtime config file flags join cmd.Flags). The
// config-file tag vocabulary (multi) is engine-owned and shared through the
// spec facade, mirroring the runtime schema builder.
func (w *walker) checkConfigFileField(node *commandNode, fieldObject types.Object, tag reflect.StructTag, branchNames map[string]string, branchShorts map[rune]string) {
	tagSpecs := spec.ConfigFileFieldTagSpecs()

	if !w.validateNodeFieldTags(node, fieldObject, tag, "config file flag", tagSpecs) {
		return
	}

	w.reportFieldError(node, fieldObject, spec.ValidateConfigFileConfigTag(tag))

	w.checkFeatureExpression(node, fieldObject, tag)

	fieldTags, err := spec.ParseFieldTags(tag, tagSpecs)
	w.reportFieldError(node, fieldObject, err)

	flagName := cmp.Or(tag.Get("name"), "config-file")
	w.trackRootConfigFileArity(node, fieldObject, fieldTags, flagName)

	w.checkAndCollectFlag(node, fieldObject, tag, flagName, branchNames, branchShorts)
}

// checkFeaturesField validates a root feature-declaration field
// (conga.Features[T]): the container tag vocabulary and, for every exported
// inner field, the per-feature tag vocabulary. Feature-name reference validation
// is left to schema build, which has the fully reflected declaration set.
func (w *walker) checkFeaturesField(node *commandNode, fieldObject types.Object, tag reflect.StructTag) {
	if !node.isRoot {
		w.reportFieldError(node, fieldObject, spec.FeaturesMustBeOnRootError())

		return
	}

	if !w.validateNodeFieldTags(node, fieldObject, tag, "features", nil) {
		return
	}

	innerType := w.containerInnerType(fieldObject.Type())

	structType, ok := innerType.Underlying().(*types.Struct)
	if !ok {
		w.reportFieldError(node, fieldObject, spec.FeaturesMustBeStructParameterError())

		return
	}

	for index := range structType.NumFields() {
		innerField := structType.Field(index)
		if !innerField.Exported() {
			continue
		}

		if !w.validateNodeFieldTags(node, innerField, reflect.StructTag(structType.Tag(index)), "feature", nil) {
			continue
		}

		if !isFeatureScalarType(innerField.Type()) {
			w.reportFieldError(node, innerField, spec.FeatureFieldTypeError(innerField.Name(), innerField.Type().String()))
		}
	}
}

// trackRootConfigFileArity records the root command's config-file flag arity
// (multi tag) and resolved flag name for the cascading guard. Nested commands
// can never declare config-file flags, so the first record wins.
func (w *walker) trackRootConfigFileArity(node *commandNode, fieldObject types.Object, fieldTags spec.FieldTags, flagName string) {
	if w.configFileMulti != nil {
		return
	}

	isMulti := fieldTags.Bool("multi")
	w.configFileMulti = &isMulti
	w.configFileFlagName = flagName

	if !isMulti {
		w.configFileFieldObject = fieldObject
		w.configFileNode = node
	}
}

// checkCascadingArity mirrors the runtime cascading guard: when the app
// enables ConfigCascading() and the root struct declares a config-file flag without
// multi:"true", the flag cannot report every merged file. Ambiguous builder
// evidence skips the check entirely, and packages anchoring more than one
// conga.New root skip it as well: builder-option evidence is aggregated
// package-wide, so it cannot be attributed to a specific root without
// risking a diagnostic on an innocent sibling — the analyzer may only miss
// diagnostics, never invent them. The runtime schema guard stays per-root
// and remains the source of truth for multi-root packages.
func (w *walker) checkCascadingArity() {
	if !w.singleAnchor {
		return
	}

	if !w.options.configCascading || w.options.configCascadingAmbiguous {
		return
	}

	if w.configFileMulti == nil || *w.configFileMulti {
		return
	}

	w.reportFieldError(w.configFileNode, w.configFileFieldObject, spec.CascadingRequiresMultiError(w.configFileFlagName))
}

// checkFlagField validates a flag field: the shared facade, the
// flag-specific value tags, the domain-type field tags, and the ancestor
// shadow contract.
func (w *walker) checkFlagField(node *commandNode, fieldObject types.Object, tag reflect.StructTag, branchNames map[string]string, branchShorts map[rune]string) {
	targetType := w.flagTargetType(fieldObject)
	tagSpecs := w.congaFieldTagSpecs(targetType)

	if !w.validateNodeFieldTags(node, fieldObject, tag, "flag", tagSpecs) {
		return
	}

	if w.congaWrapperName(fieldObject.Type()) == "Counter" {
		w.reportFieldError(node, fieldObject, spec.ValidateCounterDefaultTag(tag))
	}

	if strings.TrimSpace(tag.Get("feature")) != "" && w.congaWrapperName(fieldObject.Type()) == "RequiredFlag" {
		w.reportFieldError(node, fieldObject, spec.FeatureOnRequiredFlagError())
	}

	w.checkFeatureExpression(node, fieldObject, tag)

	w.reportFieldError(node, fieldObject, spec.ValidatePredictorTag(tag, isBasicKind(targetType, types.Bool), w.predictorOptions(), w.options.registeredNames))
	w.checkValidationSpecs(node, fieldObject, tag, targetType, tagSpecs)

	if _, err := spec.ParseFieldTags(tag, tagSpecs); err != nil {
		w.reportFieldError(node, fieldObject, err)
	}

	w.reportFieldError(node, fieldObject, strictBoolTagError(tag))

	w.checkAndCollectFlag(node, fieldObject, tag, cmp.Or(tag.Get("name"), casing.Kebab(fieldObject.Name())), branchNames, branchShorts)
}

// checkArgField validates a positional argument field.
func (w *walker) checkArgField(node *commandNode, fieldObject types.Object, tag reflect.StructTag) {
	targetType := w.containerInnerType(fieldObject.Type())
	tagSpecs := w.congaFieldTagSpecs(targetType)

	if !w.validateNodeFieldTags(node, fieldObject, tag, "positional argument", tagSpecs) {
		return
	}

	w.reportFieldError(node, fieldObject, spec.ValidatePredictorTag(tag, false, w.predictorOptions(), w.options.registeredNames))
	w.checkValidationSpecs(node, fieldObject, tag, targetType, tagSpecs)

	if _, err := spec.ParseFieldTags(tag, tagSpecs); err != nil {
		w.reportFieldError(node, fieldObject, err)
	}
}

// checkValidationSpecs mirrors the runtime parseValidationSpecs: range bounds
// against the element type, and non-negative len/items counts. The field's
// domain-type tags are parsed and threaded into bound validation so tag-aware
// types (conga.ByteSize) reject bounds that contradict their tags.
func (w *walker) checkValidationSpecs(node *commandNode, fieldObject types.Object, tag reflect.StructTag, targetType types.Type, tagSpecs []spec.TagSpec) {
	if rangeTag := tag.Get("range"); rangeTag != "" {
		rangeSpec, err := spec.ParseRangeSpec(rangeTag)
		w.reportFieldError(node, fieldObject, err)

		if err == nil {
			// The caller reports ParseFieldTags errors separately; bound
			// validation only needs the parsed tags, so a tag error skips it
			// rather than duplicating the diagnostic.
			fieldTags, err := spec.ParseFieldTags(tag, tagSpecs)
			if err == nil {
				w.reportFieldError(node, fieldObject, w.validateRangeBounds(rangeSpec, targetType, fieldTags))
			}
		}
	}

	for _, countTag := range []string{"len", "items"} {
		if countValue := tag.Get(countTag); countValue != "" {
			countSpec, err := spec.ParseRangeSpec(countValue)
			w.reportFieldError(node, fieldObject, err)

			if err == nil {
				w.reportFieldError(node, fieldObject, spec.ValidateCountRangeSpec(countSpec, countTag))
			}
		}
	}
}

// validateRangeBounds is the static port of the runtime validateRangeSpec:
// numeric element kinds get their bounds parsed, the duration type gets
// duration bounds, conga.ByteSize is parsed with the runtime grammar compiled
// into the analyzer, and other text-grammar types with an ordered projection
// are accepted without bound evaluation because their grammar is not statically
// executable. Element types without any range strategy are rejected, mirroring
// schema build.
func (w *walker) validateRangeBounds(rangeSpec *spec.Range, targetType types.Type, fieldTags spec.FieldTags) error {
	if rangeSpec == nil || targetType == nil {
		return nil
	}

	elementType := types.Unalias(targetType)
	if slice, ok := elementType.Underlying().(*types.Slice); ok {
		elementType = types.Unalias(slice.Elem())
	}

	if named, ok := elementType.(*types.Named); ok {
		switch {
		case isCongaByteSize(named.Obj()):
			return spec.ValidateScalarRange(rangeSpec, "byte size", parseByteSizeBound(fieldTags))
		case isTimeDuration(named):
			return spec.ValidateScalarRange(rangeSpec, "duration", time.ParseDuration)
		case isTextRangeType(named):
			return nil
		}
	}

	basic, ok := elementType.Underlying().(*types.Basic)
	if !ok {
		return fmt.Errorf("%w: range tag is not supported on type %s", spec.ErrInvalidRangeSyntax, types.TypeString(elementType, shortPackageQualifier))
	}

	switch basic.Kind() {
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
		return spec.ValidateScalarRange(rangeSpec, "integer", func(raw string) (int64, error) {
			return strconv.ParseInt(raw, 10, 64)
		})
	case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		return spec.ValidateScalarRange(rangeSpec, "unsigned integer", func(raw string) (uint64, error) {
			return strconv.ParseUint(raw, 10, 64)
		})
	case types.Float32, types.Float64:
		return spec.ValidateScalarRange(rangeSpec, "float", func(raw string) (float64, error) {
			return strconv.ParseFloat(raw, 64)
		})
	default:
		return fmt.Errorf("%w: range tag is not supported on type %s", spec.ErrInvalidRangeSyntax, types.TypeString(elementType, shortPackageQualifier))
	}
}

func (w *walker) flagTargetType(fieldObject types.Object) types.Type {
	if w.congaWrapperName(fieldObject.Type()) == "Counter" {
		return types.Typ[types.Int]
	}

	return w.containerInnerType(fieldObject.Type())
}

// containerInnerType extracts the type parameter T of a generic container
// field, mirroring the runtime extractGenericType.
func (w *walker) containerInnerType(fieldType types.Type) types.Type {
	containerType := fieldType
	if pointer, ok := containerType.(*types.Pointer); ok {
		containerType = pointer.Elem()
	}

	named, ok := containerType.(*types.Named)
	if !ok {
		return containerType
	}

	if arguments := named.TypeArgs(); arguments != nil && arguments.Len() > 0 {
		return arguments.At(0)
	}

	if valueObject, _, _ := types.LookupFieldOrMethod(named, true, w.conga.packageObject, "value"); valueObject != nil {
		if valueField, ok := valueObject.(*types.Var); ok {
			return valueField.Type()
		}
	}

	return containerType
}

func (w *walker) predictorOptions() spec.ValidationOptions {
	options := w.options.strictValidationOptions()
	if w.options.registeredNamesAmbiguous {
		options.StrictTagValidation = false
	}

	return options
}

// isFeatureScalarType mirrors the runtime feature-field kind check: a feature
// field must have a bool or an integer underlying kind. Uintptr is excluded to
// match the runtime switch, which accepts only Uint..Uint64.
func isFeatureScalarType(targetType types.Type) bool {
	basic, ok := targetType.Underlying().(*types.Basic)
	if !ok {
		return false
	}

	switch basic.Kind() {
	case types.Bool,
		types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		return true
	default:
		return false
	}
}

func isBasicKind(targetType types.Type, kind types.BasicKind) bool {
	basic, ok := targetType.Underlying().(*types.Basic)

	return ok && basic.Kind() == kind
}

// isCongaByteSize reports whether the type name is the conga.ByteSize domain
// type: package-path qualified so foreign types sharing the name are not
// misclassified.
func isCongaByteSize(object *types.TypeName) bool {
	return object != nil && object.Pkg() != nil &&
		object.Pkg().Path() == congaPackagePath && object.Name() == "ByteSize"
}

// isTextRangeType reports whether the runtime would accept range bounds on the
// named type: its pointer must implement encoding.TextUnmarshaler and the type
// must expose the ordered projection the runtime requires (Bytes() int64 or a
// numeric underlying kind). The grammar itself is opaque to static analysis, so
// bound parsing is deferred to schema build.
func isTextRangeType(named *types.Named) bool {
	return types.Implements(types.NewPointer(named), textUnmarshalerType()) && hasOrderedProjection(named)
}

// hasOrderedProjection mirrors spec.TextRangeDomain's projection rules: a
// Bytes() int64 method on the pointer, or a signed, unsigned, or float
// underlying kind.
func hasOrderedProjection(named *types.Named) bool {
	if method, _, _ := types.LookupFieldOrMethod(types.NewPointer(named), true, nil, "Bytes"); method != nil {
		if function, ok := method.(*types.Func); ok {
			if signature, ok := function.Type().(*types.Signature); ok &&
				signature.Params().Len() == 0 &&
				signature.Results().Len() == 1 &&
				types.Identical(signature.Results().At(0).Type(), types.Typ[types.Int64]) {
				return true
			}
		}
	}

	basic, ok := named.Underlying().(*types.Basic)
	if !ok {
		return false
	}

	switch basic.Kind() {
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
		types.Float32, types.Float64:
		return true
	default:
		return false
	}
}

// textUnmarshalerType builds the encoding.TextUnmarshaler interface statically:
// UnmarshalText(text []byte) error.
func textUnmarshalerType() *types.Interface {
	errorType := types.Universe.Lookup("error").Type()

	signature := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(types.NewParam(token.NoPos, nil, "text", types.NewSlice(types.Typ[types.Byte]))),
		types.NewTuple(types.NewParam(token.NoPos, nil, "", errorType)),
		false,
	)

	return types.NewInterfaceType(
		[]*types.Func{types.NewFunc(token.NoPos, nil, "UnmarshalText", signature)},
		nil,
	).Complete()
}

// parseByteSizeBound returns a parser for conga.ByteSize range bounds using the
// runtime grammar compiled into the analyzer (conga is already linked for the
// built-in tag specs) together with the owning field's declared tags, so bound
// acceptance matches runtime binding and schema build. spec.ValidateScalarRange
// derives the diagnostic from the raw bound, keeping messages verbatim.
func parseByteSizeBound(fieldTags spec.FieldTags) func(string) (int64, error) {
	return func(raw string) (int64, error) {
		var byteSize conga.ByteSize
		if err := byteSize.UnmarshalTextWithContext([]byte(raw), conga.DecodeContext{Tags: fieldTags}); err != nil {
			return 0, err
		}

		return byteSize.Bytes(), nil
	}
}

func isTimeDuration(named *types.Named) bool {
	object := named.Obj()

	return object != nil && object.Pkg() != nil && object.Pkg().Path() == "time" && object.Name() == "Duration"
}

func shortPackageQualifier(packageObject *types.Package) string {
	if packageObject == nil {
		return ""
	}

	return packageObject.Name()
}

func strictBoolTagError(tag reflect.StructTag) error {
	_, _, err := spec.ParseStrictBoolTag(tag, "safe")

	return err
}

func summaryTagError(tag reflect.StructTag) error {
	_, err := spec.ParseSummaryTag(tag, "")

	return err
}

func flagShortRune(tag reflect.StructTag) rune {
	if shortTag := tag.Get("short"); shortTag != "" {
		shortRune, _ := utf8.DecodeRuneInString(shortTag)

		return shortRune
	}

	return 0
}

func parseNodeDirectives(tag reflect.StructTag) spec.Directives {
	dirs, err := spec.ParseCongaDirectives(tag.Get("conga"))
	if err != nil {
		return spec.Directives{}
	}

	return dirs
}

// threadDirectiveOptions mirrors the runtime subcommand option threading:
// a conga:"allow-unknown-tags" directive on the subcommand field enables
// unknown tags for the child, recurse extends it to the whole subtree, and
// otherwise the inherited state resets unless a parent recursion is active.
func threadDirectiveOptions(node *commandNode, dirs spec.Directives) (bool, bool) {
	if dirs.AllowUnknownTags {
		return true, dirs.Recurse
	}

	if !node.allowRecurse {
		return false, false
	}

	return node.allowUnknown, node.allowRecurse
}

// rootDirectiveOptions mirrors the runtime scanSentinelDirectives: a
// conga:"allow-unknown-tags" directive on any root field enables unknown
// tags for the whole root struct, with recurse extending it downward.
func rootDirectiveOptions(structType *types.Struct) (bool, bool) {
	var (
		allowUnknown bool
		allowRecurse bool
	)

	for index := range structType.NumFields() {
		dirs, err := spec.ParseCongaDirectives(reflect.StructTag(structType.Tag(index)).Get("conga"))
		if err != nil || !dirs.AllowUnknownTags {
			continue
		}

		allowUnknown = true
		allowRecurse = allowRecurse || dirs.Recurse
	}

	return allowUnknown, allowRecurse
}
