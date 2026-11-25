package spec

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/levenshtein"
)

// ValidationOptions carries the schema-build options that affect struct tag
// validation. A static analyzer reconstructs them from the app builder chain
// (app.AllowTags) instead of runtime reflection.
type ValidationOptions struct {
	// AllowedTags lists additional struct tag keys accepted by tag
	// validation (app.AllowTags equivalent).
	AllowedTags []string

	// SkipUnknownTagKeyCheck suppresses unknown struct tag key checks. The
	// static analyzer sets it when app.AllowTags evidence is ambiguous, so it
	// may only miss diagnostics, never invent them; the runtime schema builder
	// never sets it.
	SkipUnknownTagKeyCheck bool

	// EnvPrefix is the effective environment variable prefix of the command
	// declaring the field, used to decide whether {{env}} is available in a help
	// tag. It is only consulted when EnvPrefixKnown is true.
	EnvPrefix string

	// EnvPrefixKnown reports whether EnvPrefix was resolved. An unresolved
	// prefix leaves {{env}} available, so evidence the analyzer cannot see never
	// produces a diagnostic the runtime would not report.
	EnvPrefixKnown bool
}

// ValueTarget carries the facts about a flag or positional argument value type
// that ValidateValueTarget checks the field's tags against. The runtime schema
// builder derives them through reflection, the static analyzer through
// go/types, so both report the same diagnostics.
type ValueTarget struct {
	// IsMap reports a map collection (a map type that does not parse itself).
	IsMap bool

	// MapKeyIsString reports a map key with a string underlying kind. Only
	// consulted when IsMap is true.
	MapKeyIsString bool

	// MapKeyType is the display name of the map key type, used in the
	// non-string key message. Only consulted when IsMap is true.
	MapKeyType string

	// PathSubject is the display name of a path type (conga.File, conga.Dir,
	// conga.FileContent, or a FileFilterable, DirFilterable or PathFilterable
	// type), or of a slice or pointer element of one; empty otherwise.
	PathSubject string
}

// ValidateFieldTags validates the struct tags of a single schema field:
// tag syntax, disallowed tags, {{...}} help-variable references for flags,
// positional arguments, config-file flags, Features containers, and
// trailing-argument collectors, and tag-key candidate membership for the
// field's role.
func ValidateFieldTags(tag reflect.StructTag, role string, candidates []string, opts ValidationOptions) error {
	keys, err := parseStructTagKeys(tag)
	if err != nil {
		return err
	}

	if err := validateReservedPseudoTagKeys(keys); err != nil {
		return err
	}

	if err := validateDisallowedTags(tag, keys, role); err != nil {
		return err
	}

	if role == "flag" {
		if err := validateFlagGroupTagValues(tag); err != nil {
			return err
		}
	}

	switch role {
	case "flag", "subcommand", "help command", "version command", "completion command", "config file flag":
		if err := validateSectionTagValue(tag); err != nil {
			return err
		}
	}

	if err := validateRoleTagRules(tag, role, opts); err != nil {
		return err
	}

	if !opts.SkipUnknownTagKeyCheck {
		if err := validateTagCandidates(keys, role, candidates, opts.AllowedTags); err != nil {
			return err
		}
	}

	return nil
}

// TagCandidatesForRole returns the allowed struct tag key candidates for a
// schema field role. The second result reports whether the role is known.
func TagCandidatesForRole(role string) ([]string, bool) {
	switch role {
	case "flag":
		return flagTagCandidates, true
	case "positional argument":
		return argTagCandidates, true
	case "subcommand":
		return userCommandTagCandidates, true
	case "help command", "version command", "completion command":
		return builtinCommandTagCandidates, true
	case "config file flag":
		return configFileTagCandidates, true
	case "features":
		return featuresFieldTagCandidates, true
	case "feature":
		return featureFieldTagCandidates, true
	case "dash args", "raw args":
		return dashRawTagCandidates, true
	case "blank field":
		// Known with no candidates: blank fields accept no CONGA tags, and the
		// analyzer only runs ValidateFieldTags (and its role rules) for known roles.
		return nil, true
	default:
		return nil, false
	}
}

