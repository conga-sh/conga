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
}

// ValidateFieldTags validates the struct tags of a single schema field:
// tag syntax, conga directives, disallowed tags, and — under strict tag
// validation — tag-key candidate membership for the field's role. It returns
// the parsed conga directives so callers can honor conga:"ignore" and
// allow-unknown-tags propagation.
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

// CascadingRequiresMultiError builds the cascading⟹multi arity-guard error
// for a root config-file flag: a flag without multi:"true" cannot report
// every merged file under Cascading(). The error wraps ErrSchema so runtime
// schema validation returns it directly and the static analyzer reports it
// verbatim, keeping the lint-time and runtime messages a single source of
// truth.
func CascadingRequiresMultiError(flagName string) error {
	return fmt.Errorf("%w: Cascading() merges multiple configuration files; the root config-file flag %q must declare multi:\"true\" (or drop Cascading() from app.ConfigOptions(...))", ErrSchema, flagName)
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
