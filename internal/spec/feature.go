package spec

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/slicex"
)

// Feature stage vocabulary of the stage struct tag, mirroring Kubernetes'
// feature maturity levels. The stage is declared per feature field; when any
// field declares one, every field must.
const (
	FeatureStageAlpha      = "alpha"
	FeatureStageBeta       = "beta"
	FeatureStageGA         = "ga"
	FeatureStageDeprecated = "deprecated"
)

// featureFlagDefaultHelp is the synthesized feature flag's default help text.
// It deliberately does not enumerate the features: the root help screen's
// Features section lists them, and a custom help tag can inline them through
// {{enum}}.
const featureFlagDefaultHelp = "Enable or disable features (name=value)"

// FeatureField describes one field of the root conga.Features[T] struct: a
// single feature toggle.
type FeatureField struct {
	// Name is the normalized kebab-case feature name (the map key).
	Name string
	// Help is the human-readable description shown in the root help screen's
	// Features section and in completion candidate descriptions.
	Help string
	// Stage is the declared feature maturity: FeatureStageAlpha,
	// FeatureStageBeta, FeatureStageGA, or FeatureStageDeprecated, or "" when
	// the stage tag is absent on every feature field. GA and deprecated
	// features are omitted from the help screen's Features section.
	Stage string
	// IsInt reports whether the feature is a leveled integer toggle (0 = off).
	IsInt bool
	// IsUnsigned reports whether the feature's integer field is an unsigned
	// kind, so negative values are rejected at canonicalization.
	IsUnsigned bool
	// BitSize is the storage width of the feature's integer field in bits; zero
	// for bool features.
	BitSize int
	// Enum lists the permitted non-zero values of an integer feature, in
	// declaration order. Empty for bool features.
	Enum []string
	// Default is the canonical default value ("true"/"false" for bools, a decimal
	// string for integers), or "" when no default tag is declared.
	Default string
	// EnvVars lists the environment variables the feature binds in declaration
	// order: the env tag names when present, else the name derived from the
	// container's envprefix tag. Empty when the field binds no environment
	// variable.
	EnvVars []string
	// EnvExcluded reports that the feature binds no environment variable,
	// whether through an env:"-" tag or the absence of both an env tag and a
	// container envprefix.
	EnvExcluded bool
	// FieldIndex is the index of the feature field inside the inner feature struct.
	FieldIndex int
}

// ListedInHelp reports whether the feature toggle is rendered in the root help
// screen's Features section: undeclared, alpha, and beta stages are listed,
// while GA and deprecated features — no longer experimental — are omitted.
func (f *FeatureField) ListedInHelp() bool {
	return f.Stage != FeatureStageGA && f.Stage != FeatureStageDeprecated
}

// Domain returns the canonical value domain for the feature toggle:
// "0" followed by the enum values for an integer feature, or "true" and "false"
// for a boolean feature.
func (f *FeatureField) Domain() []string {
	if f.IsInt {
		return slices.Concat([]string{"0"}, f.Enum)
	}

	return []string{"true", "false"}
}

// Canonicalize coerces a raw feature value to its canonical form: "true" or
// "false" for bool features, and a decimal string for integer features. It
// rejects values outside the feature's declared domain, negative values of
// unsigned fields, and values that do not fit the field's storage width.
func (f *FeatureField) Canonicalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)

	if f.IsInt {
		canonical, err := f.parseInteger(raw)
		if err != nil {
			return "", err
		}

		if canonical != "0" && !slices.Contains(f.Enum, canonical) {
			return "", invalidFeatureEnumError(f)
		}

		return canonical, nil
	}

	b, err := strconv.ParseBool(raw)
	if err != nil {
		return "", errFeatureMustBeBoolean
	}

	return strconv.FormatBool(b), nil
}

