package spec

import (
	"fmt"
	"reflect"
	"strings"
)

// ValidationOptions carries the schema-build options that affect struct tag
// validation. A static analyzer reconstructs them from the app builder chain
// (app.StrictTagValidation, app.AllowTags) instead of runtime reflection.
type ValidationOptions struct {
	// StrictTagValidation enables unknown struct tag key checks.
	StrictTagValidation bool

	// AllowUnknownTags disables unknown struct tag key checks entirely
	// (conga:"allow-unknown-tags" directive equivalent).
	AllowUnknownTags bool

	// AllowedTags lists additional struct tag keys accepted under strict
	// tag validation (app.AllowTags equivalent).
	AllowedTags []string

	// EnvPrefix is the effective environment variable prefix of the command
	// declaring the field, used to decide whether {{env}} is available in a help
	// tag. It is only consulted when EnvPrefixKnown is true.
	EnvPrefix string

	// EnvPrefixKnown reports whether EnvPrefix was resolved. An unresolved
	// prefix leaves {{env}} available, so evidence the analyzer cannot see never
	// produces a diagnostic the runtime would not report.
	EnvPrefixKnown bool
}

// ValidateFieldTags validates the struct tags of a single schema field:
// tag syntax, conga directives, disallowed tags, {{...}} help-variable
// references for the flag, positional-argument, and config-file-flag roles,
// and — under strict tag validation — tag-key candidate membership for the
// field's role. It returns the parsed conga directives so callers can honor
// conga:"ignore" and allow-unknown-tags propagation.
func ValidateFieldTags(tag reflect.StructTag, role string, candidates []string, opts ValidationOptions) (Directives, error) {
	keys, err := parseStructTagKeys(tag)
	if err != nil {
		return Directives{}, err
	}

	if err := validateReservedPseudoTagKeys(keys); err != nil {
		return Directives{}, err
	}

	dirs, err := ParseCongaDirectives(tag.Get("conga"))
	if err != nil {
		return Directives{}, err
	}

	if dirs.Ignore {
		return dirs, nil
	}

	if err := validateDisallowedTags(tag, keys, role); err != nil {
		return Directives{}, err
	}

	if err := validateRoleTagRules(tag, role, opts); err != nil {
		return Directives{}, err
	}

	if !opts.StrictTagValidation || opts.AllowUnknownTags || dirs.AllowUnknownTags {
		return dirs, nil
	}

	if err := validateTagCandidates(keys, role, candidates, opts.AllowedTags); err != nil {
		return Directives{}, err
	}

	return dirs, nil
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
	case "sentinel field":
		return sentinelTagCandidates, true
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
	if elemType.Kind() == reflect.Slice {
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
		return fmt.Errorf("%w: tag \"config\" on config file flag must be \"-\", got %q", ErrInvalidStructTag, configTag)
	}

	return nil
}

// ValidateCounterDefaultTag rejects a default struct tag on a counter flag.
func ValidateCounterDefaultTag(tag reflect.StructTag) error {
	if _, ok := tag.Lookup("default"); ok {
		return fmt.Errorf("%w: tag \"default\" is not supported on counter flags; counters count command-line occurrences starting at 0. Seed a base level with an env or config-file binding, or use OptionalFlag[int] for a numeric default", ErrInvalidStructTag)
	}

	return nil
}

// FeatureFieldTypeError builds the schema error for a feature field whose type
// is neither a bool nor an integer. The runtime schema builder reports it from
// the reflected field kind and the static analyzer reports it from the
// go/types kind, so the message stays a single source of truth.
func FeatureFieldTypeError(fieldName string, typeName string) error {
	return fmt.Errorf("%w: feature field %s must be a bool or an integer, got %s", ErrInvalidStructTag, fieldName, typeName)
}

// FeaturesMustBeOnRootError builds the schema error for a conga.Features
// container declared on a non-root command struct. The runtime schema builder
// returns it and the static analyzer reports it verbatim, keeping the message a
// single source of truth.
func FeaturesMustBeOnRootError() error {
	return fmt.Errorf("%w: conga.Features must be declared on the root command struct", ErrSchema)
}

// FeaturesMustBeStructParameterError builds the schema error for a
// conga.Features container whose type parameter is not a struct. The runtime
// schema builder and the static analyzer share it as the single source of
// truth.
func FeaturesMustBeStructParameterError() error {
	return fmt.Errorf("%w: conga.Features must be parameterized by a struct type", ErrSchema)
}

// FeaturesAlreadyDeclaredError builds the duplicate-container error for a
// second conga.Features field on the root command struct. The runtime schema
// builder and the static analyzer share it as the single source of truth;
// firstFlagName is the resolved synthesized flag name of the first container.
func FeaturesAlreadyDeclaredError(firstFlagName string) error {
	return fmt.Errorf("%w: Features (already declared by %s)", ErrDuplicateFlag, firstFlagName)
}