// InferPredictorFromType returns the predictor spec inferred from a domain
// type implementing Predictable, TagAwarePredictable, FileFilterable,
// DirFilterable, or PathFilterable. It is assigned to Options.InferPredictor
// by callers that enable the completion subsystem; referencing it from an
// always-linked path would keep the inference machinery (including its
// filesystem probes) in every binary.
func InferPredictorFromType(targetType reflect.Type, tags FieldTags) *PredictorSpec {
	if targetType == nil {
		return nil
	}

	elemType := targetType
	if isSliceCollection(elemType) {
		elemType = elemType.Elem()
	}

	if spec := inferFilterablePredictor(elemType); spec != nil {
		return spec
	}

	return inferPredictable(elemType, tags)
}

// ValidateConfigFileConfigTag rejects a config struct tag on a config file
// flag whose value is not "-".
func ValidateConfigFileConfigTag(tag reflect.StructTag) error {
	if configTag, ok := tag.Lookup("config"); ok && configTag != "-" {
		return schemaErrorf(`tag "config" on config file flag must be "-", got %q`, configTag)
	}

	return nil
}

// ValidateCounterDefaultTag rejects a default struct tag on a counter flag.
func ValidateCounterDefaultTag(tag reflect.StructTag) error {
	if _, ok := tag.Lookup("default"); ok {
		return schemaError(`tag "default" is not supported on counter flags`).
			withNote("counters count command-line occurrences starting at 0").
			withHint("seed a base level with an env or config-file binding, or use OptionalFlag[int] for a numeric default")
	}

	return nil
}

// FeatureFieldTypeError builds the schema error for a feature field whose type
// is neither a bool nor an integer. The runtime schema builder reports it from
// the reflected field kind and the static analyzer reports it from the
// go/types kind, so the message stays a single source of truth. The caller
// names the field as the error's owner.
func FeatureFieldTypeError(typeName string) error {
	return schemaErrorf("feature fields must be a bool or an integer, got %s", typeName)
}

// FeaturesMustBeOnRootError builds the schema error for a conga.Features
// container declared on a non-root command struct. The runtime schema builder
// returns it and the static analyzer reports it verbatim, keeping the message a
// single source of truth.
func FeaturesMustBeOnRootError() error {
	return schemaError("conga.Features must be declared on the root command struct")
}

// FeaturesMustBeStructParameterError builds the schema error for a
// conga.Features container whose type parameter is not a struct. The runtime
// schema builder and the static analyzer share it as the single source of
// truth.
func FeaturesMustBeStructParameterError() error {
	return schemaError("conga.Features must be parameterized by a struct type")
}

// FeaturesAlreadyDeclaredError builds the duplicate-container error for a
// second conga.Features field on the root command struct. The runtime schema
// builder and the static analyzer share it as the single source of truth;
// firstFlagName is the resolved synthesized flag name of the first container.
func FeaturesAlreadyDeclaredError(firstFlagName string) error {
	return schemaError("duplicate Features container").withNotef("already declared by %s", firstFlagName)
}

// FeatureEnvModeConflictError builds the schema error for a conga.Features
// container declaring both env and envprefix tags: env names the combined
// map variable while envprefix derives per-feature variables, and the two
// surfaces would compete over the same keys. The runtime schema builder and
// the static analyzer share it as the single source of truth.
func FeatureEnvModeConflictError() error {
	return schemaError(`tag "env" and tag "envprefix" cannot both be declared on a conga.Features container`).
		withNote("env names the combined map variable; envprefix derives one variable per feature")
}

// FeatureFieldEnvWithContainerEnvError builds the schema error for a feature
// field declaring env while the Features container names a combined map
// variable: the two surfaces would write the same keys with no defined
// precedence between them. The runtime schema builder and the static
// analyzer share it as the single source of truth.
func FeatureFieldEnvWithContainerEnvError(fieldName string) error {
	return schemaErrorf("feature field %s declares env while the Features container declares env", fieldName).
		withNote("the container's env tag names the combined map variable").
		withHint("declare one env surface per container")
}