// MatchFeature reports whether featureExpr is satisfied by activeFeatures.
// featureExpr is a comma-separated conjunction of bare feature names, each
// optionally negated with a leading "!"; a feature is on when activeFeatures
// marks it true. The expression is parsed on every call, so schema build stores
// the [NormalizeFeatureExpression] form on flags and commands (no AST cache is
// kept); direct callers may pass a raw expression unchanged.
func MatchFeature(featureExpr string, activeFeatures map[string]bool) bool {
	if featureExpr == "" {
		return true
	}

	for rawPart := range strings.SplitSeq(featureExpr, ",") {
		if !matchSingleFeature(rawPart, activeFeatures) {
			return false
		}
	}

	return true
}

// ValidateFeatureExpression validates that featureExpr conforms to valid feature tag syntax.
func ValidateFeatureExpression(featureExpr string) error {
	if featureExpr == "" {
		return nil
	}

	for rawPart := range strings.SplitSeq(featureExpr, ",") {
		if _, ok := parseFeatureTerm(rawPart); !ok {
			return fmt.Errorf("%w: invalid feature tag syntax %q", ErrInvalidStructTag, featureExpr)
		}
	}

	return nil
}

// NormalizeFeatureExpression canonicalizes an already-valid feature expression
// to its stored form: comma-separated terms with surrounding whitespace trimmed
// and empty terms dropped. Schema build validates the raw expression first and
// stores the normalized result on flags and commands, so display sites (help,
// --why, error messages) show one canonical spelling. Direct callers of
// [MatchFeature] are unaffected: it still accepts a raw expression.
func NormalizeFeatureExpression(featureExpr string) string {
	if featureExpr == "" {
		return ""
	}

	terms := make([]string, 0, strings.Count(featureExpr, ",")+1)

	for rawPart := range strings.SplitSeq(featureExpr, ",") {
		term, ok := parseFeatureTerm(rawPart)
		if !ok {
			continue
		}

		formatted := term.name
		if term.negated {
			formatted = "!" + formatted
		}

		terms = append(terms, formatted)
	}

	return strings.Join(terms, ",")
}

// FeatureFieldName derives the canonical kebab-case feature name of an inner
// feature struct field: the name tag when present, else the Go field name. The
// runtime schema builder and the static analyzer share it as the single source
// of truth for duplicate and empty-name diagnostics.
func FeatureFieldName(fieldName string, tag reflect.StructTag) (string, error) {
	name := casing.Kebab(cmp.Or(strings.TrimSpace(tag.Get("name")), fieldName))
	if name == "" {
		return "", fmt.Errorf("%w: feature field %s declares an empty name", ErrInvalidStructTag, fieldName)
	}

	return name, nil
}

// ValidateFeatureFieldTags validates and records the enum, default, and stage
// tags of one feature field: enum is valid only on integer features and may
// not be empty, every enum value must canonicalize within the feature's domain
// and storage width, an integer feature requires at least one enum level, a
// declared default must canonicalize against the enum domain, and a declared
// stage must name one of the four maturity levels. The runtime schema builder
// reports it from the reflected field and the static analyzer reconstructs the
// same metadata from go/types, keeping the diagnostics a single source of
// truth.
func ValidateFeatureFieldTags(field *FeatureField, tag reflect.StructTag) error {
	if stageTag, hasStage := tag.Lookup("stage"); hasStage {
		stage := strings.TrimSpace(stageTag)
		if stage == "" {
			return fmt.Errorf("%w: tag \"stage\" cannot be empty", ErrInvalidStructTag)
		}

		switch stage {
		case FeatureStageAlpha, FeatureStageBeta, FeatureStageGA, FeatureStageDeprecated:
			field.Stage = stage
		default:
			return fmt.Errorf(`%w: tag "stage" must be one of %s, %s, %s, or %s, got %q`, ErrInvalidStructTag, FeatureStageAlpha, FeatureStageBeta, FeatureStageGA, FeatureStageDeprecated, stage)
		}
	}

	if enumTag, hasEnum := tag.Lookup("enum"); hasEnum {
		if !field.IsInt {
			return fmt.Errorf("%w: tag \"enum\" is only valid on integer features", ErrInvalidStructTag)
		}

		if err := validateFeatureEnum(field, enumTag); err != nil {
			return err
		}
	}

	if defaultTag, hasDefault := tag.Lookup("default"); hasDefault {
		canonical, err := field.Canonicalize(defaultTag)
		if err != nil {
			return fmt.Errorf("%w: feature %q default %q: %w", ErrInvalidStructTag, field.Name, defaultTag, err)
		}

		field.Default = canonical
	}

	if field.IsInt && len(field.Enum) == 0 {
		return fmt.Errorf("%w: feature %q is an integer feature and requires an enum tag listing its non-zero levels", ErrInvalidStructTag, field.Name)
	}

	return nil
}

