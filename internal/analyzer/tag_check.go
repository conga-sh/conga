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

	err := spec.ValidateFieldTags(tag, role, candidates, w.options.validationOptionsFor(node))
	w.reportFieldError(node, fieldObject, err)

	return err == nil
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

	return w.buildChildNode(node, fieldObject, tag)
}

// checkFeatureExpression mirrors the runtime feature-tag handling: a declared
// feature expression under an app that never calls EnableFeatures is rejected,
// and an enabled app validates the expression syntax. The runtime schema
// builder validates subcommand, flag, and config-file flag feature tags through
// the same shared validators, so the analyzer reports the identical diagnostic.
func (w *walker) checkFeatureExpression(node *commandNode, fieldObject types.Object, tag reflect.StructTag) {
	featureExpr := strings.TrimSpace(tag.Get("feature"))
	if featureExpr == "" {
		return
	}

	if w.featureState == optInDisabled {
		w.reportFieldError(node, fieldObject, spec.FeatureTagRequiresEnableError())

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
// validation runs first (an invalid field short-circuits the guard, exactly
// like the runtime), and undecided per-anchor evidence leaves the diagnostic
// suppressed.
func (w *walker) checkCompletionCommandField(node *commandNode, fieldObject types.Object, tag reflect.StructTag) {
	if !w.validateNodeFieldTags(node, fieldObject, tag, "completion command", nil) {
		return
	}

	w.reportFieldError(node, fieldObject, summaryTagError(tag))

	if w.completionState == optInDisabled {
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
	w.recordFlagEnvOwners(node, tag, flagName, true)
}

// checkFeaturesField validates a root feature-declaration field
// (conga.Features[T]): the container tag vocabulary and, for every exported
// inner field, the per-feature tag vocabulary, the scalar kind, and the
// enum/default contract through the shared spec validators. Feature-name
// reference validation is left to schema build, which has the fully reflected
// declaration set. A CLI-exposed container registers its synthesized flag name
// in the branch shadow maps, and a second container on the root reports the
// duplicate-container error.
func (w *walker) checkFeaturesField(node *commandNode, fieldObject types.Object, tag reflect.StructTag, branchNames map[string]string) {
	if !w.validateNodeFieldTags(node, fieldObject, tag, "features", nil) {
		return
	}

	if w.featureState == optInDisabled {
		w.reportFieldError(node, fieldObject, spec.FeaturesRequiresEnableError())

		return
	}

	if !node.isRoot {
		w.reportFieldError(node, fieldObject, spec.FeaturesMustBeOnRootError())

		return
	}

	if node.featuresDeclared {
		w.reportFieldError(node, fieldObject, spec.FeaturesAlreadyDeclaredError(node.featureFlagName))

		return
	}

	innerType := w.containerInnerType(fieldObject.Type())

	structType, ok := innerType.Underlying().(*types.Struct)
	if !ok {
		w.reportFieldError(node, fieldObject, spec.FeaturesMustBeStructParameterError())

		return
	}

	// The runtime installs the synthesized flag only after every inner field
	// validates, so a failed container neither reserves the flag name against
	// descendants nor counts as the first declaration.
	valid, envDeclarer := w.checkFeatureFields(node, innerType, structType)
	if !valid {
		return
	}

	envConflict := w.checkFeatureEnvSurfaceConflicts(node, fieldObject, tag, envDeclarer)

	flagName := strings.TrimSpace(tag.Get("flag"))
	noCLI := flagName == "" || flagName == "-"

	if noCLI {
		flagName = casing.Kebab(fieldObject.Name())
	}

	node.featuresDeclared = true
	node.featureFlagName = flagName

	if !noCLI {
		w.checkAndCollectSynthesizedFlag(node, fieldObject, flagName, branchNames)
	}

	if !envConflict {
		w.recordFeatureEnvBindings(node, fieldObject, tag, structType, flagName)
	}
}

// checkFeatureFields validates every exported inner field of a feature struct
// and reports whether the container is valid. It also returns the first valid
// field whose env tag declares a variable, so surface validation and the env
// binding pass need no additional scan for the declarer.
func (w *walker) checkFeatureFields(node *commandNode, innerType types.Type, structType *types.Struct) (bool, string) {
	innerStructName := ""
	if named, ok := types.Unalias(innerType).(*types.Named); ok {
		innerStructName = named.Obj().Name()
	}

	var (
		envDeclarer string
		valid       = true
	)

	seen := make(map[string]bool)

	// stagedFields accumulates the fields that survived validation so the
	// all-or-nothing stage guard can report the first stage-less field after
	// the whole struct is inspected.
	type stagedField struct {
		obj   types.Object
		name  string
		stage string
	}

	var stagedFields []stagedField

	for index := range structType.NumFields() {
		innerField := structType.Field(index)
		if !innerField.Exported() {
			continue
		}

		innerTag := reflect.StructTag(structType.Tag(index))

		if !w.validateNodeFieldTags(node, innerField, innerTag, "feature", nil) {
			valid = false

			continue
		}

		field, ok := featureField(innerField.Type(), w.pass.TypesSizes)
		if !ok {
			w.reportFieldError(node, innerField, spec.FeatureFieldTypeError(innerField.Name(), innerField.Type().String()))

			valid = false

			continue
		}

		name, err := spec.FeatureFieldName(innerField.Name(), innerTag)
		w.reportFieldError(node, innerField, err)

		if err != nil {
			valid = false

			continue
		}

		if seen[name] {
			w.reportFieldError(node, innerField, spec.DuplicateFeatureError(name, innerStructName))

			valid = false

			continue
		}

		seen[name] = true
		field.Name = name

		if err := spec.ValidateFeatureFieldTags(field, innerTag); err != nil {
			w.reportFieldError(node, innerField, err)

			valid = false

			continue
		}

		if envDeclarer == "" && spec.FeatureFieldDeclaresEnv(innerTag) {
			envDeclarer = innerField.Name()
		}

		stagedFields = append(stagedFields, stagedField{obj: innerField, name: name, stage: field.Stage})
	}

	if slices.ContainsFunc(stagedFields, func(entry stagedField) bool { return entry.stage != "" }) {
		for _, entry := range stagedFields {
			if entry.stage == "" {
				w.reportFieldError(node, entry.obj, spec.FeatureStageAllOrNothingError(entry.name))

				valid = false

				break
			}
		}
	}

	return valid, envDeclarer
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
func (w *walker) checkFlagField(node *commandNode, fieldObject types.Object, tag reflect.StructTag, branchNames map[string]string, branchShorts map[rune]string, branchGroups map[groupPair]string) {
	targetType := w.flagTargetType(fieldObject)
	tagSpecs := w.congaFieldTagSpecs(targetType)

	if !w.validateNodeFieldTags(node, fieldObject, tag, "flag", tagSpecs) {
		return
	}

	wrapperName := w.congaWrapperName(fieldObject.Type())

	guardRejected := false

	if wrapperName == "Counter" {
		if err := spec.ValidateCounterDefaultTag(tag); err != nil {
			guardRejected = true

			w.reportFieldError(node, fieldObject, err)
		}
	}

	if w.checkFlagValueGuards(node, fieldObject, tag, wrapperName) {
		guardRejected = true
	}

	flagName := cmp.Or(tag.Get("name"), casing.Kebab(fieldObject.Name()))

	// A flag that shares both its oneof group and its and group with another
	// member can never be supplied: selecting it forces the other member, which
	// then violates the exactly-one rule.
	w.checkOneOfAndOverlap(node, fieldObject, tag, flagName, wrapperName == "RequiredFlag", branchGroups)

	// Flags the runtime rejects before the built-in short guard are never
	// checked for a collision, so skip them here as well.
	if !guardRejected {
		w.checkBuiltinShortConflict(node, fieldObject, tag, flagName)
	}

	// Feature-tag syntax is validated before the required-flag guard, mirroring
	// the runtime parse order: an invalid expression reports its syntax error
	// instead of the required-flag rejection. When features are disabled, the
	// runtime rejects the tag before parsing it, so the enable guard wins.
	featureExpr := strings.TrimSpace(tag.Get("feature"))
	if featureExpr != "" {
		if w.featureState == optInDisabled {
			w.reportFieldError(node, fieldObject, spec.FeatureTagRequiresEnableError())
		} else {
			syntaxErr := spec.ValidateFeatureExpression(featureExpr)
			w.reportFieldError(node, fieldObject, syntaxErr)

			if syntaxErr == nil && wrapperName == "RequiredFlag" {
				w.reportFieldError(node, fieldObject, spec.FeatureOnRequiredFlagError())
			}
		}
	}

	w.reportFieldError(node, fieldObject, spec.ValidatePredictorTag(tag, isBasicKind(targetType, types.Bool), w.options.registeredNames, !w.options.registeredNamesAmbiguous))
	w.checkBootstrapField(node, fieldObject, tag, targetType)
	w.checkValidationSpecs(node, fieldObject, tag, targetType, tagSpecs)

	if _, err := spec.ParseFieldTags(tag, tagSpecs); err != nil {
		w.reportFieldError(node, fieldObject, err)
	}

	w.reportFieldError(node, fieldObject, strictBoolTagError(tag))

	w.checkAndCollectFlag(node, fieldObject, tag, flagName, branchNames, branchShorts)
	w.recordFlagEnvOwners(node, tag, flagName, false)
}

// checkFlagValueGuards reports the value-tag guards the runtime applies while
// building a flag and reports whether one rejected it. Required flags reject
// fallback, deprecation, and group tags; optional oneof members reject a
// default.
func (w *walker) checkFlagValueGuards(node *commandNode, fieldObject types.Object, tag reflect.StructTag, wrapperName string) bool {
	_, hasDefault := tag.Lookup("default")
	_, hasDeprecated := tag.Lookup("deprecated")
	_, hasXor := tag.Lookup("xor")
	_, hasOneOf := tag.Lookup("oneof")

	if wrapperName == "RequiredFlag" {
		switch {
		case hasDeprecated:
			w.reportFieldError(node, fieldObject, spec.DeprecatedOnRequiredFlagError())

			return true
		case hasDefault:
			w.reportFieldError(node, fieldObject, spec.DefaultOnRequiredFlagError())

			return true
		case hasXor:
			w.reportFieldError(node, fieldObject, spec.GroupTagOnRequiredFlagError("xor"))

			return true
		case hasOneOf:
			w.reportFieldError(node, fieldObject, spec.GroupTagOnRequiredFlagError("oneof"))

			return true
		}

		return false
	}

	if hasOneOf && hasDefault {
		w.reportFieldError(node, fieldObject, spec.DefaultOnOneOfFlagError())

		return true
	}

	return false
}

// checkOneOfAndOverlap rejects a flag that shares both its oneof group and its
// and group with another member of the same exactly-one group: selecting either
// member forces the other, which then violates the exactly-one rule. Flags that
// already failed a schema guard above are never built at runtime, so they are
// not paired here.
func (w *walker) checkOneOfAndOverlap(node *commandNode, fieldObject types.Object, tag reflect.StructTag, flagName string, required bool, branchGroups map[groupPair]string) {
	if required {
		return
	}

	oneOfGroup := tag.Get("oneof")
	andGroup := tag.Get("and")

	if oneOfGroup == "" || andGroup == "" {
		return
	}

	if _, hasDefault := tag.Lookup("default"); hasDefault {
		return
	}

	pair := groupPair{oneOf: oneOfGroup, and: andGroup}
	if firstFlag, ok := branchGroups[pair]; ok {
		w.reportFieldError(node, fieldObject, spec.OneOfAndOverlapError(oneOfGroup, andGroup, firstFlag, flagName))

		return
	}

	branchGroups[pair] = flagName
}

// checkBuiltinShortConflict rejects a user-declared short that claims the rune
// assigned to a built-in the command offers, mirroring the runtime schema
// guard: the built-in --help flag is injected on every command that offers
// help, --version is root-only, and a disabled built-in leaves its rune free.
// Ambiguous rune evidence suppresses the check instead of inventing it, and
// packages anchoring more than one conga.New root skip it entirely: the rune
// evidence is aggregated package-wide, so it cannot be attributed to a
// specific root without risking a diagnostic on an innocent sibling — the
// analyzer may only miss diagnostics, never invent them. The runtime schema
// guard stays per-root and remains the source of truth for multi-root packages.
func (w *walker) checkBuiltinShortConflict(node *commandNode, fieldObject types.Object, tag reflect.StructTag, flagName string) {
	if !w.singleAnchor {
		return
	}

	short := spec.ParseFlagShort(tag)
	if short == 0 {
		return
	}

	if !w.options.helpShortAmbiguous && w.options.helpShort != 0 && !w.options.disableHelp && short == w.options.helpShort {
		w.reportFieldError(node, fieldObject, spec.BuiltinShortConflictError("help", short, flagName))

		return
	}

	if !w.options.versionShortAmbiguous && w.options.versionShort != 0 && !w.options.disableVersion && node.isRoot && short == w.options.versionShort {
		w.reportFieldError(node, fieldObject, spec.BuiltinShortConflictError("version", short, flagName))
	}
}

// checkBuiltinShortPair mirrors the runtime root-level guard: assigning the
// same rune to both built-ins while both are offered leaves the second
// registration dropped while the help screen still advertises it. Like
// checkBuiltinShortConflict, package-wide rune evidence suppresses the check in
// packages anchoring more than one conga.New root rather than risking a
// diagnostic on an innocent sibling.
func (w *walker) checkBuiltinShortPair(node *commandNode) {
	if !w.singleAnchor || !node.isRoot {
		return
	}

	if w.options.helpShortAmbiguous || w.options.versionShortAmbiguous ||
		w.options.helpShort == 0 || w.options.helpShort != w.options.versionShort ||
		w.options.disableHelp || w.options.disableVersion {
		return
	}

	position := node.reportPos
	if node.inPackage {
		position = node.named.Obj().Pos()
	}

	w.report(position, spec.BuiltinShortPairError(w.options.helpShort).Error())
}

// checkArgField validates a positional argument field.
func (w *walker) checkArgField(node *commandNode, fieldObject types.Object, tag reflect.StructTag) {
	targetType := w.containerInnerType(fieldObject.Type())
	tagSpecs := w.congaFieldTagSpecs(targetType)

	if !w.validateNodeFieldTags(node, fieldObject, tag, "positional argument", tagSpecs) {
		return
	}

	w.reportFieldError(node, fieldObject, spec.ValidatePredictorTag(tag, false, w.options.registeredNames, !w.options.registeredNamesAmbiguous))
	w.checkBootstrapField(node, fieldObject, tag, targetType)
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

// checkBootstrapField mirrors the runtime bootstrap-tag guards: a declared
// bootstrap tag requires the command or an ancestor command to implement
// conga.Bootstrapper, cannot carry a feature expression, and must target a
// scalar, slice-of-scalar, or map-of-scalar type.
func (w *walker) checkBootstrapField(node *commandNode, fieldObject types.Object, tag reflect.StructTag, targetType types.Type) {
	if !spec.BootstrapTagged(tag) {
		return
	}

	if !node.hasBootstrapper {
		w.reportFieldError(node, fieldObject, spec.BootstrapRequiresBootstrapperError())

		return
	}

	if strings.TrimSpace(tag.Get("feature")) != "" {
		w.reportFieldError(node, fieldObject, spec.BootstrapFeatureConflictError())

		return
	}

	if targetType != nil && !isStaticBootstrapType(targetType) {
		w.reportFieldError(node, fieldObject, spec.BootstrapFieldTypeError())
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

// isStaticBootstrapType mirrors spec.isBootstrapType over go/types.
func isStaticBootstrapType(targetType types.Type) bool {
	return isStaticBootstrapTypeVisiting(targetType, make(map[types.Type]bool))
}

// isStaticBootstrapTypeVisiting mirrors isBootstrapTypeVisiting over go/types,
// rejecting self-referential slice and map shapes instead of recursing until
// the analyzer overflows its stack.
func isStaticBootstrapTypeVisiting(targetType types.Type, visiting map[types.Type]bool) bool {
	if targetType == nil {
		return false
	}

	switch underlying := targetType.Underlying().(type) {
	case *types.Basic:
		if underlying.Kind() == types.Uintptr {
			return false
		}

		info := underlying.Info()

		return info&(types.IsBoolean|types.IsString|types.IsInteger|types.IsFloat) != 0
	case *types.Slice:
		if visiting[targetType] {
			return false
		}

		visiting[targetType] = true
		defer delete(visiting, targetType)

		return isStaticBootstrapTypeVisiting(underlying.Elem(), visiting)
	case *types.Map:
		if visiting[targetType] {
			return false
		}

		visiting[targetType] = true
		defer delete(visiting, targetType)

		return isStaticBootstrapTypeVisiting(underlying.Key(), visiting) &&
			isStaticBootstrapTypeVisiting(underlying.Elem(), visiting)
	default:
		return false
	}
}

// featureField mirrors the runtime feature-field kind switch: a feature field
// must have a bool or an integer underlying kind (Uintptr is excluded to match
// the runtime switch, which accepts only Uint..Uint64), and the reconstructed
// metadata feeds the shared enum/default validators so lint-time and runtime
// diagnostics stay identical. Bit sizes come from the package type sizes; when
// they are unavailable the zero bit size falls back to the platform width,
// which can only miss narrow-width overflow diagnostics.
func featureField(targetType types.Type, sizes types.Sizes) (*spec.FeatureField, bool) {
	basic, ok := targetType.Underlying().(*types.Basic)
	if !ok {
		return nil, false
	}

	var (
		isInt      bool
		isUnsigned bool
	)

	switch basic.Kind() {
	case types.Bool:
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
		isInt = true
	case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		isInt = true
		isUnsigned = true
	default:
		return nil, false
	}

	field := &spec.FeatureField{IsInt: isInt, IsUnsigned: isUnsigned}

	if isInt && sizes != nil {
		field.BitSize = int(sizes.Sizeof(targetType)) * 8
	}

	return field, true
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