// ConfigEnvSide names the config side of a config env collision: the root
// ConfigFile flag when one is declared (configFileFlagName), and the config
// file variable a registered decoder reads otherwise.
func ConfigEnvSide(configFileFlagName string) string {
	if configFileFlagName == "" {
		return "the config file variable"
	}

	return "flag --" + configFileFlagName
}

// ConfigEnvFlagCollisionError builds the schema error for a flag bound to the
// environment variable the configuration engine reads a config file path
// from. configSide comes from ConfigEnvSide. The runtime schema builder and
// the static analyzer share it as the single source of truth.
func ConfigEnvFlagCollisionError(envVar string, configSide string, flagName string) error {
	return configEnvCollisionError(envVar, configSide, "flag --"+flagName, `give the flag another env tag or exclude it with env:"-"`)
}

// ConfigEnvFeatureCollisionError builds the schema error for a feature field
// bound to the config file variable. configSide comes from ConfigEnvSide.
func ConfigEnvFeatureCollisionError(envVar string, configSide string, featureName string) error {
	return configEnvCollisionError(envVar, configSide, "feature "+strconv.Quote(featureName), `give the feature another env tag or exclude it with env:"-"`)
}

// ConfigEnvFeatureMapCollisionError builds the schema error for the Features
// container's combined map variable bound to the config file variable. It
// names the feature map flag (flagName) when the container has one on the
// command line, and the container when noCLI is set.
func ConfigEnvFeatureMapCollisionError(envVar string, configSide string, flagName string, noCLI bool) error {
	owner := "the feature map flag --" + flagName
	if noCLI {
		owner = "the Features container"
	}

	return configEnvCollisionError(envVar, configSide, owner, "give the Features container another env tag")
}

// FeatureEnvCollisionError builds the schema error for one environment
// variable bound by two schema members: two feature fields resolving to the
// same variable, or a feature variable a flag also binds. The runtime schema
// builder and the static analyzer share it as the single source of truth.
func FeatureEnvCollisionError(envVar string, first string, second string) error {
	return schemaErrorf("environment variable %q is bound by both %s and %s", envVar, first, second)
}

// FeatureOnRequiredFlagError builds the schema error for a feature tag on a
// required flag: a feature-gated flag must be optional. The runtime schema
// builder and the static analyzer share it as the single source of truth.
func FeatureOnRequiredFlagError() error {
	return schemaError(`tag "feature" cannot be used on required flags`).withNote("feature-gated flags must be optional")
}

// DeprecatedOnRequiredFlagError builds the schema error for a deprecated tag
// on a required flag: callers cannot migrate away from a flag they are still
// forced to supply. The runtime schema builder and the static analyzer share
// it as the single source of truth.
func DeprecatedOnRequiredFlagError() error {
	return schemaError(`tag "deprecated" cannot be used on required flags`).
		withHint("make the flag optional before deprecating it so callers can migrate without error")
}

// GroupTagOnRequiredFlagError builds the schema error for an xor or oneof
// group tag declared on a required flag. Required flags are individually
// mandatory, so group tags belong on optional flags; oneof is the tag that
// expresses exactly-one semantics. The runtime schema builder and the static
// analyzer share it as the single source of truth.
func GroupTagOnRequiredFlagError(tag string) error {
	return schemaErrorf("tag %q cannot be used on required flags", tag).
		withNote("required flags are individually mandatory").
		withHint("declare xor/oneof groups on optional flags, and use oneof when exactly one must be supplied")
}

// DefaultOnRequiredFlagError builds the schema error for a default tag on a
// required flag: a default would satisfy the requirement without any caller
// input, making the requirement meaningless. The runtime schema builder and
// the static analyzer share it as the single source of truth.
func DefaultOnRequiredFlagError() error {
	return schemaError(`tag "default" cannot be used on required flags`).
		withNote("the requirement must be satisfied by CLI, environment, or configuration input")
}