// DuplicateFeatureError builds the schema error for two feature fields that
// normalize to the same kebab-case name. The runtime schema builder and the
// static analyzer share it as the single source of truth.
func DuplicateFeatureError(name string, typeName string) error {
	return fmt.Errorf("%w: feature %q (already declared in %s)", ErrDuplicateFlag, name, typeName)
}

// FeatureStageAllOrNothingError builds the schema error for a feature field
// that omits the stage tag while a sibling declares one. The runtime schema
// builder and the static analyzer share it as the single source of truth.
func FeatureStageAllOrNothingError(fieldName string) error {
	return fmt.Errorf("%w: feature field %s does not declare the stage tag; declare stage on every feature field or on none", ErrInvalidStructTag, fieldName)
}

// ParseFeatureTag parses, validates, and normalizes the "feature" struct tag expression.
func ParseFeatureTag(feature string, required bool) (string, error) {
	if required {
		return "", FeatureOnRequiredFlagError()
	}

	feature = strings.TrimSpace(feature)
	if feature == "" {
		return "", nil
	}

	if err := ValidateFeatureExpression(feature); err != nil {
		return "", err
	}

	return NormalizeFeatureExpression(feature), nil
}

// ProcessFeaturesField adapts processFeaturesField for the spec.Options hook.
func ProcessFeaturesField(builder any, structField reflect.StructField, fieldHandle any, classification container.Classification) error {
	b, ok := builder.(*schemaBuilder)
	if !ok {
		panic("spec: invalid schemaBuilder passed to ProcessFeaturesField")
	}

	return b.processFeaturesField(structField, fieldHandle, classification)
}

// ValidateFeatureReferences rejects feature expressions that reference a feature
// not declared through the root conga.Features container.
func ValidateFeatureReferences(root *Command) error {
	return validateFeatureReferences(root)
}

// ParseFeatureContainerEnv resolves the combined map variable's environment
// names from the container tag: explicit env names only, with env:"-" and
// tag-less containers excluded from the environment tier entirely. Features
// never derive environment variables from any prefix. The runtime schema
// builder and the static analyzer share it as the single source of truth.
func ParseFeatureContainerEnv(containerTag reflect.StructTag) ([]string, bool) {
	envTag, hasEnvTag := containerTag.Lookup("env")
	if !hasEnvTag || strings.TrimSpace(envTag) == "-" {
		return nil, true
	}

	envVars := SplitTagList(containerTag, "env")
	if len(envVars) == 0 {
		return nil, true
	}

	return envVars, false
}

// ValidateFeatureEnvSurfaces rejects ambiguous env declarations on the
// feature container: env and envprefix name different env surfaces (the
// combined map variable versus per-feature variables), and a field env tag
// under a combined-map container would create a second env surface writing
// the same keys. An env tag whose value is "-" or carries no names is
// absence, so it never conflicts. The runtime schema builder and the static
// analyzer share it as the single source of truth.
func ValidateFeatureEnvSurfaces(containerTag reflect.StructTag, envDeclarer string) error {
	_, envExcluded := ParseFeatureContainerEnv(containerTag)
	if envExcluded {
		return nil
	}

	if strings.TrimSpace(containerTag.Get("envprefix")) != "" {
		return FeatureEnvModeConflictError()
	}

	if envDeclarer != "" {
		return FeatureFieldEnvWithContainerEnvError(envDeclarer)
	}

	return nil
}

