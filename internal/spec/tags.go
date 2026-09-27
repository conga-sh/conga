package spec

import (
	"cmp"
	"encoding"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/slicex"
)

// Directives holds the parsed directives of a conga:"..." struct tag.
type Directives struct {
	Ignore           bool
	AllowUnknownTags bool
	Recurse          bool
}

// ParseCongaDirectives parses the directives of a conga:"..." struct tag.
func ParseCongaDirectives(congaTag string) (Directives, error) {
	var directives Directives
	if congaTag == "" {
		return directives, nil
	}

	parts := slicex.SplitAndTrim(congaTag, ",")
	for _, directive := range parts {
		switch directive {
		case "-", "ignore":
			directives.Ignore = true
		case "allow-unknown-tags":
			directives.AllowUnknownTags = true
		case "recurse":
			directives.Recurse = true
		default:
			return Directives{}, fmt.Errorf("%w: unrecognized conga directive %q. Note: flag names and short flags must be declared using name:\"...\" and short:\"...\"", ErrInvalidStructTag, directive)
		}
	}

	return directives, nil
}

// ParseRangeSpec parses a range tag value (min..max, min.., ..max, or exact).
func ParseRangeSpec(tag string) (*Range, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, fmt.Errorf("%w: tag cannot be empty", ErrInvalidRangeSyntax)
	}

	if strings.Contains(tag, "..") {
		parts := strings.Split(tag, "..")
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: %q (expected min..max, min.., or ..max)", ErrInvalidRangeSyntax, tag)
		}

		minStr := strings.TrimSpace(parts[0])
		maxStr := strings.TrimSpace(parts[1])

		if minStr == "" && maxStr == "" {
			return nil, fmt.Errorf("%w: %q (at least one bound required)", ErrInvalidRangeSyntax, tag)
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

// ValidateScalarRange parses the min/max bounds of rangeSpec with parse and
// checks their order. boundKind names the value kind in parse-error messages
// ("duration", "integer", "unsigned integer", "float", "byte size").
func ValidateScalarRange[T cmp.Ordered](rangeSpec *Range, boundKind string, parse func(string) (T, error)) error {
	var (
		minValue T
		maxValue T
	)

	if rangeSpec.HasMin {
		v, err := parse(rangeSpec.MinStr)
		if err != nil {
			return fmt.Errorf("%w: invalid %s min bound %q", ErrInvalidRangeSyntax, boundKind, rangeSpec.MinStr)
		}

		minValue = v
	}

	if rangeSpec.HasMax {
		v, err := parse(rangeSpec.MaxStr)
		if err != nil {
			return fmt.Errorf("%w: invalid %s max bound %q", ErrInvalidRangeSyntax, boundKind, rangeSpec.MaxStr)
		}

		maxValue = v
	}

	if rangeSpec.HasMin && rangeSpec.HasMax && minValue > maxValue {
		return fmt.Errorf("%w: min bound %q cannot be greater than max bound %q", ErrInvalidRangeSyntax, rangeSpec.MinStr, rangeSpec.MaxStr)
	}

	return nil
}

// ValidateCountRangeSpec validates len/items range bounds as non-negative integers.
func ValidateCountRangeSpec(spec *Range, tagName string) error {
	if spec == nil {
		return nil
	}

	var minValue, maxValue int

	if spec.HasMin {
		v, err := strconv.Atoi(spec.MinStr)
		if err != nil || v < 0 {
			return fmt.Errorf("%w: invalid %s min bound %q (must be non-negative integer)", ErrInvalidRangeSyntax, tagName, spec.MinStr)
		}

		minValue = v
	}

	if spec.HasMax {
		v, err := strconv.Atoi(spec.MaxStr)
		if err != nil || v < 0 {
			return fmt.Errorf("%w: invalid %s max bound %q (must be non-negative integer)", ErrInvalidRangeSyntax, tagName, spec.MaxStr)
		}

		maxValue = v
	}

	if spec.HasMin && spec.HasMax && minValue > maxValue {
		return fmt.Errorf("%w: %s min bound %q cannot be greater than max bound %q", ErrInvalidRangeSyntax, tagName, spec.MinStr, spec.MaxStr)
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
		return false, true, fmt.Errorf("%w: tag %q expects \"true\" or \"false\", got %q", ErrInvalidStructTag, key, raw)
	}
}

// ParseGlobPatterns splits a comma-separated glob tag value into trimmed
// patterns and validates each one: empty elements, path separators, and
// malformed filepath.Match syntax are rejected at schema build. Patterns
// match file names (not paths) during completion traversal, so a
// wildcard-free pattern such as "Makefile" matches that literal name.
// Errors wrap ErrInvalidStructTag.
func ParseGlobPatterns(value string) ([]string, error) {
	elements := strings.Split(value, ",")
	for _, element := range elements {
		if strings.TrimSpace(element) == "" {
			return nil, fmt.Errorf("%w: empty glob pattern element (expected comma-separated patterns like \"Makefile,Dockerfile\" or \"*.yaml\")", ErrInvalidStructTag)
		}
	}

	patterns := slicex.Map(elements, strings.TrimSpace)

	for _, pattern := range patterns {
		if strings.ContainsAny(pattern, `/\`) {
			return nil, fmt.Errorf("%w: glob pattern %q must not contain path separators (patterns match file names during completion traversal)", ErrInvalidStructTag, pattern)
		}

		if _, err := filepath.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("%w: glob pattern %q is malformed: %w", ErrInvalidStructTag, pattern, err)
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
		return "", fmt.Errorf("%w: tag 'summary' cannot be empty", ErrInvalidStructTag)
	}

	if strings.ContainsAny(summaryTag, "\r\n") {
		return "", fmt.Errorf("%w: tag 'summary' must be a single line without newlines", ErrInvalidStructTag)
	}

	return trimmed, nil
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
		return "", "", fmt.Errorf("%w: %q", ErrInvalidStructTag, fullTag)
	}

	key := tagStr[:i]
	remaining := tagStr[i+1:]

	return key, remaining, nil
}

func scanTagValue(tagStr string, fullTag reflect.StructTag) (string, string, error) {
	if tagStr == "" || tagStr[0] != '"' {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidStructTag, fullTag)
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
		return "", "", fmt.Errorf("%w: %q", ErrInvalidStructTag, fullTag)
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

	isMap := innerType.Kind() == reflect.Map
	isSlice := innerType.Kind() == reflect.Slice
	isBool := innerType.Kind() == reflect.Bool
	isNegatable := isBool && parsePresenceOrTrueTag(structField.Tag, "negatable")

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

func parseFlagSpec(structField reflect.StructField, flagHandle any, classification container.Classification, fieldTags FieldTags, opts Options) (*Flag, error) {
	kindInfo := extractFlagKindInfo(structField, classification)
	name := extractFlagName(structField)
	shortRune := extractFlagShort(structField.Tag)

	envVars, envExcluded := parseFlagEnvVars(structField.Tag, name, opts.EnvPrefix)
	enumValues := SplitTagList(structField.Tag, "enum")
	aliases := SplitTagList(structField.Tag, "aliases")

	rangeSpec, lenSpec, itemsSpec, err := parseValidationSpecs(structField.Tag, kindInfo.targetType)
	if err != nil {
		return nil, err
	}

	helpText := structField.Tag.Get("help")
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
		Enum:        enumValues,
		Range:       rangeSpec,
		Len:         lenSpec,
		Items:       itemsSpec,
		Help:        helpText,
		Placeholder: placeholder,
		Group:       structField.Tag.Get("group"),
		Hidden:      parsePresenceOrTrueTag(structField.Tag, "hidden"),
		Safe:        safe,
		Deprecated:  structField.Tag.Get("deprecated"),
		Sep:         kindInfo.sep,
		IsBool:      kindInfo.isBool,
		IsSlice:     kindInfo.isSlice,
		IsMap:       kindInfo.isMap,
		IsCounter:   kindInfo.isCounter,
		IsNegatable: kindInfo.isNegatable,
		Required:    classification.Required,
		Predictor:   predSpec,
		TargetType:  kindInfo.targetType,
		Handle:      flagHandle,
	}, nil
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

func parseValidationSpecs(structTag reflect.StructTag, targetType reflect.Type) (*Range, *Range, *Range, error) {
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

		if err := validateRangeSpec(spec, targetType); err != nil {
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

func validateRangeSpec(rangeSpec *Range, targetType reflect.Type) error {
	if rangeSpec == nil || targetType == nil {
		return nil
	}

	elemType := targetType
	if elemType.Kind() == reflect.Slice {
		elemType = elemType.Elem()
	}

	// Byte-size bounds are dispatched on the element type's bare name rather
	// than its package path: the concrete ByteSize type lives in package conga
	// (which imports spec), so spec cannot reference it by type. Any foreign
	// type named ByteSize is therefore routed through the same bound-parse
	// path; types without a TextUnmarshaler silently skip bound parsing.
	if elemType.Name() == "ByteSize" {
		return validateByteSizeRange(rangeSpec, elemType)
	}

	switch elemType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if elemType == reflect.TypeFor[time.Duration]() {
			return ValidateScalarRange(rangeSpec, "duration", time.ParseDuration)
		}

		return ValidateScalarRange(rangeSpec, "integer", func(raw string) (int64, error) {
			return strconv.ParseInt(raw, 10, 64)
		})

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return ValidateScalarRange(rangeSpec, "unsigned integer", func(raw string) (uint64, error) {
			return strconv.ParseUint(raw, 10, 64)
		})

	case reflect.Float32, reflect.Float64:
		return ValidateScalarRange(rangeSpec, "float", func(raw string) (float64, error) {
			return strconv.ParseFloat(raw, 64)
		})

	default:
		return fmt.Errorf("%w: range tag is not supported on type %s", ErrInvalidRangeSyntax, elemType.String())
	}
}

func validateByteSizeRange(rangeSpec *Range, elemType reflect.Type) error {
	parse := func(raw string) (int64, error) {
		unmarshaler, ok := reflect.TypeAssert[encoding.TextUnmarshaler](reflect.New(elemType))
		if !ok {
			// Foreign types without a TextUnmarshaler skip bound parsing; the
			// unparseable-bounds lint gap is tracked separately (Phase 2.9).
			return 0, nil
		}

		if err := unmarshaler.UnmarshalText([]byte(raw)); err != nil {
			return 0, err
		}

		return extractByteSize(unmarshaler), nil
	}

	return ValidateScalarRange(rangeSpec, "byte size", parse)
}

func extractByteSize(v any) int64 {
	if bc, ok := v.(interface{ Bytes() int64 }); ok {
		return bc.Bytes()
	}

	value := reflect.ValueOf(v)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := value.Uint()
		if u > math.MaxInt64 {
			return math.MaxInt64
		}

		return int64(u)
	default:
		return 0
	}
}

func parseArgSpec(structField reflect.StructField, argHandle any, classification container.Classification, fieldTags FieldTags, opts Options) (*Arg, error) {
	name := cond.SetOrDefaultFunc(structField.Tag.Get("name"), func() string {
		return fmt.Sprintf("<%s>", casing.Kebab(structField.Name))
	})

	innerType := extractGenericType(structField.Type)
	isSlice := innerType.Kind() == reflect.Slice
	isMap := innerType.Kind() == reflect.Map

	sep, sepFound := structField.Tag.Lookup("sep")
	if !sepFound {
		sep = ","
	}

	enumValues := SplitTagList(structField.Tag, "enum")

	rangeSpec, lenSpec, itemsSpec, err := parseValidationSpecs(structField.Tag, innerType)
	if err != nil {
		return nil, err
	}

	predSpec, err := parsePredictorSpec(structField.Tag, innerType, false, fieldTags, opts)
	if err != nil {
		return nil, err
	}

	return &Arg{
		Name:       name,
		Default:    structField.Tag.Get("default"),
		Help:       structField.Tag.Get("help"),
		Enum:       enumValues,
		Range:      rangeSpec,
		Len:        lenSpec,
		Items:      itemsSpec,
		Sep:        sep,
		IsSlice:    isSlice,
		IsMap:      isMap,
		Required:   classification.Required,
		Predictor:  predSpec,
		TargetType: innerType,
		Handle:     argHandle,
	}, nil
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

func parsePresenceOrTrueTag(tag reflect.StructTag, key string) bool {
	value, ok := tag.Lookup(key)
	if !ok {
		return false
	}

	return value == "" || value == "true"
}

func parseFlagEnvVars(tag reflect.StructTag, flagName string, envPrefix string) ([]string, bool) {
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