// DefaultOnOneOfFlagError builds the schema error for a default tag on a
// oneof member: the default marks the member as set in Get/IsSet and in --why
// without satisfying the Changed()-based exactly-one group, so the flag would
// look chosen while validation still demands a choice. The runtime schema
// builder and the static analyzer share it as the single source of truth.
func DefaultOnOneOfFlagError() error {
	return schemaError(`tag "default" cannot be used on oneof flags`).
		withNote("a default sets the flag's value and marks it as set in --why without satisfying the exactly-one group").
		withHint("remove the default or the oneof tag")
}

// SingleMemberGroupError builds the schema error for a group with only one
// member. A one-member xor or and group is a no-op; a one-member oneof group on
// an optional flag duplicates RequiredFlag. A one-member oneof on a Counter is
// legal and never reaches this constructor. The runtime schema builder and the
// static analyzer share it as the single source of truth.
func SingleMemberGroupError(kind, group, flagName, suggestion string) error {
	var note, hint string

	switch kind {
	case "oneof":
		note = "exactly-one groups need at least two members"
		hint = "use RequiredFlag for a single mandatory flag"
	case "xor", "and":
		note = fmt.Sprintf("a one-member %s group has no effect", kind)
		hint = "remove the tag or add another member"
	default:
		note = "groups need at least two members"
	}

	if suggestion != "" {
		if hint != "" {
			hint += "; "
		}

		hint += levenshtein.QuotedHint(suggestion)
	}

	return schemaErrorf("%s group %q has only one member (--%s)", kind, group, flagName).withNote(note).withHint(hint)
}

// OneOfAndOverlapError builds the schema error for two members of the same
// oneof group that also share an and group: selecting either member forces the
// other, so the exactly-one rule can never be satisfied. The runtime schema
// builder and the static analyzer share it as the single source of truth.
func OneOfAndOverlapError(oneOfGroup, andGroup, firstFlag, secondFlag string) error {
	return schemaErrorf(`flags --%s and --%s belong to oneof group %q and share "and" group %q`, firstFlag, secondFlag, oneOfGroup, andGroup).
		withNote("selecting either flag forces the other, so the exactly-one group can never be satisfied").
		withHint("remove one of the group tags")
}

// XorAndOverlapError builds the schema error for two members of the same xor
// group that also share an and group: supplying either member forces the other,
// so the mutually-exclusive rule can never be satisfied. The runtime schema
// builder and the static analyzer share it as the single source of truth.
func XorAndOverlapError(xorGroup, andGroup, firstFlag, secondFlag string) error {
	return schemaErrorf(`flags --%s and --%s belong to xor group %q and share "and" group %q`, firstFlag, secondFlag, xorGroup, andGroup).
		withNote("supplying either flag forces the other, so the mutually-exclusive group can never be satisfied").
		withHint("remove one of the group tags")
}

// BuiltinShortConflictError builds the schema error for a user-declared short
// flag rune that claims the rune assigned to a built-in flag the command
// offers: the binder would keep the user flag while the help screen still
// advertises the rune for the built-in. The runtime schema builder and the
// static analyzer share it as the single source of truth.
func BuiltinShortConflictError(builtin string, short rune, flagName string) error {
	return schemaErrorf("built-in --%s short flag -%c collides with --%s", builtin, short, flagName).
		withHint("assign a different rune to one of them")
}

// BuiltinShortPairError builds the schema error for app.HelpShortFlag and
// app.VersionShortFlag claiming the same rune while both built-ins are offered:
// the binder registers --help first, so the --version rune would be dropped
// while both flag rows still advertise it. The runtime schema builder and the
// static analyzer share it as the single source of truth.
func BuiltinShortPairError(short rune) error {
	return schemaErrorf("built-in --help and --version both claim short flag -%c", short).withHint("assign a different rune to one of them")
}

// ConfigExplicitConflictError builds the error when ConfigExplicit is combined with ConfigPaths or ConfigCascading.
// Both runtime schema validation and the static analyzer report it verbatim,
// keeping the lint-time and runtime messages a single source of truth.
func ConfigExplicitConflictError() error {
	return schemaError("conga.ConfigExplicit() names a single configuration file and cannot be combined with conga.ConfigPaths() or conga.ConfigCascading()")
}