type featureTerm struct {
	name    string
	negated bool
}

// parseInteger parses raw into the canonical decimal string for the feature's
// integer kind, enforcing non-negativity for unsigned kinds and the field's
// storage width for both.
func (f *FeatureField) parseInteger(raw string) (string, error) {
	if f.IsUnsigned {
		parsed, err := strconv.ParseUint(raw, 10, f.BitSize)
		if err != nil {
			return "", featureIntegerParseError(err, false)
		}

		return strconv.FormatUint(parsed, 10), nil
	}

	parsed, err := strconv.ParseInt(raw, 10, f.BitSize)
	if err != nil {
		return "", featureIntegerParseError(err, true)
	}

	return strconv.FormatInt(parsed, 10), nil
}

// processFeaturesField validates and reflects the root feature-declaration
// container (conga.Features[T]) into per-feature metadata and synthesizes the
// single map flag that carries feature values from the command line,
// environment, and configuration files. A conga:"-" field is ignored before
// the root-placement and duplicate checks, matching the other container
// processors.
//
// The environment surface is declared on the container in exactly one of two
// exclusive shapes: env names the combined map variable, and envprefix
// derives one variable per feature field. App EnvPrefix never reaches
// features.
func (b *schemaBuilder) processFeaturesField(structField reflect.StructField, fieldHandle any, classification container.Classification) error {
	dirs, err := b.validateFieldTags(structField, "features", featuresFieldTagCandidates)
	if err != nil {
		return err
	}

	if dirs.Ignore {
		return nil
	}

	if !b.isRoot {
		return FeaturesMustBeOnRootError()
	}

	if b.cmd.FeatureFlag != nil {
		return FeaturesAlreadyDeclaredError(b.cmd.FeatureFlag.Name)
	}

	innerType, err := featureStructType(classification.Target)
	if err != nil {
		return err
	}

	envPrefix := strings.TrimSpace(structField.Tag.Get("envprefix"))

	fields, envDeclarer, err := b.buildFeatureFields(innerType, envPrefix)
	if err != nil {
		return err
	}

	if err := ValidateFeatureEnvSurfaces(structField.Tag, envDeclarer); err != nil {
		return err
	}

	b.cmd.FeatureFields = fields
	b.cmd.featureFieldName = structField.Name
	b.cmd.featureStructName = b.structType.Name()

	return b.installFeatureFlag(structField, fieldHandle, fields)
}

// buildFeatureFields reflects the exported bool/int fields of the feature
// struct into feature metadata, deriving the feature key set from the field
// names. Each field's environment binding resolves against the container's
// envprefix: an explicit env tag names the variables, env:"-" excludes the
// field, and an untagged field derives one variable from the prefix when one
// is declared. The second result names the first field whose env tag declares
// a variable, so surface validation needs no second reflection walk.
func (b *schemaBuilder) buildFeatureFields(innerType reflect.Type, envPrefix string) ([]*FeatureField, string, error) {
	seen := make(map[string]bool)

	var (
		envDeclarer string
		fields      []*FeatureField
	)

	for index := range innerType.NumField() {
		fieldSpec := innerType.Field(index)
		if !fieldSpec.IsExported() {
			continue
		}

		dirs, err := b.validateFieldTags(fieldSpec, "feature", featureFieldTagCandidates)
		if err != nil {
			return nil, "", err
		}

		if dirs.Ignore {
			continue
		}

		field, err := b.buildSingleFeatureField(fieldSpec, innerType.Name(), index, seen, envPrefix)
		if err != nil {
			return nil, "", err
		}

		if envDeclarer == "" && FeatureFieldDeclaresEnv(fieldSpec.Tag) {
			envDeclarer = fieldSpec.Name
		}

		fields = append(fields, field)
	}

	if err := enforceFeatureStageAllOrNothing(fields); err != nil {
		return nil, "", err
	}

	return fields, envDeclarer, nil
}

