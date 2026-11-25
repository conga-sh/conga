package spec

import (
	"cmp"
	"encoding"
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/errlines"
	"github.com/conga-sh/conga/internal/intx"
	"github.com/conga-sh/conga/internal/slicex"
)

const (
	// RangeSigned compares bounds and values as int64: Bytes() int64
	// projections and signed integer underlying kinds.
	RangeSigned RangeDomain = iota

	// RangeUnsigned compares bounds and values as uint64: unsigned integer
	// underlying kinds.
	RangeUnsigned

	// RangeFloat compares bounds and values as float64: float underlying
	// kinds.
	RangeFloat
)

// RangeDomain selects the native comparison domain of a text-grammar range
// type. Bytes() int64 projections and signed integer kinds compare as int64,
// unsigned integer kinds as uint64, and float kinds as float64.
type RangeDomain uint8

// ParseRangeSpec parses a range tag value (min..max, min.., ..max, or exact).
func ParseRangeSpec(tag string) (*Range, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, schemaError("range tag cannot be empty")
	}

	if strings.Contains(tag, "..") {
		parts := strings.Split(tag, "..")
		if len(parts) != 2 || strings.Contains(tag, "...") {
			return nil, schemaErrorf("invalid range syntax %q", tag).withNote("expected min..max, min.., or ..max")
		}

		minStr := strings.TrimSpace(parts[0])
		maxStr := strings.TrimSpace(parts[1])

		if minStr == "" && maxStr == "" {
			return nil, schemaErrorf("invalid range syntax %q", tag).withNote("at least one bound required")
		}

		spec := &Range{
			MinStr: minStr,
			MaxStr: maxStr,
			HasMin: minStr != "",
			HasMax: maxStr != "",
		}

		return spec, nil
	}

	return &Range{
		MinStr: tag,
		MaxStr: tag,
		HasMin: true,
		HasMax: true,
		Exact:  true,
	}, nil
}

// ValidateTextBounds checks each declared bound side with validate, which
// rejects a bound the field's tags don't accept (a decimal byte size under
// units:"binary"). The schema error keeps validate's error as its cause and
// takes its note and hint, falling back to its text as the note when it has
// none. Schema build and the static analyzer share it, so both report the same
// diagnostic.
func ValidateTextBounds(rangeSpec *Range, boundKind string, validate func(string) error) error {
	if rangeSpec.HasMin {
		if err := validate(rangeSpec.MinStr); err != nil {
			return boundError(boundKind, "min", rangeSpec.MinStr, err)
		}
	}

	if rangeSpec.HasMax {
		if err := validate(rangeSpec.MaxStr); err != nil {
			return boundError(boundKind, "max", rangeSpec.MaxStr, err)
		}
	}

	return nil
}

// FloatBoundParser returns the range-bound parser for a float of bitSize bits
// (32 or 64). A float32 bound parses at float32 precision and widens to
// float64 the same way a float32 value does, so a value equal to its bound
// compares equal; a bound outside the float32 range fails to parse. Schema
// build, the binder, and conga-lint share it.
func FloatBoundParser(bitSize int) func(string) (float64, error) {
	return func(raw string) (float64, error) {
		return strconv.ParseFloat(raw, bitSize)
	}
}

// ValidateScalarRange parses the min/max bounds of rangeSpec with parse and
// checks their order. boundKind names the value kind in parse-error messages
// ("duration", "integer", "unsigned integer", "float", "byte size"). A bound
// that fails to parse gets the parse error as its cause and no note of its
// own, so a text dialect whose errors may lack a note checks its bounds with
// [ValidateTextBounds] first.
func ValidateScalarRange[T cmp.Ordered](rangeSpec *Range, boundKind string, parse func(string) (T, error)) error {
	return validateRange(rangeSpec, boundKind, parse, scalarBoundError)
}

// ParseCountBound parses a len or items bound: a non-negative integer in the
// integer syntax that fits an int.
func ParseCountBound(raw string) (int, error) {
	value, err := intx.ParseUint(raw, strconv.IntSize-1)
	if err != nil {
		return 0, err
	}

	return int(value), nil //nolint:gosec // parsed with bitSize strconv.IntSize-1, so the value fits int
}

// ValidateCountRangeSpec validates len/items range bounds as non-negative integers.
func ValidateCountRangeSpec(spec *Range, tagName string) error {
	if spec == nil {
		return nil
	}

	var minValue, maxValue int

	if spec.HasMin {
		v, err := ParseCountBound(spec.MinStr)
		if err != nil {
			return countBoundError(tagName, "min", spec.MinStr, err)
		}

		minValue = v
	}

	if spec.HasMax {
		v, err := ParseCountBound(spec.MaxStr)
		if err != nil {
			return countBoundError(tagName, "max", spec.MaxStr, err)
		}

		maxValue = v
	}

	if spec.HasMin && spec.HasMax && minValue > maxValue {
		return schemaErrorf("%s min bound %q cannot be greater than max bound %q", tagName, spec.MinStr, spec.MaxStr)
	}

	return nil
}