// CascadingRequiresMultiError builds the cascading⟹multi arity-guard error
// for a root config-file flag: a flag without multi:"true" cannot report
// every merged file under Cascading(). Both runtime schema validation and the
// static analyzer report it verbatim, keeping the lint-time and runtime
// messages a single source of truth.
func CascadingRequiresMultiError(flagName string) error {
	return schemaErrorf(`the root config-file flag %q must declare multi:"true"`, flagName).
		withNote("conga.ConfigCascading() merges multiple config files").
		withHint(`add multi:"true" or drop conga.ConfigCascading() from app.ConfigOptions(...)`)
}

// CompletionRequiresEnableError builds the completion⟹enable schema guard: a
// command struct that declares a CompletionCommand field while completion is
// disabled would have the field silently dropped, so schema build rejects it
// and names the missing opt-in. Both runtime schema validation and the static
// analyzer report it verbatim, keeping the lint-time and runtime messages a
// single source of truth.
func CompletionRequiresEnableError() error {
	return schemaError("completion command declared without completion enabled").
		withHint("call app.EnableCompletion() when constructing the app, or remove the field")
}

// BootstrapRequiresBootstrapperError builds the schema error for a
// bootstrap-tagged field under a command hierarchy that never implements
// conga.Bootstrapper. The runtime schema builder and the static analyzer share
// it as the single source of truth.
func BootstrapRequiresBootstrapperError() error {
	return schemaError(`tag "bootstrap" requires the command or an ancestor command to implement conga.Bootstrapper`).
		withHint("implement Bootstrap(ctx context.Context, b *conga.Bootstrap) error or remove the tag")
}

// BootstrapFeatureConflictError builds the schema error for a bootstrap field
// that is also feature-gated. The bootstrap phase runs before feature gates
// resolve, so a gated bootstrap field cannot be evaluated reliably.
func BootstrapFeatureConflictError() error {
	return schemaError(`tag "bootstrap" cannot be combined with tag "feature"`).withNote("the bootstrap phase runs before feature gates are resolved")
}

// BootstrapFieldTypeError builds the schema error for a bootstrap field whose
// type is not a pure scalar value. Restricting bootstrap fields to scalar,
// slice, and map shapes keeps the second coercion cheap and free of I/O.
func BootstrapFieldTypeError() error {
	return schemaError("bootstrap fields must be scalars, slices of scalars, or maps of scalars").withNote("structs and pointers are not supported")
}

// BootstrapReadsWhileParsingError builds the schema error for a bootstrap
// field typed conga.FileContent (or a slice or map of it). Bootstrap fields are
// coerced in two passes, and FileContent reads its file, or stdin, each time.
func BootstrapReadsWhileParsingError() error {
	return schemaError(`tag "bootstrap" is not supported on conga.FileContent`).
		withNote("bootstrap fields are parsed twice and FileContent reads its file while parsing").
		withHint("use conga.File and read it in Bootstrap")
}

// BootstrapTagged reports whether the bootstrap tag is on: bare presence or
// "true". It reads a value ValidateFieldTags has already checked for the
// field's role, so a misspelled value has been reported before it reads as
// off. conga-lint's bootstrap check uses it; schema build parses the tag with
// ParseStrictBoolTag in parseBootstrapTag.
func BootstrapTagged(tag reflect.StructTag) bool {
	return strictBoolTag(tag, "bootstrap")
}

// FeaturesRequiresEnableError returns the schema error for a conga.Features
// container declared under an app that never calls app.EnableFeatures(). The
// runtime schema builder and the static analyzer share it as the single source
// of truth.
func FeaturesRequiresEnableError() error {
	return schemaError("conga.Features declared on root command requires app.EnableFeatures()")
}

// FeatureTagRequiresEnableError returns the schema error for a feature:"..."
// struct tag declared under an app that never calls app.EnableFeatures(). The
// runtime schema builder and the static analyzer share it as the single source
// of truth.
func FeatureTagRequiresEnableError() error {
	return schemaError("feature struct tag requires app.EnableFeatures()")
}