func (b *schemaBuilder) buildSingleFeatureField(fieldSpec reflect.StructField, typeName string, index int, seen map[string]bool, envPrefix string) (*FeatureField, error) {
	var (
		isInt      bool
		isUnsigned bool
	)

	switch fieldSpec.Type.Kind() {
	case reflect.Bool:
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		isInt = true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		isInt = true
		isUnsigned = true
	default:
		return nil, FeatureFieldTypeError(fieldSpec.Name, fieldSpec.Type.String())
	}

	name, err := FeatureFieldName(fieldSpec.Name, fieldSpec.Tag)
	if err != nil {
		return nil, err
	}

	if seen[name] {
		return nil, DuplicateFeatureError(name, typeName)
	}

	seen[name] = true

	field := &FeatureField{
		Name:       name,
		Help:       strings.TrimSpace(fieldSpec.Tag.Get("help")),
		IsInt:      isInt,
		IsUnsigned: isUnsigned,
		FieldIndex: index,
	}

	if isInt {
		field.BitSize = fieldSpec.Type.Bits()
	}

	if err := ValidateFeatureFieldTags(field, fieldSpec.Tag); err != nil {
		return nil, err
	}

	// A feature field with no env tag and no container prefix binds nothing,
	// which is the excluded state for features: absence and env:"-" are
	// equivalent on feature fields.
	envVars, envExcludedTag := ResolveFlagEnvVars(fieldSpec.Tag, name, envPrefix)
	field.EnvVars = envVars
	field.EnvExcluded = envExcludedTag || len(envVars) == 0

	return field, nil
}