// FeatureEnvModeConflictError builds the schema error for a conga.Features
// container declaring both env and envprefix tags: env names the combined
// map variable while envprefix derives per-feature variables, and the two
// surfaces would compete over the same keys. The runtime schema builder and
// the static analyzer share it as the single source of truth.
func FeatureEnvModeConflictError() error {
	return fmt.Errorf("%w: tag \"env\" and tag \"envprefix\" cannot both be declared on a conga.Features container (env names the combined map variable; envprefix derives one variable per feature)", ErrSchema)
}

// FeatureFieldEnvWithContainerEnvError builds the schema error for a feature
// field declaring env while the Features container names a combined map
// variable: the two surfaces would write the same keys with no defined
// precedence between them. The runtime schema builder and the static
// analyzer share it as the single source of truth.
func FeatureFieldEnvWithContainerEnvError(fieldName string) error {
	return fmt.Errorf("%w: feature field %s declares env while the Features container declares env (the combined map variable); declare one env surface per container", ErrSchema, fieldName)
}

// FeatureEnvCollisionError builds the schema error for one environment
// variable bound by two schema members: two feature fields resolving to the
// same variable, or a feature variable a flag also binds. The runtime schema
// builder and the static analyzer share it as the single source of truth.
func FeatureEnvCollisionError(envVar string, first string, second string) error {
	return fmt.Errorf("%w: environment variable %q is bound by both %s and %s", ErrSchema, envVar, first, second)
}

// FeatureOnRequiredFlagError builds the schema error for a feature tag on a
// required flag: a feature-gated flag must be optional. The runtime schema
// builder and the static analyzer share it as the single source of truth.
func FeatureOnRequiredFlagError() error {
	return fmt.Errorf("%w: tag \"feature\" cannot be used on required flags (feature-gated flags must be optional)", ErrInvalidStructTag)
}

// ConfigExplicitConflictError builds the error when ConfigExplicit is combined with ConfigPaths or ConfigCascading.
// The error wraps ErrSchema so runtime schema validation returns it directly and the static analyzer reports it verbatim,
// keeping the lint-time and runtime messages a single source of truth.
func ConfigExplicitConflictError() error {
	return fmt.Errorf("%w: conga.ConfigExplicit() specifies an absolute configuration file and cannot be combined with conga.ConfigPaths() or conga.ConfigCascading()", ErrSchema)
}

// CascadingRequiresMultiError builds the cascading⟹multi arity-guard error
// for a root config-file flag: a flag without multi:"true" cannot report
// every merged file under Cascading(). The error wraps ErrSchema so runtime
// schema validation returns it directly and the static analyzer reports it
// verbatim, keeping the lint-time and runtime messages a single source of
// truth.
func CascadingRequiresMultiError(flagName string) error {
	return fmt.Errorf("%w: conga.ConfigCascading() merges multiple config files; the root config-file flag %q must declare multi:\"true\" (or drop conga.ConfigCascading() from app.ConfigOptions(...))", ErrSchema, flagName)
}

// ConfigLoaderOptionsConflictError builds the error when conga.ConfigLoader()
// is combined with a file-based configuration option (conga.ConfigPaths,
// conga.ConfigExplicit, or conga.ConfigCascading). The loader replaces file
// discovery entirely, so the options would be silently ignored. The error
// wraps ErrSchema so runtime schema validation returns it directly and the
// static analyzer reports it verbatim, keeping the lint-time and runtime
// messages a single source of truth.
func ConfigLoaderOptionsConflictError() error {
	return fmt.Errorf("%w: conga.ConfigLoader() supplies the configuration document programmatically and cannot be combined with conga.ConfigPaths(), conga.ConfigExplicit(), or conga.ConfigCascading()", ErrSchema)
}

// ConfigLoaderDecoderConflictError builds the error when conga.ConfigLoader()
// is combined with a registered configuration decoder. With a loader there is
// no file loading, so decoders would be dead weight. The error wraps ErrSchema
// so runtime schema validation returns it directly and the static analyzer
// reports it verbatim, keeping the lint-time and runtime messages a single
// source of truth.
func ConfigLoaderDecoderConflictError() error {
	return fmt.Errorf("%w: conga.ConfigLoader() replaces file loading and cannot be combined with app.RegisterDecoder (remove the decoder registrations or the loader)", ErrSchema)
}

// ConfigLoaderConfigFileConflictError builds the error when conga.ConfigLoader()
// is combined with a root conga.ConfigFile field. The loader replaces file
// loading, so the config-file flag would never load a file. The error wraps
// ErrSchema so runtime schema validation returns it directly and the static
// analyzer reports it verbatim, keeping the lint-time and runtime messages a
// single source of truth.
func ConfigLoaderConfigFileConflictError(flagName string) error {
	return fmt.Errorf("%w: the root command declares config-file flag %q, but conga.ConfigLoader() replaces file loading and never loads a file (remove the conga.ConfigFile field or the loader)", ErrSchema, flagName)
}

// CompletionRequiresEnableError builds the completion⟹enable schema guard: a
// command struct that declares a CompletionCommand field while completion is
// disabled would have the field silently dropped, so schema build rejects it
// and names the missing opt-in. The error wraps ErrSchema so runtime schema
// validation returns it directly and the static analyzer reports it verbatim,
// keeping the lint-time and runtime messages a single source of truth.
func CompletionRequiresEnableError() error {
	return fmt.Errorf("%w: completion command declared without completion enabled; call app.EnableCompletion() when constructing the app, or remove the field", ErrSchema)
}