// ValidatePredictorTag validates an explicit predictor:"..." struct tag
// without runtime predictor inference: built-in predictor vocabulary is always
// checked; custom predictor names must be registered unless checkCustomNames is
// false. The static analyzer passes false when app.Predictor evidence is
// ambiguous, so it may only miss diagnostics, never invent them; the runtime
// schema builder always checks custom names.
func ValidatePredictorTag(tag reflect.StructTag, isBool bool, registeredNames []string, checkCustomNames bool) error {
	tagValue, hasPred := tag.Lookup("predictor")
	if !hasPred {
		return nil
	}

	_, err := validateExplicitPredictor(strings.TrimSpace(tagValue), isBool, registeredNames, checkCustomNames)

	return err
}

// ValidateNonContainerFieldTags rejects CONGA tags on fields that schema
// building ignores. Plain struct fields are not flattened, so a tag such as
// name or help on one would silently do nothing; surfacing it as a schema error
// keeps the declaration honest. fieldTagKeys carries the domain-type tags the
// field's value type registers, which are equally ignored on a plain field.
// Tag syntax is always validated; well-formed foreign keys are ignored, and
// keys listed in AllowedTags are accepted. The reserved ~-prefixed namespace and the required
// tag stay rejected regardless of AllowedTags, mirroring container roles. The
// static analyzer resolves only the built-in CONGA domain types, so
// user-defined TagSpecProviders stay runtime-only, and it skips the key
// rejection when it cannot prove the allow-list (SkipUnknownTagKeyCheck), so it
// never invents a diagnostic. Unexported fields never reach this check, so
// embedded behavior mixins remain supported.
func ValidateNonContainerFieldTags(tag reflect.StructTag, fieldTagKeys []string, opts ValidationOptions) error {
	keys, err := parseStructTagKeys(tag)
	if err != nil {
		return err
	}

	if err := validateReservedPseudoTagKeys(keys); err != nil {
		return err
	}

	if err := validateDisallowedTags(tag, keys, "plain field"); err != nil {
		return err
	}

	for _, key := range keys {
		if !isAnyCongaTag(key) && !slices.Contains(fieldTagKeys, key) {
			continue
		}

		if opts.SkipUnknownTagKeyCheck || slices.Contains(opts.AllowedTags, key) {
			continue
		}

		return schemaErrorf("tag %q is not valid on a plain field", key).
			withHint("declare a flag, argument, command, config-file, features, or trailing-args container")
	}

	return nil
}

// ValidateValueTarget checks the tags of a flag or positional argument against
// its value type: maps need string keys and cannot declare enum, range or len,
// and path fields (a path type, or a path predictor tag) cannot declare enum.
// It returns the first violation.
func ValidateValueTarget(tag reflect.StructTag, target ValueTarget) error {
	if err := validateMapConstraints(tag, target); err != nil {
		return err
	}

	return validateEnumTarget(tag, target.PathSubject)
}

// validateRoleTagRules applies the role-specific tag rules shared by schema
// build and the static analyzer: constraint tag values for flags and
// positional arguments, {{...}} help-variable availability for the roles that
// substitute them, and the enumhint opt-out. The rules run regardless of
// strict tag validation, mirroring the runtime schema build.
func validateRoleTagRules(tag reflect.StructTag, role string, opts ValidationOptions) error {
	switch role {
	case "flag", "positional argument":
		if err := validateConstraintTagValues(tag); err != nil {
			return err
		}

		fallthrough
	case "config file flag", "features":
		declared := helpVarAvailability(tag, role, opts.EnvPrefix, opts.EnvPrefixKnown)

		validator := validateHelpVariables
		if role == "features" {
			validator = validateFeatureHelpVariables
		}

		if err := validator(tag.Get("help"), declared); err != nil {
			return err
		}
	case "dash args", "raw args":
		// Trailing collectors declare no range, enum, default, or env, so
		// every help variable is unavailable.
		if err := validateHelpVariables(tag.Get("help"), helpVars{}); err != nil {
			return err
		}
	}

	if err := validateStrictBoolTags(tag, role); err != nil {
		return err
	}

	switch role {
	case "flag", "positional argument", "features":
		if err := validateEnumHintTag(tag, role); err != nil {
			return err
		}
	case "blank field":
		if _, ok := tag.Lookup("envprefix"); ok {
			return blankFieldEnvPrefixError()
		}
	}

	return nil
}