// installFeatureFlag synthesizes the single map flag that carries feature
// values across the command line, environment, and configuration surfaces, and
// stores it on the root command for feature resolution.
func (b *schemaBuilder) installFeatureFlag(structField reflect.StructField, fieldHandle any, fields []*FeatureField) error {
	name := strings.TrimSpace(structField.Tag.Get("flag"))
	noCLI := name == "" || name == "-"

	if noCLI {
		name = casing.Kebab(structField.Name)
	}

	sep, found := structField.Tag.Lookup("sep")
	if !found {
		sep = ","
	}

	// The combined map variable binds only explicit env names: Features never
	// derive environment variables from any prefix, so a container without an
	// env tag is excluded from the environment tier entirely — identical to
	// declaring env:"-".
	envVars, envExcluded := ParseFeatureContainerEnv(structField.Tag)

	configKey := strings.TrimSpace(structField.Tag.Get("config"))
	if configKey == "" {
		configKey = "-"
	}

	enumKeys := make([]string, 0, len(fields))
	for _, field := range fields {
		enumKeys = append(enumKeys, field.Name)
	}

	// {{env}} on the Features container names only the container's combined-map
	// variables: one environment surface has one display location, and the
	// per-feature variables (field env tags or envprefix-derived names) are
	// rendered next to their feature in the help screen's Features section
	// instead. The synthesized flag's own EnvVars stay binding-only, so this
	// list is never used by the environment resolution tier.
	helpText := strings.TrimSpace(structField.Tag.Get("help"))
	referenced := helpVars{}

	if helpText == "" {
		helpText = featureFlagDefaultHelp
	} else {
		var err error

		helpText, referenced, err = interpolateHelp(helpText, helpVarValuesFor(nil, nil, nil, enumKeys, "", envVars))
		if err != nil {
			return err
		}
	}

	// The root help screen's Features section lists every visible feature by
	// name, so the automatic [enum: ...] hint on the synthesized flag is
	// suppressed while any feature remains listed there. When every feature is
	// GA or deprecated (hidden from the section), the hint stays and remains
	// the only help-screen surface naming the gates.
	if slices.ContainsFunc(fields, (*FeatureField).ListedInHelp) {
		referenced.Enum = true
	}

	flagSpec := &Flag{
		Name:           name,
		Help:           helpText,
		HelpVars:       referenced,
		Sep:            sep,
		IsMap:          true,
		EngineOwned:    true,
		ConfigKey:      configKey,
		EnvVars:        envVars,
		EnvExcluded:    envExcluded,
		NoCLI:          noCLI,
		Hidden:         noCLI,
		Safe:           true,
		TargetType:     reflect.TypeFor[map[string]string](),
		EnumHintHidden: hasTag(structField.Tag, "enumhint"),
		// Enum is metadata only on the synthesized map flag: it seeds the help
		// hint and completion candidates. Map flags validate their items, not
		// the flag value, so this list never gates the map flag's binding; the
		// per-feature domains are enforced by FeatureField.Canonicalize.
		Enum:   enumKeys,
		Handle: fieldHandle,
	}

	if b.opts.FeaturePredictor != nil {
		flagSpec.Predictor = b.opts.FeaturePredictor(name, featureValueDomains(fields), featureHelpDescriptions(fields))
	}

	// A NoCLI synthesized flag is engine-owned and never CLI-routable, so it
	// must not reserve its derived name against a user CLI flag that happens to
	// share it. A second Features field is still rejected by the FeatureFlag
	// duplicate-container guard in processFeaturesField. CLI-exposed
	// synthesized flags (flag:"name") keep full collision tracking.
	if !noCLI {
		if err := b.flagTracker.CheckAndAdd(flagSpec, structField.Name); err != nil {
			return err
		}
	}

	flagSpec.KeyCanonicalizer = func(key string) string {
		if feature := b.cmd.FindFeature(casing.Kebab(key)); feature != nil {
			return feature.Name
		}

		return key
	}

	b.cmd.Flags = append(b.cmd.Flags, flagSpec)
	b.cmd.FeatureFlag = flagSpec

	return nil
}

// validateFeatureHelpVariables reports an unavailable {{env}} reference on a
// Features container with a feature-specific message: the container
// substitutes {{env}} only for its combined-map env tag, so a container whose
// sole environment surface is a per-feature envprefix or field env tags cannot
// inline it. Other unavailable variables fall back to the shared message. Both
// the runtime schema build and conga-lint report each message verbatim.
func validateFeatureHelpVariables(helpText string, declared helpVars) error {
	_, referenced, err := interpolateHelp(helpText, helpVarValues{})
	if err != nil {
		return err
	}

	if referenced.Env && !declared.Env {
		return fmt.Errorf(`%w: tag "help" references {{env}}, but the Features container declares no combined-map env tag; per-feature variables are rendered in the Features section`, ErrInvalidStructTag)
	}

	return validateHelpVariables(helpText, declared)
}

// enforceFeatureStageAllOrNothing reports the first feature field (in
// declaration order) that omits the stage tag while a sibling declares it: a
// feature set is either fully staged or entirely stage-less, so the help
// screen's Features section never renders a mix of hinted and unhinted rows.
func enforceFeatureStageAllOrNothing(fields []*FeatureField) error {
	if !slices.ContainsFunc(fields, func(field *FeatureField) bool { return field.Stage != "" }) {
		return nil
	}

	for _, field := range fields {
		if field.Stage == "" {
			return FeatureStageAllOrNothingError(field.Name)
		}
	}

	return nil
}