// ParseStrictBoolTag parses a strict boolean struct tag: absence is reported
// via the present flag (value false), "" (bare presence) and "true" map to
// true, "false" maps to false, and any other value is a schema error.
// Security-adjacent tags must not fail silently, so a typo (e.g. safe:"ture")
// surfaces at schema build instead of silently inverting the tag's intent.
func ParseStrictBoolTag(tag reflect.StructTag, key string) (bool, bool, error) {
	raw, ok := tag.Lookup(key)
	if !ok {
		return false, false, nil
	}

	switch raw {
	case "", "true":
		return true, true, nil
	case "false":
		return false, true, nil
	default:
		return false, true, schemaErrorf(`tag %q expects "true" or "false", got %q`, key, raw)
	}
}

// ParseGlobPatterns splits a comma-separated glob tag value into trimmed
// patterns and validates each one: empty elements, path separators, and
// malformed filepath.Match syntax are rejected at schema build. Patterns
// match file names (not paths) during completion traversal, so a
// wildcard-free pattern such as "Makefile" matches that literal name.
// Errors return a *SchemaError.
func ParseGlobPatterns(value string) ([]string, error) {
	elements := strings.Split(value, ",")
	for _, element := range elements {
		if strings.TrimSpace(element) == "" {
			return nil, schemaError("empty glob pattern element").withNote(`expected comma-separated patterns like "Makefile,Dockerfile" or "*.yaml"`)
		}
	}

	patterns := slicex.Map(elements, strings.TrimSpace)

	for _, pattern := range patterns {
		if strings.ContainsAny(pattern, `/\`) {
			return nil, schemaErrorf("glob pattern %q must not contain path separators", pattern).
				withNote("patterns match file names during completion traversal")
		}

		if _, err := filepath.Match(pattern, ""); err != nil {
			return nil, schemaErrorf("glob pattern %q is malformed: %s", pattern, err).withCause(err)
		}
	}

	return patterns, nil
}

// SplitTagList splits a comma-separated struct tag value into trimmed entries.
func SplitTagList(tag reflect.StructTag, key string) []string {
	value := tag.Get(key)
	if value == "" {
		return nil
	}

	return slicex.SplitAndTrim(value, ",")
}

// ParseSummaryTag validates and returns the summary tag, falling back to
// defaultSummary when the tag is absent.
func ParseSummaryTag(tag reflect.StructTag, defaultSummary string) (string, error) {
	summaryTag, hasSummary := tag.Lookup("summary")
	if !hasSummary {
		return defaultSummary, nil
	}

	trimmed := strings.TrimSpace(summaryTag)
	if trimmed == "" {
		return "", schemaError(`tag "summary" cannot be empty`)
	}

	if strings.ContainsAny(summaryTag, "\r\n") {
		return "", schemaError(`tag "summary" must be a single line without newlines`)
	}

	return trimmed, nil
}

// TextRangeDomain reports whether elemType carries range bounds through its own
// encoding.TextUnmarshaler grammar, and which native domain compares it. Bound
// parsing reads every bound into a fresh instance via UnmarshalText, so the
// type's own grammar is the single source of truth; boundKind names the kind in
// parse-error messages ("byte size" for Bytes() int64 projections, "value" for
// numeric-underlying dialects). Pointer element types and types without an
// ordered projection are rejected.
func TextRangeDomain(elemType reflect.Type) (RangeDomain, string, bool) {
	if elemType == nil || elemType.Kind() == reflect.Pointer {
		return 0, "", false
	}

	pointerType := reflect.PointerTo(elemType)
	if !pointerType.Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
		return 0, "", false
	}

	if pointerType.Implements(reflect.TypeFor[interface{ Bytes() int64 }]()) {
		return RangeSigned, "byte size", true
	}

	switch elemType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return RangeSigned, "value", true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return RangeUnsigned, "value", true
	case reflect.Float32, reflect.Float64:
		return RangeFloat, "value", true
	default:
		return 0, "", false
	}
}

// TextRangeParser returns the bound parser for elemType in the caller's native
// domain. ok is false when elemType is not a text-range type or its domain does
// not match T.
func TextRangeParser[T cmp.Ordered](elemType reflect.Type) (func(string) (T, error), bool) {
	domain, _, ok := TextRangeDomain(elemType)
	if !ok || !domainMatches[T](domain) {
		return nil, false
	}

	var zero T

	parse := func(raw string) (T, error) {
		bound, err := parseTextBound(elemType, domain, raw)
		if err != nil {
			return zero, err
		}

		value, ok := domainValue[T](domain, bound)
		if !ok {
			return zero, schemaErrorf("range type %s does not project into its native domain", elemType)
		}

		return value, nil
	}

	return parse, true
}

// ProjectTextValue projects a coerced value of elemType into the caller's
// native domain. ok is false when elemType is not a text-range type, its domain
// does not match T, or the value cannot be projected.
func ProjectTextValue[T cmp.Ordered](elemType reflect.Type, value reflect.Value) (T, bool) {
	domain, _, ok := TextRangeDomain(elemType)
	if !ok || !domainMatches[T](domain) {
		var zero T

		return zero, false
	}

	pointer, ok := pointerToValue(elemType, value)
	if !ok {
		var zero T

		return zero, false
	}

	bound, ok := projectTextBound(pointer, domain)
	if !ok {
		var zero T

		return zero, false
	}

	return domainValue[T](domain, bound)
}

// ResolveFlagEnvVars resolves a flag field's environment variable names the
// way the schema builder does: an explicit env tag names the variables,
// env:"-" excludes them, and an untagged field derives one variable from
// envPrefix (<PREFIX>_<UPPER_SNAKE(flag name)>, with the trailing underscore
// normalized). The second result reports the exclusion. The static analyzer
// shares it as the single source of truth for derived variable names.
func ResolveFlagEnvVars(tag reflect.StructTag, flagName string, envPrefix string) ([]string, bool) {
	if envTag, ok := tag.Lookup("env"); ok {
		if strings.TrimSpace(envTag) == "-" {
			return nil, true
		}

		vars := SplitTagList(tag, "env")
		if len(vars) > 0 {
			return vars, false
		}
	}

	if envPrefix != "" {
		if !strings.HasSuffix(envPrefix, "_") {
			envPrefix += "_"
		}

		return []string{envPrefix + casing.UpperSnake(flagName)}, false
	}

	return nil, false
}

// FeatureFieldDeclaresEnv reports whether a feature field's env tag names at
// least one environment variable. A missing tag, env:"-", and an empty tag
// are absence, so they never conflict with a combined-map container.
func FeatureFieldDeclaresEnv(tag reflect.StructTag) bool {
	envTag, ok := tag.Lookup("env")
	if !ok || strings.TrimSpace(envTag) == "-" {
		return false
	}

	return len(SplitTagList(tag, "env")) > 0
}

// ParseFlagShort returns the rune declared by a flag's short tag, or 0 when the
// tag is absent. It is the shared decoding rule for the runtime schema builder
// and the static analyzer, so both see the same rune when deciding whether a
// user-declared short collides with a built-in one.
func ParseFlagShort(tag reflect.StructTag) rune {
	return extractFlagShort(tag)
}

// DefaultArgName returns the positional-argument display name used in usage
// syntax and diagnostics: the name tag when present, else the angle-bracketed
// kebab-case struct field name. The runtime schema builder and the static
// analyzer share it as the single source of truth.
func DefaultArgName(fieldName string, tag reflect.StructTag) string {
	return cond.SetOrDefaultFunc(tag.Get("name"), func() string {
		return "<" + casing.Kebab(fieldName) + ">"
	})
}

type flagKindInfo struct {
	targetType  reflect.Type
	isBool      bool
	isSlice     bool
	isMap       bool
	isCounter   bool
	isNegatable bool
	sep         string
}

// textBound carries a parsed text-grammar bound in its native domain; exactly
// one field is set according to the RangeDomain it was projected into.
type textBound struct {
	signed   int64
	unsigned uint64
	float    float64
}

// validateRange parses the min/max bounds of rangeSpec with parse, checks
// their order, and builds parse failures with buildError.
func validateRange[T cmp.Ordered](rangeSpec *Range, boundKind string, parse func(string) (T, error), buildError func(boundKind, side, raw string, err error) *SchemaError) error {
	var (
		minValue T
		maxValue T
	)

	if rangeSpec.HasMin {
		value, err := parse(rangeSpec.MinStr)
		if err != nil {
			return buildError(boundKind, "min", rangeSpec.MinStr, err)
		}

		minValue = value
	}

	if rangeSpec.HasMax {
		value, err := parse(rangeSpec.MaxStr)
		if err != nil {
			return buildError(boundKind, "max", rangeSpec.MaxStr, err)
		}

		maxValue = value
	}

	if rangeSpec.HasMin && rangeSpec.HasMax && minValue > maxValue {
		return schemaErrorf("min bound %q cannot be greater than max bound %q", rangeSpec.MinStr, rangeSpec.MaxStr)
	}

	return nil
}

// scalarBoundError reports a scalar range bound its parser rejected, with the
// parse error as its cause. A bound out of range gets the note "is out of
// range". Any other note and the hint are the cause's, such as the integer
// syntax's leading-zero correction.
func scalarBoundError(boundKind, side, raw string, err error) *SchemaError {
	boundErr := schemaErrorf("invalid %s %s bound %q", boundKind, side, raw).withCause(err)
	if errors.Is(err, strconv.ErrRange) {
		return boundErr.withNote("is out of range")
	}

	return boundErr
}

// boundError reports a range bound a text type's parser rejected, with the
// parser as its cause. The note is the parser's, or its text when it has none.
func boundError(boundKind, side, raw string, err error) *SchemaError {
	boundErr := schemaErrorf("invalid %s %s bound %q", boundKind, side, raw).withCause(err)
	if errlines.Note(err) == "" {
		return boundErr.withNote(err.Error())
	}

	return boundErr
}

// countBoundError reports an invalid len or items bound. A leading zero gets
// the integer syntax's correction. Any other failure says what a bound must be.
func countBoundError(tagName, side, raw string, err error) error {
	boundErr := schemaErrorf("invalid %s %s bound %q", tagName, side, raw).withCause(err)
	if errlines.Note(err) == "" {
		return boundErr.withNote("must be non-negative integer")
	}

	return boundErr
}

func parseStructTagKeys(tag reflect.StructTag) ([]string, error) {
	var keys []string

	tagStr := string(tag)

	for tagStr != "" {
		tagStr = strings.TrimLeft(tagStr, " ")
		if tagStr == "" {
			break
		}

		key, remaining, err := scanTagKey(tagStr, tag)
		if err != nil {
			return nil, err
		}

		_, remaining, err = scanTagValue(remaining, tag)
		if err != nil {
			return nil, err
		}

		keys = append(keys, key)
		tagStr = remaining
	}

	return keys, nil
}

func scanTagKey(tagStr string, fullTag reflect.StructTag) (string, string, error) {
	i := 0
	for i < len(tagStr) && tagStr[i] > ' ' && tagStr[i] != ':' && tagStr[i] != '"' && tagStr[i] != 0x7f {
		i++
	}

	if i == 0 || i >= len(tagStr) || tagStr[i] != ':' {
		return "", "", schemaErrorf("malformed struct tag %q", fullTag)
	}

	key := tagStr[:i]
	remaining := tagStr[i+1:]

	return key, remaining, nil
}

func scanTagValue(tagStr string, fullTag reflect.StructTag) (string, string, error) {
	if tagStr == "" || tagStr[0] != '"' {
		return "", "", schemaErrorf("malformed struct tag %q", fullTag)
	}

	tagStr = tagStr[1:]
	i := 0

	for i < len(tagStr) && tagStr[i] != '"' {
		if tagStr[i] == '\\' {
			i++
		}

		i++
	}

	if i >= len(tagStr) {
		return "", "", schemaErrorf("malformed struct tag %q", fullTag)
	}

	value := tagStr[:i]
	remaining := tagStr[i+1:]

	return value, remaining, nil
}

func extractFlagKindInfo(structField reflect.StructField, classification container.Classification) flagKindInfo {
	isCounter := classification.Kind == container.KindCounter

	innerType := extractGenericType(structField.Type)
	if isCounter {
		innerType = reflect.TypeFor[int]()
	}

	isMap := isMapCollection(innerType)
	isSlice := isSliceCollection(innerType)
	isBool := innerType.Kind() == reflect.Bool
	isNegatable := isBool && strictBoolTag(structField.Tag, "negatable")

	sep, sepFound := structField.Tag.Lookup("sep")
	if !sepFound {
		sep = ","
	}

	return flagKindInfo{
		targetType:  innerType,
		isBool:      isBool,
		isSlice:     isSlice,
		isMap:       isMap,
		isCounter:   isCounter,
		isNegatable: isNegatable,
		sep:         sep,
	}
}

func parseFlagSpec(structField reflect.StructField, flagHandle any, classification container.Classification, fieldTags FieldTags, opts *Options) (*Flag, error) {
	kindInfo := extractFlagKindInfo(structField, classification)

	if kindInfo.isCounter {
		if err := ValidateCounterDefaultTag(structField.Tag); err != nil {
			return nil, err
		}
	}

	name := extractFlagName(structField)
	shortRune := extractFlagShort(structField.Tag)

	envVars, envExcluded := ResolveFlagEnvVars(structField.Tag, name, opts.EnvPrefix)
	enumValues := SplitTagList(structField.Tag, "enum")
	aliases := SplitTagList(structField.Tag, "aliases")

	rangeSpec, lenSpec, itemsSpec, err := parseValidationSpecs(structField.Tag, kindInfo.targetType, fieldTags, opts)
	if err != nil {
		return nil, err
	}

	integerEnum, err := parseFlagIntegerEnum(enumValues, kindInfo, opts)
	if err != nil {
		return nil, err
	}

	helpText, referenced, err := interpolateHelp(
		structField.Tag.Get("help"),
		helpVarValuesFor(rangeSpec, lenSpec, itemsSpec, enumValues, structField.Tag.Get("default"), envVars),
	)
	if err != nil {
		return nil, err
	}

	placeholder := structField.Tag.Get("placeholder")

	safeTagValue, safePresent, err := ParseStrictBoolTag(structField.Tag, "safe")
	if err != nil {
		return nil, err
	}

	safe := safeTagValue
	if !safePresent {
		safe = kindInfo.isBool || kindInfo.isCounter
	}

	predSpec, err := parsePredictorSpec(structField.Tag, kindInfo.targetType, kindInfo.isBool, fieldTags, opts)
	if err != nil {
		return nil, err
	}

	feature, err := parseTagFeature(structField.Tag, classification.Required, opts)
	if err != nil {
		return nil, err
	}

	bootstrap, err := parseBootstrapField(structField.Tag, kindInfo.targetType, feature, opts)
	if err != nil {
		return nil, err
	}

	return &Flag{
		Name:        name,
		Short:       shortRune,
		Aliases:     aliases,
		Default:     structField.Tag.Get("default"),
		EnvVars:     envVars,
		EnvExcluded: envExcluded,
		ConfigKey:   structField.Tag.Get("config"),
		XorGroup:    structField.Tag.Get("xor"),
		AndGroup:    structField.Tag.Get("and"),
		OneOfGroup:  structField.Tag.Get("oneof"),
		Enum:        enumValues,
		IntegerEnum: integerEnum,
		Range:       rangeSpec,
		Len:         lenSpec,
		Items:       itemsSpec,
		Help:        helpText,
		HelpVars:    referenced,
		// The enumhint tag's "-" value was validated by ValidateFieldTags before
		// parsing; presence alone opts the flag out of the enum hint.
		EnumHintHidden: hasTag(structField.Tag, "enumhint"),
		Placeholder:    placeholder,
		Section:        structField.Tag.Get("section"),
		Hidden:         strictBoolTag(structField.Tag, "hidden"),
		Safe:           safe,
		Deprecated:     structField.Tag.Get("deprecated"),
		Feature:        feature,
		Bootstrap:      bootstrap,
		Sep:            kindInfo.sep,
		IsBool:         kindInfo.isBool,
		IsSlice:        kindInfo.isSlice,
		IsMap:          kindInfo.isMap,
		IsCounter:      kindInfo.isCounter,
		IsNegatable:    kindInfo.isNegatable,
		Required:       classification.Required,
		Predictor:      predSpec,
		TargetType:     kindInfo.targetType,
		Handle:         flagHandle,
	}, nil
}

// parseBootstrapField validates and reports the bootstrap tag for a flag or
// positional argument. Bootstrap fields must have a pure scalar shape and may
// only be declared under a command that implements Bootstrapper; feature-gated
// bootstrap inputs are rejected because the bootstrap pass runs before feature
// gates resolve.
func parseBootstrapField(tag reflect.StructTag, targetType reflect.Type, feature string, opts *Options) (bool, error) {
	bootstrap, err := parseBootstrapTag(tag, feature, opts)
	if err != nil || !bootstrap {
		return bootstrap, err
	}

	if opts.ReadsWhileParsing != nil && bootstrapReadsWhileParsing(targetType, opts.ReadsWhileParsing) {
		return false, BootstrapReadsWhileParsingError()
	}

	if !isBootstrapType(targetType) {
		return false, BootstrapFieldTypeError()
	}

	return true, nil
}

// bootstrapReadsWhileParsing reports whether targetType, or the element or
// key type of a slice or map, reads its input while parsing.
func bootstrapReadsWhileParsing(targetType reflect.Type, readsWhileParsing func(reflect.Type) bool) bool {
	if targetType == nil {
		return false
	}

	if readsWhileParsing(targetType) {
		return true
	}

	if !IsCollection(targetType) {
		return false
	}

	switch targetType.Kind() {
	case reflect.Slice:
		return readsWhileParsing(targetType.Elem())
	case reflect.Map:
		return readsWhileParsing(targetType.Key()) || readsWhileParsing(targetType.Elem())
	default:
		return false
	}
}

// parseBootstrapTag validates the bootstrap tag's Bootstrapper requirement and
// feature exclusion without checking the field's value shape.
func parseBootstrapTag(tag reflect.StructTag, feature string, opts *Options) (bool, error) {
	bootstrap, _, err := ParseStrictBoolTag(tag, "bootstrap")
	if err != nil || !bootstrap {
		return false, err
	}

	if opts == nil || !opts.HasBootstrapper {
		return false, BootstrapRequiresBootstrapperError()
	}

	if feature != "" {
		return false, BootstrapFeatureConflictError()
	}

	return true, nil
}

// isBootstrapType reports whether targetType is a scalar kind or a slice/map
// whose keys and elements are recursively scalar. Structs such as File and
// FileContent are rejected so the bootstrap phase cannot perform I/O that the
// normal binding pass would repeat.
func isBootstrapType(targetType reflect.Type) bool {
	return isBootstrapTypeVisiting(targetType, make(map[reflect.Type]bool))
}

// isBootstrapTypeVisiting walks container types while tracking the types on
// the current recursion path, rejecting self-referential slice and map shapes
// instead of recursing until the stack overflows.
func isBootstrapTypeVisiting(targetType reflect.Type, visiting map[reflect.Type]bool) bool {
	if targetType == nil {
		return false
	}

	switch targetType.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	case reflect.Slice, reflect.Map:
	default:
		return false
	}

	if visiting[targetType] {
		return false
	}

	visiting[targetType] = true
	defer delete(visiting, targetType)

	if targetType.Kind() == reflect.Slice {
		return isBootstrapTypeVisiting(targetType.Elem(), visiting)
	}

	return isBootstrapTypeVisiting(targetType.Key(), visiting) &&
		isBootstrapTypeVisiting(targetType.Elem(), visiting)
}

func parseTagFeature(tag reflect.StructTag, required bool, opts *Options) (string, error) {
	raw := tag.Get("feature")
	if raw == "" {
		return "", nil
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	if opts.ParseFeatureTag == nil {
		return "", FeatureTagRequiresEnableError()
	}

	return opts.ParseFeatureTag(raw, required)
}

func extractFlagName(structField reflect.StructField) string {
	name := structField.Tag.Get("name")
	if name != "" {
		return name
	}

	return casing.Kebab(structField.Name)
}

func extractFlagShort(structTag reflect.StructTag) rune {
	if shortTag := structTag.Get("short"); shortTag != "" {
		shortRune, _ := utf8.DecodeRuneInString(shortTag)

		return shortRune
	}

	return 0
}

func parseValidationSpecs(structTag reflect.StructTag, targetType reflect.Type, fieldTags FieldTags, opts *Options) (*Range, *Range, *Range, error) {
	var (
		rangeSpec *Range
		lenSpec   *Range
		itemsSpec *Range
	)

	if rangeTag := structTag.Get("range"); rangeTag != "" {
		spec, err := ParseRangeSpec(rangeTag)
		if err != nil {
			return nil, nil, nil, err
		}

		if err := validateRangeSpec(spec, targetType, fieldTags, opts); err != nil {
			return nil, nil, nil, err
		}

		rangeSpec = spec
	}

	if lenTag := structTag.Get("len"); lenTag != "" {
		spec, err := ParseRangeSpec(lenTag)
		if err != nil {
			return nil, nil, nil, err
		}

		if err := ValidateCountRangeSpec(spec, "len"); err != nil {
			return nil, nil, nil, err
		}

		lenSpec = spec
	}

	if itemsTag := structTag.Get("items"); itemsTag != "" {
		spec, err := ParseRangeSpec(itemsTag)
		if err != nil {
			return nil, nil, nil, err
		}

		if err := ValidateCountRangeSpec(spec, "items"); err != nil {
			return nil, nil, nil, err
		}

		itemsSpec = spec
	}

	return rangeSpec, lenSpec, itemsSpec, nil
}

// validateRangeSpec is the runtime counterpart of the analyzer's
// validateRangeBounds: numeric element kinds get their bounds parsed, the
// duration type gets duration bounds, text-grammar types get their own
// UnmarshalText grammar, and any other type is rejected. fieldTags and opts
// feed the optional tag-aware bound validator (see Options.ValidateTextRangeBound)
// so domain types can enforce their own tag constraints on range bounds.
func validateRangeSpec(rangeSpec *Range, targetType reflect.Type, fieldTags FieldTags, opts *Options) error {
	if rangeSpec == nil || targetType == nil {
		return nil
	}

	elemType := targetType
	if isSliceCollection(elemType) {
		elemType = elemType.Elem()
	}

	if elemType == reflect.TypeFor[time.Duration]() {
		return ValidateScalarRange(rangeSpec, "duration", time.ParseDuration)
	}

	if domain, boundKind, ok := TextRangeDomain(elemType); ok {
		if validator := boundValidatorOf(opts); validator != nil {
			if err := validateTaggedTextBounds(rangeSpec, boundKind, elemType, fieldTags, validator); err != nil {
				return err
			}
		}

		switch domain {
		case RangeSigned:
			parse, _ := TextRangeParser[int64](elemType)

			return validateRange(rangeSpec, boundKind, parse, boundError)
		case RangeUnsigned:
			parse, _ := TextRangeParser[uint64](elemType)

			return validateRange(rangeSpec, boundKind, parse, boundError)
		case RangeFloat:
			parse, _ := TextRangeParser[float64](elemType)

			return validateRange(rangeSpec, boundKind, parse, boundError)
		}
	}

	switch elemType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return ValidateScalarRange(rangeSpec, "integer", func(raw string) (int64, error) {
			return intx.ParseInt(raw, 64)
		})

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return ValidateScalarRange(rangeSpec, "unsigned integer", func(raw string) (uint64, error) {
			return intx.ParseUint(raw, 64)
		})

	case reflect.Float32, reflect.Float64:
		return ValidateScalarRange(rangeSpec, "float", FloatBoundParser(elemType.Bits()))

	default:
		return RangeUnsupportedTypeError(elemType.String())
	}
}

// validateTaggedTextBounds runs the caller-supplied tag-aware bound validator
// over the declared bound sides through [ValidateTextBounds].
func validateTaggedTextBounds(rangeSpec *Range, boundKind string, elemType reflect.Type, fieldTags FieldTags, validate func(reflect.Type, FieldTags, string) error) error {
	return ValidateTextBounds(rangeSpec, boundKind, func(raw string) error {
		return validate(elemType, fieldTags, raw)
	})
}

func boundValidatorOf(opts *Options) func(reflect.Type, FieldTags, string) error {
	if opts != nil {
		return opts.ValidateTextRangeBound
	}

	return nil
}

// parseTextBound reads raw through elemType's UnmarshalText into a fresh
// instance and projects the parsed value into its native domain. The
// TextUnmarshaler and projection checks re-verify the TextRangeDomain gate and
// guard against silent zero-value comparisons if the two ever drift.
func parseTextBound(elemType reflect.Type, domain RangeDomain, raw string) (textBound, error) {
	instance := reflect.New(elemType)

	unmarshaler, ok := reflect.TypeAssert[encoding.TextUnmarshaler](instance)
	if !ok {
		return textBound{}, schemaErrorf("range type %s does not implement encoding.TextUnmarshaler", elemType)
	}

	if err := unmarshaler.UnmarshalText([]byte(raw)); err != nil {
		return textBound{}, err
	}

	bound, ok := projectTextBound(instance, domain)
	if !ok {
		return textBound{}, schemaErrorf("range type %s has no ordered projection", elemType)
	}

	return bound, nil
}

// projectTextBound projects a *elemType instance into its native domain,
// preferring the Bytes() int64 projection over the numeric underlying kind.
func projectTextBound(pointer reflect.Value, domain RangeDomain) (textBound, bool) {
	if byteSize, ok := reflect.TypeAssert[interface{ Bytes() int64 }](pointer); ok {
		return textBound{signed: byteSize.Bytes()}, true
	}

	element := pointer.Elem()

	switch domain {
	case RangeSigned:
		if isSignedKind(element.Kind()) {
			return textBound{signed: element.Int()}, true
		}
	case RangeUnsigned:
		if isUnsignedKind(element.Kind()) {
			return textBound{unsigned: element.Uint()}, true
		}
	case RangeFloat:
		if element.Kind() == reflect.Float32 || element.Kind() == reflect.Float64 {
			return textBound{float: element.Float()}, true
		}
	}

	return textBound{}, false
}

// pointerToValue returns an addressable pointer to elemType holding value,
// accepting either a value or a pointer.
func pointerToValue(elemType reflect.Type, value reflect.Value) (reflect.Value, bool) {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() || value.Type().Elem() != elemType {
			return reflect.Value{}, false
		}

		return value, true
	}

	if value.Type() != elemType {
		return reflect.Value{}, false
	}

	pointer := reflect.New(elemType)
	pointer.Elem().Set(value)

	return pointer, true
}

// domainMatches reports whether T is the native Go type of domain.
func domainMatches[T cmp.Ordered](domain RangeDomain) bool {
	var zero T

	switch any(zero).(type) {
	case int64:
		return domain == RangeSigned
	case uint64:
		return domain == RangeUnsigned
	case float64:
		return domain == RangeFloat
	default:
		return false
	}
}

// domainValue converts a projected bound into the caller's native domain. ok is
// false when T does not match the domain, which callers rule out beforehand
// through domainMatches.
func domainValue[T cmp.Ordered](domain RangeDomain, bound textBound) (T, bool) {
	var zero T

	switch domain {
	case RangeSigned:
		value, ok := any(bound.signed).(T)

		return value, ok
	case RangeUnsigned:
		value, ok := any(bound.unsigned).(T)

		return value, ok
	case RangeFloat:
		value, ok := any(bound.float).(T)

		return value, ok
	default:
		return zero, false
	}
}

func isSignedKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}

func isUnsignedKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func parseArgSpec(structField reflect.StructField, argHandle any, classification container.Classification, fieldTags FieldTags, opts *Options) (*Arg, error) {
	name := DefaultArgName(structField.Name, structField.Tag)

	innerType := extractGenericType(structField.Type)
	isSlice := isSliceCollection(innerType)
	isMap := isMapCollection(innerType)

	sep, sepFound := structField.Tag.Lookup("sep")
	if !sepFound {
		sep = ","
	}

	enumValues := SplitTagList(structField.Tag, "enum")

	rangeSpec, lenSpec, itemsSpec, err := parseValidationSpecs(structField.Tag, innerType, fieldTags, opts)
	if err != nil {
		return nil, err
	}

	integerEnum, err := parseIntegerEnum(enumValues, innerType, opts)
	if err != nil {
		return nil, err
	}

	predSpec, err := parsePredictorSpec(structField.Tag, innerType, false, fieldTags, opts)
	if err != nil {
		return nil, err
	}

	helpText, referenced, err := interpolateHelp(
		structField.Tag.Get("help"),
		helpVarValuesFor(rangeSpec, lenSpec, itemsSpec, enumValues, structField.Tag.Get("default"), nil),
	)
	if err != nil {
		return nil, err
	}

	bootstrap, err := parseBootstrapField(structField.Tag, innerType, "", opts)
	if err != nil {
		return nil, err
	}

	return &Arg{
		Name:     name,
		Default:  structField.Tag.Get("default"),
		Help:     helpText,
		HelpVars: referenced,
		// The enumhint tag's "-" value was validated by ValidateFieldTags
		// before parsing; presence alone opts the argument out of the enum hint.
		EnumHintHidden: hasTag(structField.Tag, "enumhint"),
		Enum:           enumValues,
		IntegerEnum:    integerEnum,
		Range:          rangeSpec,
		Len:            lenSpec,
		Items:          itemsSpec,
		Sep:            sep,
		IsSlice:        isSlice,
		IsMap:          isMap,
		Bootstrap:      bootstrap,
		Required:       classification.Required,
		Predictor:      predSpec,
		TargetType:     innerType,
		Handle:         argHandle,
	}, nil
}

// parseFlagIntegerEnum parses a flag's integer enum values (see
// parseIntegerEnum). Counters bind no enum, so they parse none.
func parseFlagIntegerEnum(enumValues []string, kindInfo flagKindInfo, opts *Options) ([]string, error) {
	if kindInfo.isCounter {
		return nil, nil
	}

	return parseIntegerEnum(enumValues, kindInfo.targetType, opts)
}

// parseIntegerEnum returns the decimal form of each enum value when
// targetType, or a slice's element type, decodes as a built-in integer, and
// nil otherwise. Each value parses with the integer value syntax at the
// element's bit size, so a value no input could match fails schema build.
func parseIntegerEnum(enumValues []string, targetType reflect.Type, opts *Options) ([]string, error) {
	if len(enumValues) == 0 {
		return nil, nil
	}

	elemType := targetType
	if isSliceCollection(elemType) {
		elemType = elemType.Elem()
	}

	if !decodesAsBuiltinInteger(elemType, opts) {
		return nil, nil
	}

	decimals := make([]string, 0, len(enumValues))

	for _, value := range enumValues {
		decimal, err := integerEnumDecimal(value, elemType)
		if err != nil {
			return nil, err
		}

		decimals = append(decimals, decimal)
	}

	return decimals, nil
}

// integerEnumDecimal parses one enum value at elemType's signedness and bit
// size and returns its decimal form.
func integerEnumDecimal(value string, elemType reflect.Type) (string, error) {
	var (
		decimal string
		err     error
	)

	switch elemType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var parsed int64

		parsed, err = intx.ParseInt(value, elemType.Bits())
		decimal = strconv.FormatInt(parsed, 10)
	default:
		var parsed uint64

		parsed, err = intx.ParseUint(value, elemType.Bits())
		decimal = strconv.FormatUint(parsed, 10)
	}

	if err == nil {
		return decimal, nil
	}

	if errors.Is(err, strconv.ErrRange) {
		return "", schemaErrorf("enum value %q is out of range for %s", value, elemType).withCause(err)
	}

	return "", schemaErrorf("enum value %q is not a valid %s", value, elemType).withCause(err)
}

// decodesAsBuiltinInteger reports whether the binder decodes elemType through
// the built-in integer parser: an integer kind other than time.Duration with
// no UnmarshalText method and no registered parser.
func decodesAsBuiltinInteger(elemType reflect.Type, opts *Options) bool {
	switch elemType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return false
	}

	if elemType == reflect.TypeFor[time.Duration]() || ParsesItself(elemType) {
		return false
	}

	return opts == nil || opts.HasTypeParser == nil || !opts.HasTypeParser(elemType)
}

// extractGenericType extracts the type parameter T from Flag[T] / Arg[T].
func extractGenericType(containerType reflect.Type) reflect.Type {
	if containerType.Kind() == reflect.Pointer {
		containerType = containerType.Elem()
	}

	if containerType.Kind() == reflect.Struct {
		if valueField, ok := containerType.FieldByName("value"); ok {
			return valueField.Type
		}
	}

	return containerType
}

func hasTag(tag reflect.StructTag, key string) bool {
	_, ok := tag.Lookup(key)

	return ok
}

// isStrictBoolTagKey reports whether key is one of the CONGA tags whose value
// is a strict boolean (see ParseStrictBoolTag).
func isStrictBoolTagKey(key string) bool {
	switch key {
	case "hidden", "negatable", "bootstrap", "implicit", "safe":
		return true
	default:
		return false
	}
}

// validateStrictBoolTags rejects a value other than "", "true", or "false" on
// the strict boolean tags that are CONGA tags for role. A strict boolean tag
// on a role that doesn't take it is left to the candidate check, which names
// the misplacement; a key the developer allowed with app.AllowTags on such a
// role is not a CONGA tag there and isn't checked.
func validateStrictBoolTags(tag reflect.StructTag, role string) error {
	candidates, _ := TagCandidatesForRole(role)

	for _, key := range candidates {
		if !isStrictBoolTagKey(key) {
			continue
		}

		if _, _, err := ParseStrictBoolTag(tag, key); err != nil {
			return err
		}
	}

	return nil
}

// strictBoolTag reads a strict boolean tag that ValidateFieldTags has already
// validated for the field's role: bare presence and "true" are true, absent
// and "false" are false. Callers that can't rely on that order use
// ParseStrictBoolTag, which reports any other value.
func strictBoolTag(tag reflect.StructTag, key string) bool {
	value, ok := tag.Lookup(key)

	return ok && (value == "" || value == "true")
}