// blankFieldEnvPrefixError reports an envprefix tag on a blank (_) field. The
// blank-field prefix form was removed; the check runs regardless of the
// unknown-tag-key setting so legacy declarations never silently lose their
// prefix.
func blankFieldEnvPrefixError() error {
	return schemaError(`tag "envprefix" is not supported on blank (_) fields`).
		withHint("set the root prefix with app.EnvPrefix(...) and a subcommand prefix with the envprefix tag on its Command[T] field")
}

// validateEnumHintTag validates the enumhint struct tag of a flag, positional
// argument, or features container. The tag opts the field out of the automatic
// [enum: ...] help hint; its only accepted value is "-", the field must
// declare enum values to suppress (the features container always carries the
// declared feature names), and the help tag must not reference {{enum}} —
// inlining the values and hiding them are contradictory requests. The runtime
// schema build and the static analyzer share it as the single source of truth.
func validateEnumHintTag(tag reflect.StructTag, role string) error {
	raw, ok := tag.Lookup("enumhint")
	if !ok {
		return nil
	}

	if strings.TrimSpace(raw) != "-" {
		return schemaError(`tag "enumhint" must be "-"`)
	}

	if role != "features" && len(SplitTagList(tag, "enum")) == 0 {
		return schemaError(`tag "enumhint" suppresses the enum hint, but the field declares no enum values`)
	}

	_, referenced, err := interpolateHelp(tag.Get("help"), helpVarValues{})
	if err != nil {
		return err
	}

	if referenced.Enum {
		return schemaError(`tag "enumhint" is redundant: the help tag already references {{enum}}`)
	}

	return nil
}

// validateReservedPseudoTagKeys rejects the engine-owned "~" pseudo-tag
// namespace in source-declared struct tags (~explicit, ~searchpaths,
// ~cascading). The namespace is a pure reservation: the app-level
// configuration settings travel as a typed ConfigSettings struct on the root
// command, never as struct tags, but a source-declared "~"-prefixed tag
// could collide with the reserved vocabulary, so it is rejected
// unconditionally — app.AllowTags never widens the reserved namespace.
func validateReservedPseudoTagKeys(keys []string) error {
	for _, key := range keys {
		if strings.HasPrefix(key, "~") {
			return schemaErrorf("struct tag %q is reserved for CONGA's app-level configuration settings", key).
				withHint("declare it via app.ConfigOptions(...) instead")
		}
	}

	return nil
}

// validateConstraintTagValues rejects a constraint tag that is present but
// carries no value. An empty range, len, items, or enum tag is a no-op the
// engine would otherwise silently ignore, so a help variable referencing it
// would expand to an empty string; rejecting the tag keeps the schema honest.
func validateConstraintTagValues(tag reflect.StructTag) error {
	for _, key := range []string{"range", "len", "items"} {
		if value, ok := tag.Lookup(key); ok && strings.TrimSpace(value) == "" {
			return schemaErrorf("tag %q cannot be empty", key)
		}
	}

	if _, ok := tag.Lookup("enum"); ok && len(SplitTagList(tag, "enum")) == 0 {
		return schemaError(`tag "enum" cannot be empty`)
	}

	return nil
}

// validateSectionTagValue rejects a section title padded with whitespace: the
// padded and unpadded spellings would render as separate help sections. An
// empty section value means "no section" and stays valid.
func validateSectionTagValue(tag reflect.StructTag) error {
	value, ok := tag.Lookup("section")
	if !ok {
		return nil
	}

	if strings.TrimSpace(value) != value {
		return schemaError(`tag "section" cannot have leading or trailing whitespace`)
	}

	return nil
}