// featureIntegerParseError classifies a failed feature-integer parse: range
// failures name the storage width, unsigned syntax failures name the
// non-negativity requirement, and everything else is a plain integer error.
func featureIntegerParseError(err error, signed bool) error {
	if errors.Is(err, strconv.ErrRange) {
		return errFeatureIntegerOutOfRange
	}

	if !signed {
		return errFeatureMustBeNonNegativeInteger
	}

	return errFeatureMustBeInteger
}

// invalidFeatureEnumError builds the enum-domain rejection listing the
// permitted levels.
func invalidFeatureEnumError(f *FeatureField) error {
	return fmt.Errorf("%w: %s", errFeatureInvalidEnum, strings.Join(f.Domain(), ", "))
}

// validateFeatureEnum parses and records the non-zero levels of an integer
// feature's enum tag. Level 0 is rejected: it is the reserved off level and is
// always accepted, so listing it would duplicate it in the rendered domain.
func validateFeatureEnum(field *FeatureField, enumTag string) error {
	parts := slicex.SplitAndTrim(enumTag, ",")
	if len(parts) == 0 {
		return fmt.Errorf("%w: tag \"enum\" cannot be empty", ErrInvalidStructTag)
	}

	for _, part := range parts {
		canonical, err := field.parseInteger(part)
		if err != nil {
			return fmt.Errorf("%w: feature enum value %q: %w", ErrInvalidStructTag, part, err)
		}

		if canonical == "0" {
			return fmt.Errorf("%w: feature %q enum value \"0\" is the reserved off level; list only non-zero levels", ErrInvalidStructTag, field.Name)
		}

		if !slices.Contains(field.Enum, canonical) {
			field.Enum = append(field.Enum, canonical)
		}
	}

	return nil
}

func parseFeatureTerm(term string) (featureTerm, bool) {
	term = strings.TrimSpace(term)
	if term == "" {
		return featureTerm{}, false
	}

	var result featureTerm

	if withoutBang, ok := strings.CutPrefix(term, "!"); ok {
		result.negated = true
		term = strings.TrimSpace(withoutBang)
	}

	if strings.ContainsAny(term, "!=") {
		return featureTerm{}, false
	}

	result.name = casing.Kebab(term)
	if result.name == "" {
		return featureTerm{}, false
	}

	return result, true
}

func matchSingleFeature(expr string, activeFeatures map[string]bool) bool {
	term, ok := parseFeatureTerm(expr)
	if !ok {
		return false
	}

	enabled := activeFeatures[term.name]

	if term.negated {
		return !enabled
	}

	return enabled
}

// featureValueDomains maps each declared feature to its completion value
// domain: true/false for bools and every permitted level for integers.
func featureValueDomains(fields []*FeatureField) map[string][]string {
	domains := make(map[string][]string, len(fields))

	for _, field := range fields {
		domains[field.Name] = field.Domain()
	}

	return domains
}

func featureStructType(target any) (reflect.Type, error) {
	innerValue := reflect.ValueOf(target)
	if !innerValue.IsValid() || innerValue.Kind() != reflect.Pointer || innerValue.Elem().Kind() != reflect.Struct {
		return nil, FeaturesMustBeStructParameterError()
	}

	return innerValue.Elem().Type(), nil
}

// featureHelpDescriptions maps each declared feature name to its help tag
// text, consumed as the description of the feature's completion key candidate.
func featureHelpDescriptions(fields []*FeatureField) map[string]string {
	descriptions := make(map[string]string, len(fields))

	for _, field := range fields {
		descriptions[field.Name] = field.Help
	}

	return descriptions
}

func validateFeatureReferences(root *Command) error {
	declared := make(map[string]bool, len(root.FeatureFields))
	for _, feature := range root.FeatureFields {
		declared[feature.Name] = true
	}

	if err := validateFeatureReferencesInCommand(root, declared); err != nil {
		return err
	}

	return validateFeatureEnvUniqueness(root)
}