// BootstrapRequiresLoaderError builds the schema error for a bootstrap-tagged
// field under an app that never declares a configuration loader. The runtime
// schema builder and the static analyzer share it as the single source of
// truth.
func BootstrapRequiresLoaderError() error {
	return fmt.Errorf("%w: tag \"bootstrap\" requires a configuration loader; declare app.ConfigOptions(conga.ConfigLoader(...)) or remove the tag", ErrSchema)
}

// BootstrapFeatureConflictError builds the schema error for a bootstrap field
// that is also feature-gated. The loader runs before feature gates resolve, so
// a gated bootstrap field cannot be evaluated reliably.
func BootstrapFeatureConflictError() error {
	return fmt.Errorf("%w: tag \"bootstrap\" cannot be combined with tag \"feature\" (the loader runs before feature gates are resolved)", ErrInvalidStructTag)
}

// BootstrapFieldTypeError builds the schema error for a bootstrap field whose
// type is not a pure scalar value. Restricting bootstrap fields to scalar,
// slice, and map shapes keeps the second coercion cheap and free of I/O.
func BootstrapFieldTypeError() error {
	return fmt.Errorf("%w: bootstrap fields must be scalars, slices of scalars, or maps of scalars (structs and pointers are not supported)", ErrInvalidStructTag)
}

// BootstrapTagged reports whether the bootstrap tag is declared with an
// accepted presence-or-true value. The runtime schema builder and the static
// analyzer share it as the single source of truth.
func BootstrapTagged(tag reflect.StructTag) bool {
	return parsePresenceOrTrueTag(tag, "bootstrap")
}

// FeaturesRequiresEnableError returns the schema error for a conga.Features
// container declared under an app that never calls app.EnableFeatures(). The
// runtime schema builder and the static analyzer share it as the single source
// of truth.
func FeaturesRequiresEnableError() error {
	return errFeaturesRequiresEnableFeatures
}

// FeatureTagRequiresEnableError returns the schema error for a feature:"..."
// struct tag declared under an app that never calls app.EnableFeatures(). The
// runtime schema builder and the static analyzer share it as the single source
// of truth.
func FeatureTagRequiresEnableError() error {
	return errFeatureTagRequiresEnableFeatures
}

// ValidatePredictorTag validates an explicit predictor:"..." struct tag
// without runtime predictor inference: built-in predictor vocabulary is
// always checked; custom predictor names only under strict tag validation.
func ValidatePredictorTag(tag reflect.StructTag, isBool bool, opts ValidationOptions, registeredNames []string) error {
	tagValue, hasPred := tag.Lookup("predictor")
	if !hasPred {
		return nil
	}

	_, err := validateExplicitPredictor(strings.TrimSpace(tagValue), isBool, opts.StrictTagValidation, registeredNames)

	return err
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
		if err := validateHelpVariables(tag.Get("help"), declared); err != nil {
			return err
		}
	}

	switch role {
	case "flag", "positional argument", "features":
		if err := validateEnumHintTag(tag, role); err != nil {
			return err
		}
	}

	return nil
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
		return fmt.Errorf("%w: tag \"enumhint\" must be \"-\"", ErrInvalidStructTag)
	}

	if role != "features" && len(SplitTagList(tag, "enum")) == 0 {
		return fmt.Errorf("%w: tag \"enumhint\" suppresses the enum hint, but the field declares no enum values", ErrInvalidStructTag)
	}

	_, referenced, err := interpolateHelp(tag.Get("help"), helpVarValues{})
	if err != nil {
		return err
	}

	if referenced.Enum {
		return fmt.Errorf("%w: tag \"enumhint\" is redundant: the help tag already references {{enum}}", ErrInvalidStructTag)
	}

	return nil
}

// validateReservedPseudoTagKeys rejects the engine-owned "~" pseudo-tag
// namespace in source-declared struct tags (~explicit, ~searchpaths,
// ~cascading). The namespace is a pure reservation: the app-level
// configuration settings travel as a typed ConfigSettings struct on the root
// command, never as struct tags, but a source-declared "~"-prefixed tag
// could collide with the reserved vocabulary, so it is rejected
// unconditionally — allow-unknown-tags escape hatches never widen the
// reserved namespace.
func validateReservedPseudoTagKeys(keys []string) error {
	for _, key := range keys {
		if strings.HasPrefix(key, "~") {
			return fmt.Errorf("%w: struct tag %q is reserved for CONGA's app-level configuration settings; declare it via app.ConfigOptions(...) instead", ErrInvalidStructTag, key)
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
			return fmt.Errorf("%w: tag %q cannot be empty", ErrInvalidStructTag, key)
		}
	}

	if _, ok := tag.Lookup("enum"); ok && len(SplitTagList(tag, "enum")) == 0 {
		return fmt.Errorf("%w: tag %q cannot be empty", ErrInvalidStructTag, "enum")
	}

	return nil
}