// validateFlagGroupTagValues rejects group tags that are present but carry no
// usable group name, or that would silently form a different group because of
// surrounding whitespace. The runtime schema builder and the static analyzer
// share the check through ValidateFieldTags.
func validateFlagGroupTagValues(tag reflect.StructTag) error {
	for _, key := range []string{"xor", "and", "oneof"} {
		value, ok := tag.Lookup(key)
		if !ok {
			continue
		}

		trimmed := strings.TrimSpace(value)

		if trimmed == "" {
			return schemaErrorf("tag %q cannot be empty", key)
		}

		if trimmed != value {
			return schemaErrorf("tag %q cannot have leading or trailing whitespace", key)
		}
	}

	return nil
}

// configEnvCollisionError formats a config env collision: first names the
// config side, second the colliding binding, and remedy says how to separate
// them.
func configEnvCollisionError(envVar string, first string, second string, remedy string) error {
	return schemaErrorf("environment variable %q is bound by both %s and %s", envVar, first, second).withHint(remedy)
}

// validateMapConstraints rejects map-typed flag/arg fields whose key type is
// not a string, or whose tags carry per-entry constraints (enum/range/len)
// that maps cannot honor; map entries are constrained via "items" instead.
func validateMapConstraints(tag reflect.StructTag, target ValueTarget) error {
	if !target.IsMap {
		return nil
	}

	if !target.MapKeyIsString {
		return schemaErrorf("map keys must be of type string, got %s", target.MapKeyType)
	}

	for _, key := range []string{"enum", "range", "len"} {
		if hasTag(tag, key) {
			return schemaErrorf("tag %q is not supported on map types", key).
				withHint(`use tag "items" to constrain the number of map entries, or custom domain types with SelfValidator to validate map entries`)
		}
	}

	return nil
}

// validateEnumTarget rejects an enum tag on a path field: a path type named by
// pathSubject, or a field whose predictor tag names a path predictor. Path
// values compare as expanded paths (or file content), so an enum list of names
// cannot describe them; glob narrows a conga.File's completion, and a string
// field carries a fixed set of names. Predictor syntax errors are left to
// ValidatePredictorTag.
func validateEnumTarget(tag reflect.StructTag, pathSubject string) error {
	if !hasTag(tag, "enum") {
		return nil
	}

	if pathSubject != "" {
		return enumOnPathTypeError(tag, pathSubject)
	}

	if name := pathPredictorName(tag); name != "" {
		return enumWithPathPredictorError(name)
	}

	return nil
}

// pathPredictorName returns the canonical name of the path predictor a
// predictor tag names (file, dir, symlink, exec), or "" for any other or
// malformed value.
func pathPredictorName(tag reflect.StructTag) string {
	value := strings.TrimSpace(tag.Get("predictor"))

	predictorSpec, ok, err := parseBuiltinFilesystemPredictor(value)
	if !ok {
		predictorSpec, ok, err = parseBuiltinEnginePredictor(value)
	}

	if !ok || err != nil || predictorSpec == nil || !isPathPredictorKind(predictorSpec.Kind) {
		return ""
	}

	return predictorSpec.Name
}

// enumOnPathTypeError builds the enum rejection for a path type. Only a
// conga.File without executable or symlink accepts glob, so only that field
// gets the glob hint.
func enumOnPathTypeError(tag reflect.StructTag, pathSubject string) error {
	const stringFieldHint = `a string field with tag "enum" to accept a fixed set of names`

	_, hasExecutable := tag.Lookup("executable")
	_, hasSymlink := tag.Lookup("symlink")

	if pathSubject == "conga.File" && !hasExecutable && !hasSymlink {
		return schemaErrorf(`tag "enum" is not supported on %s`, pathSubject).
			withHintf(`use tag "glob" to filter completion candidates, or %s`, stringFieldHint)
	}

	return schemaErrorf(`tag "enum" is not supported on %s`, pathSubject).withHint("use " + stringFieldHint)
}

// enumWithPathPredictorError builds the enum rejection for a path predictor
// tag, named canonically.
func enumWithPathPredictorError(predictorName string) error {
	err := schemaErrorf(`tag "enum" cannot be combined with predictor:%q`, predictorName).
		withHint(`remove tag "enum", or remove the predictor tag to complete the enum values`)
	if predictorName == "file" {
		return err.withNote(`predictor:"file:<patterns>" filters completion candidates`)
	}

	return err
}