// validateFeatureEnvUniqueness rejects environment variable collisions
// involving feature bindings: two feature fields resolving to the same
// variable, or a feature variable that a flag in the command tree also
// binds. It runs through the ValidateFeatureReferences hook, so the walk
// stays unreferenced — and dead-code eliminated — for applications that
// never enable features. Flag-to-flag collisions are a separate, pre-existing
// gap and are deliberately not reported here.
func validateFeatureEnvUniqueness(root *Command) error {
	if root.FeatureFlag == nil {
		return nil
	}

	owners := make(map[string]string)

	addOwner := func(envVar string, owner string) error {
		if previous, ok := owners[envVar]; ok {
			return FeatureEnvCollisionError(envVar, previous, owner)
		}

		owners[envVar] = owner

		return nil
	}

	for _, envVar := range root.FeatureFlag.EnvVars {
		if err := addOwner(envVar, "the feature map flag --"+root.FeatureFlag.Name); err != nil {
			return wrapFeatureEnvCollision(root, err)
		}
	}

	for _, feature := range root.FeatureFields {
		for _, envVar := range feature.EnvVars {
			if err := addOwner(envVar, "feature "+strconv.Quote(feature.Name)); err != nil {
				return wrapFeatureEnvCollision(root, err)
			}
		}
	}

	if len(owners) == 0 {
		return nil
	}

	return wrapFeatureEnvCollision(root, checkFlagEnvCollisions(root, owners))
}

// wrapFeatureEnvCollision adds the root struct and Features field context to a
// collision error, matching the analyzer's reportFieldError prefix and the
// schema-error contract for every other field-scoped diagnostic.
func wrapFeatureEnvCollision(root *Command, err error) error {
	if err == nil || root.featureFieldName == "" {
		return err
	}

	return fmt.Errorf("struct %s field %s: %w", root.featureStructName, root.featureFieldName, err)
}

// checkFlagEnvCollisions walks the command tree comparing every flag's
// environment bindings against the feature-owned variables recorded above.
// Flag variables are never added to the owner map, so flag-to-flag collisions
// stay unreported. The feature map flag itself is skipped: in combined-map
// mode its variables are the feature surface already recorded above.
func checkFlagEnvCollisions(cmd *Command, owners map[string]string) error {
	for _, flagSpec := range cmd.Flags {
		if flagSpec == cmd.FeatureFlag {
			continue
		}

		for _, envVar := range flagSpec.EnvVars {
			if previous, ok := owners[envVar]; ok {
				return FeatureEnvCollisionError(envVar, previous, "flag --"+flagSpec.Name)
			}
		}
	}

	for _, subcommand := range cmd.Subcommands {
		if err := checkFlagEnvCollisions(subcommand, owners); err != nil {
			return err
		}
	}

	return nil
}

func validateFeatureReferencesInCommand(cmd *Command, declared map[string]bool) error {
	if cmd.Feature != "" {
		if err := validateFeatureExpressionReferences("command "+strconv.Quote(cmd.Name), cmd.Feature, declared); err != nil {
			return err
		}
	}

	for _, flagSpec := range cmd.Flags {
		if flagSpec.Feature == "" {
			continue
		}

		if err := validateFeatureExpressionReferences("flag "+strconv.Quote("--"+flagSpec.Name), flagSpec.Feature, declared); err != nil {
			return err
		}
	}

	for _, subcommand := range cmd.Subcommands {
		if err := validateFeatureReferencesInCommand(subcommand, declared); err != nil {
			return err
		}
	}

	return nil
}

func validateFeatureExpressionReferences(owner, featureExpr string, declared map[string]bool) error {
	for rawPart := range strings.SplitSeq(featureExpr, ",") {
		term, _ := parseFeatureTerm(rawPart)
		if !declared[term.name] {
			return fmt.Errorf("%w: %s references undeclared feature %q; declare it in the root conga.Features container", ErrSchema, owner, term.name)
		}
	}

	return nil
}
