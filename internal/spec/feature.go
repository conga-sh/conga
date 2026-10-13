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
	"github.com/conga-sh/conga/internal/slicex"
)

// FeatureField describes one field of the root conga.Features[T] struct: a
// single feature toggle.
type FeatureField struct {
	// Name is the normalized kebab-case feature name (the map key).
	Name string
	// Help is the human-readable description shown in help and completion.
	Help string
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
	// FieldIndex is the index of the feature field inside the inner feature struct.
	FieldIndex int
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

// ValidateFeatureFieldTags validates and records the enum and default tags of
// one feature field: enum is valid only on integer features and may not be
// empty, every enum value must canonicalize within the feature's domain and
// storage width, an integer feature requires at least one enum level, and a
// declared default must canonicalize against the enum domain. The runtime
// schema builder reports it from the reflected field and the static analyzer
// reconstructs the same metadata from go/types, keeping the diagnostics a
// single source of truth.
func ValidateFeatureFieldTags(field *FeatureField, tag reflect.StructTag) error {
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

// parseFeatureTag extracts, validates, and normalizes the "feature" struct tag.
func parseFeatureTag(tag reflect.StructTag) (string, error) {
	feature := strings.TrimSpace(tag.Get("feature"))
	if feature == "" {
		return "", nil
	}

	if err := ValidateFeatureExpression(feature); err != nil {
		return "", err
	}

	return NormalizeFeatureExpression(feature), nil
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
