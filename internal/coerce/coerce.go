// Package coerce converts raw CLI values to Go types using built-in and
// registered parsers.
package coerce

import (
	"encoding"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/conga-sh/conga/internal/spec"
)

var (
	// ErrCoerce indicates a raw value could not be converted to its target type.
	ErrCoerce = errors.New("coercion error")

	// ErrUnsupportedType indicates a value could not be coerced to its target type.
	ErrUnsupportedType = fmt.Errorf("%w: unsupported type", ErrCoerce)
)

const (
	urlErrorMalformed     urlErrorKind = iota // url.Parse rejected the value
	urlErrorMissingScheme                     // parsed, but Scheme == ""
)

const (
	numErrorInvalid    numErrorKind = iota // strconv syntax failure
	numErrorOutOfRange                     // strconv.ErrRange
)

const (
	mapPairErrorInvalid  mapPairErrorKind = iota // no '=' separator
	mapPairErrorEmptyKey                         // key is empty after trimming
)

// TypeParser parses a raw string token into a concrete value of the registered target type.
type TypeParser func(string) (any, error)

// Registry maps reflect.Type to custom TypeParser functions.
type Registry map[reflect.Type]TypeParser

// DecodeContext carries the ambient input for coercing one raw value into
// its target type: the owning field's declared custom tags and the
// engine-configured standard streams. The zero value selects no tags and
// process defaults (os.Stdin/os.Stdout), so values decoded outside an App
// run behave identically to direct UnmarshalText calls. It is a plain value
// struct: unlike context.Context it carries no cancellation or deadlines.
type DecodeContext struct {
	// Tags holds the owning field's declared custom tags (see
	// conga.TagSpecProvider); nil for fields that declare no registered tags.
	Tags spec.FieldTags

	// Stdin is the engine-configured standard input (App.Stdin); nil selects
	// os.Stdin for types implementing "-" conventions.
	Stdin io.Reader

	// Stdout is the engine-configured standard output (App.Stdout); nil
	// selects os.Stdout.
	Stdout io.Writer
}

// MapEntry is a key-value pair parsed from raw CLI tokens.
type MapEntry struct {
	Key   string
	Value string
}

// Value converts a raw string value into the target type T.
func Value[T any](raw string) (T, error) {
	var zero T

	value, err := Scalar(raw, reflect.TypeFor[T]())
	if err != nil {
		return zero, err
	}

	typed, ok := value.(T)
	if !ok {
		return zero, &conversionError{value: value, targetType: reflect.TypeFor[T]()}
	}

	return typed, nil
}

// ScalarWith converts a raw string value into the target type represented by targetType,
// first checking if a custom parser is registered for targetType in reg. decodeContext
// carries the owning field's declared custom tags and the engine-configured standard
// streams for context-aware text unmarshaling.
func ScalarWith(raw string, targetType reflect.Type, reg Registry, decodeContext DecodeContext) (any, error) {
	if len(reg) > 0 {
		if value, handled, err := coerceCustomRegistry(raw, targetType, reg); handled || err != nil {
			return value, err
		}
	}

	return scalar(raw, targetType, decodeContext)
}

// Scalar converts a raw string value into the target type represented by targetType.
func Scalar(raw string, targetType reflect.Type) (any, error) {
	return scalar(raw, targetType, DecodeContext{})
}

// SliceWith converts a slice of raw string tokens into a slice of targetType using the supplied custom registry.
// decodeContext carries the owning field's declared custom tags and the
// engine-configured standard streams for context-aware text unmarshaling of each element.
func SliceWith(raws []string, sliceType reflect.Type, sep string, reg Registry, decodeContext DecodeContext) (any, error) {
	elemType := sliceType.Elem()
	sliceValue := reflect.MakeSlice(sliceType, 0, len(raws))

	preserveWhitespace := sep != "" && sepDisabled(sep) && elemType.Kind() == reflect.String

	for _, raw := range raws {
		items := splitMultiValue(raw, sep)

		for _, item := range items {
			if !preserveWhitespace {
				item = strings.TrimSpace(item)
			}

			elem, err := ScalarWith(item, elemType, reg, decodeContext)
			if err != nil {
				return nil, err
			}

			sliceValue = reflect.Append(sliceValue, reflect.ValueOf(elem))
		}
	}

	return sliceValue.Interface(), nil
}

// SplitMapItems splits raw string tokens into trimmed KEY=VALUE items using sep.
func SplitMapItems(raws []string, sep string) []string {
	var items []string

	for _, raw := range raws {
		for _, item := range splitMultiValue(raw, sep) {
			item = strings.TrimSpace(item)
			if item != "" {
				items = append(items, item)
			}
		}
	}

	return items
}

// MapEntries converts a slice of raw string tokens into an ordered slice of
// MapEntry pairs, preserving input token sequence.
func MapEntries(raws []string, sep string) ([]MapEntry, error) {
	items := SplitMapItems(raws, sep)
	entries := make([]MapEntry, 0, len(items))

	for _, item := range items {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return nil, &mapPairError{item: item, kind: mapPairErrorInvalid}
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return nil, &mapPairError{item: item, kind: mapPairErrorEmptyKey}
		}

		entries = append(entries, MapEntry{
			Key:   key,
			Value: strings.TrimSpace(value),
		})
	}

	return entries, nil
}

// Map converts a slice of raw string tokens into a map[string]string.
func Map(raws []string, sep string) (map[string]string, error) {
	entries, err := MapEntries(raws, sep)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(entries))
	for _, entry := range entries {
		result[entry.Key] = entry.Value
	}

	return result, nil
}

// contextAwareTextUnmarshaler mirrors conga.ContextAwareTextUnmarshaler:
// domain types that need the owning field's tags and/or the engine's
// ambient environment during value decoding. The coercion pipeline prefers
// it over plain encoding.TextUnmarshaler.
type contextAwareTextUnmarshaler interface {
	UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error
}

// unsupportedTypeError is the root cause for target types coercion cannot
// handle (json.UnsupportedTypeError precedent).
type unsupportedTypeError struct {
	targetType reflect.Type
}

func (e *unsupportedTypeError) Error() string {
	return fmt.Sprintf("unsupported type %s", e.targetType)
}

func (e *unsupportedTypeError) Unwrap() error { return ErrUnsupportedType }

// booleanError is the root cause for unparseable boolean values.
type booleanError struct {
	raw string
}

func (e *booleanError) Error() string {
	return fmt.Sprintf("invalid boolean value %q, expected true or false", e.raw)
}

func (e *booleanError) Unwrap() error { return ErrCoerce }

// durationError is the root cause for unparseable duration values.
type durationError struct {
	raw string
}

func (e *durationError) Error() string {
	return fmt.Sprintf("invalid duration %q (valid units: ms, s, m, h)", e.raw)
}

func (e *durationError) Unwrap() error { return ErrCoerce }

// urlErrorKind identifies the URL expectation violated by a raw value.
type urlErrorKind uint8

type urlError struct {
	raw  string
	kind urlErrorKind
}

func (e *urlError) Error() string {
	switch e.kind {
	case urlErrorMalformed:
		return fmt.Sprintf("invalid URL %q", e.raw)
	case urlErrorMissingScheme:
		return fmt.Sprintf("invalid URL %q (missing scheme, e.g. https://)", e.raw)
	default:
		return fmt.Sprintf("invalid URL %q", e.raw)
	}
}

func (e *urlError) Unwrap() error { return ErrCoerce }

// numErrorKind identifies the numeric parse failure mode.
type numErrorKind uint8

type numError struct {
	raw        string
	noun       string       // "integer" | "unsigned integer" | "float" (invalid kind only)
	targetType reflect.Type // out-of-range kind only
	kind       numErrorKind
}

func (e *numError) Error() string {
	switch e.kind {
	case numErrorInvalid:
		return fmt.Sprintf("invalid %s value %q", e.noun, e.raw)
	case numErrorOutOfRange:
		return fmt.Sprintf("value %q is out of range for %s", e.raw, e.targetType)
	default:
		return fmt.Sprintf("invalid numeric value %q", e.raw)
	}
}

func (e *numError) Unwrap() error { return ErrCoerce }

// mapPairErrorKind identifies the KEY=VALUE syntax failure mode.
type mapPairErrorKind uint8

type mapPairError struct {
	item string
	kind mapPairErrorKind
}

func (e *mapPairError) Error() string {
	switch e.kind {
	case mapPairErrorInvalid:
		return fmt.Sprintf("invalid key-value pair %q, expected KEY=VALUE", e.item)
	case mapPairErrorEmptyKey:
		return fmt.Sprintf("empty key in key-value pair %q", e.item)
	default:
		return fmt.Sprintf("invalid key-value pair %q", e.item)
	}
}

func (e *mapPairError) Unwrap() error { return ErrCoerce }

// conversionError is the root cause for the defensive type-assertion failure
// in Value (a custom parser returned a value of the wrong type).
type conversionError struct {
	value      any
	targetType reflect.Type
}

func (e *conversionError) Error() string {
	return fmt.Sprintf("cannot convert %T to %s", e.value, e.targetType)
}

func (e *conversionError) Unwrap() error { return ErrCoerce }

// assignabilityError is the root cause for registered-parser results that are
// not assignable to the target type.
type assignabilityError struct {
	valueType  reflect.Type
	targetType reflect.Type
}

func (e *assignabilityError) Error() string {
	return fmt.Sprintf("registered parser returned %s which is not assignable to %s", e.valueType, e.targetType)
}

func (e *assignabilityError) Unwrap() error { return ErrCoerce }

// wrappedError anchors an externally produced error (custom registry parser,
// TextUnmarshaler) in the ErrCoerce taxonomy while delegating prose to it.
type wrappedError struct {
	cause error
}

func (e *wrappedError) Error() string {
	if e.cause == nil {
		panic("coerce: invariant violation: wrappedError requires a non-nil cause")
	}

	return e.cause.Error()
}

func (e *wrappedError) Unwrap() error { return e.cause }

func (e *wrappedError) Is(target error) bool { return target == ErrCoerce }

// scalar converts a raw string value into the target type represented by
// targetType, delivering the field's declared tags and the engine's
// configured streams to context-aware text unmarshalers.
func scalar(raw string, targetType reflect.Type, decodeContext DecodeContext) (any, error) {
	// 1. TextUnmarshaler (context-aware preferred)
	if value, ok, err := coerceTextUnmarshaler(raw, targetType, decodeContext); ok || err != nil {
		return value, err
	}

	// 2. Standard primitives & specialized types
	switch targetType.Kind() {
	case reflect.String:
		return convertValue(raw, targetType), nil

	case reflect.Bool:
		return coerceBool(raw, targetType)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if targetType == reflect.TypeFor[time.Duration]() {
			return coerceDuration(raw)
		}

		return coerceSignedInt(raw, targetType)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return coerceUnsignedInt(raw, targetType)

	case reflect.Float32, reflect.Float64:
		return coerceFloat(raw, targetType)

	case reflect.Pointer:
		if targetType == reflect.TypeFor[*url.URL]() {
			return coerceURL(raw)
		}

		return nil, &unsupportedTypeError{targetType: targetType}

	case reflect.Struct:
		if targetType == reflect.TypeFor[url.URL]() {
			parsedURL, err := coerceURL(raw)
			if err != nil {
				return nil, err
			}

			return *parsedURL, nil
		}

		return nil, &unsupportedTypeError{targetType: targetType}

	default:
		return nil, &unsupportedTypeError{targetType: targetType}
	}
}

func unmarshalTextBytes(unmarshaler encoding.TextUnmarshaler, raw string) error {
	if err := unmarshaler.UnmarshalText([]byte(raw)); err != nil {
		if errors.Is(err, ErrCoerce) {
			return err
		}

		return &wrappedError{cause: err}
	}

	return nil
}

func coerceTextUnmarshaler(raw string, targetType reflect.Type, decodeContext DecodeContext) (any, bool, error) {
	if targetType.Kind() == reflect.Pointer {
		elemValue := reflect.New(targetType.Elem())

		handled, err := unmarshalTextInto(elemValue, raw, decodeContext)
		if handled || err != nil {
			return elemValue.Interface(), true, err
		}
	}

	ptrValue := reflect.New(targetType)

	handled, err := unmarshalTextInto(ptrValue, raw, decodeContext)
	if handled || err != nil {
		return ptrValue.Elem().Interface(), true, err
	}

	return nil, false, nil
}

// unmarshalTextInto unmarshals raw into the pointer value, preferring the
// context-aware interface over plain encoding.TextUnmarshaler. It reports
// whether the value type handled the text.
func unmarshalTextInto(target reflect.Value, raw string, decodeContext DecodeContext) (bool, error) {
	if unmarshaler, ok := reflect.TypeAssert[contextAwareTextUnmarshaler](target); ok {
		err := unmarshaler.UnmarshalTextWithContext([]byte(raw), decodeContext)
		if err != nil && !errors.Is(err, ErrCoerce) {
			return true, &wrappedError{cause: err}
		}

		return true, err
	}

	if unmarshaler, ok := reflect.TypeAssert[encoding.TextUnmarshaler](target); ok {
		return true, unmarshalTextBytes(unmarshaler, raw)
	}

	return false, nil
}

func coerceURL(raw string) (*url.URL, error) {
	parsedURL, err := url.Parse(raw)
	if err != nil {
		return nil, &urlError{raw: raw, kind: urlErrorMalformed}
	}

	if parsedURL.Scheme == "" {
		return nil, &urlError{raw: raw, kind: urlErrorMissingScheme}
	}

	return parsedURL, nil
}

func coerceBool(raw string, targetType reflect.Type) (any, error) {
	if raw == "" || raw == "true" || raw == "1" {
		return convertValue(true, targetType), nil
	}

	if raw == "false" || raw == "0" {
		return convertValue(false, targetType), nil
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, &booleanError{raw: raw}
	}

	return convertValue(parsed, targetType), nil
}

func coerceDuration(raw string) (time.Duration, error) {
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, &durationError{raw: raw}
	}

	return duration, nil
}

func coerceSignedInt(raw string, targetType reflect.Type) (any, error) {
	bitSize, ok := integerBitSize(targetType.Kind())
	if !ok {
		return nil, &unsupportedTypeError{targetType: targetType}
	}

	num, err := strconv.ParseInt(raw, 10, bitSize)
	if err != nil {
		return nil, wrapParseNumError(err, raw, targetType, "integer")
	}

	return convertValue(num, targetType), nil
}

func coerceUnsignedInt(raw string, targetType reflect.Type) (any, error) {
	bitSize, ok := integerBitSize(targetType.Kind())
	if !ok {
		return nil, &unsupportedTypeError{targetType: targetType}
	}

	num, err := strconv.ParseUint(raw, 10, bitSize)
	if err != nil {
		return nil, wrapParseNumError(err, raw, targetType, "unsigned integer")
	}

	return convertValue(num, targetType), nil
}

func coerceFloat(raw string, targetType reflect.Type) (any, error) {
	bitSize, ok := floatBitSize(targetType.Kind())
	if !ok {
		return nil, &unsupportedTypeError{targetType: targetType}
	}

	num, err := strconv.ParseFloat(raw, bitSize)
	if err != nil {
		return nil, wrapParseNumError(err, raw, targetType, "float")
	}

	return convertValue(num, targetType), nil
}

func convertValue(value any, targetType reflect.Type) any {
	if reflect.TypeOf(value) == targetType {
		return value
	}

	return reflect.ValueOf(value).Convert(targetType).Interface()
}

func splitMultiValue(raw, sep string) []string {
	if !sepDisabled(sep) && strings.Contains(raw, sep) {
		return strings.Split(raw, sep)
	}

	return []string{raw}
}

// sepDisabled reports whether sep disables multi-value splitting.
func sepDisabled(sep string) bool {
	return sep == "" || sep == "none" || sep == "off" || sep == "-"
}

func integerBitSize(kind reflect.Kind) (int, bool) {
	switch kind {
	case reflect.Int, reflect.Uint:
		return 0, true
	case reflect.Int8, reflect.Uint8:
		return 8, true
	case reflect.Int16, reflect.Uint16:
		return 16, true
	case reflect.Int32, reflect.Uint32:
		return 32, true
	case reflect.Int64, reflect.Uint64:
		return 64, true
	default:
		return 0, false
	}
}

func floatBitSize(kind reflect.Kind) (int, bool) {
	switch kind {
	case reflect.Float32:
		return 32, true
	case reflect.Float64:
		return 64, true
	default:
		return 0, false
	}
}

func wrapParseNumError(err error, raw string, targetType reflect.Type, noun string) error {
	if errors.Is(err, strconv.ErrRange) {
		return &numError{raw: raw, targetType: targetType, kind: numErrorOutOfRange}
	}

	return &numError{raw: raw, noun: noun, kind: numErrorInvalid}
}

func ensureAssignable(value any, targetType reflect.Type) (any, error) {
	if value == nil {
		return reflect.Zero(targetType).Interface(), nil
	}

	valType := reflect.TypeOf(value)
	if !valType.AssignableTo(targetType) {
		return nil, &assignabilityError{valueType: valType, targetType: targetType}
	}

	return value, nil
}

func coercePointerTarget(raw string, targetType reflect.Type, reg Registry) (any, bool, error) {
	if targetType.Kind() != reflect.Pointer {
		return nil, false, nil
	}

	elemType := targetType.Elem()

	parser, ok := reg[elemType]
	if !ok {
		return nil, false, nil
	}

	value, err := parser(raw)
	if err != nil {
		return nil, true, &wrappedError{cause: err}
	}

	checkedValue, err := ensureAssignable(value, elemType)
	if err != nil {
		return nil, true, err
	}

	ptrValue := reflect.New(elemType)
	ptrValue.Elem().Set(reflect.ValueOf(checkedValue))

	return ptrValue.Interface(), true, nil
}

func coerceValueTargetFromPointer(raw string, targetType reflect.Type, reg Registry) (any, bool, error) {
	ptrType := reflect.PointerTo(targetType)

	parser, ok := reg[ptrType]
	if !ok {
		return nil, false, nil
	}

	value, err := parser(raw)
	if err != nil {
		return nil, true, &wrappedError{cause: err}
	}

	reflectedValue := reflect.ValueOf(value)
	if reflectedValue.Kind() == reflect.Pointer {
		if reflectedValue.IsNil() {
			return reflect.Zero(targetType).Interface(), true, nil
		}

		valElem, err := ensureAssignable(reflectedValue.Elem().Interface(), targetType)
		if err != nil {
			return nil, true, err
		}

		return valElem, true, nil
	}

	checkedValue, err := ensureAssignable(value, targetType)
	if err != nil {
		return nil, true, err
	}

	return checkedValue, true, nil
}

func coerceCustomRegistry(raw string, targetType reflect.Type, reg Registry) (any, bool, error) {
	if parser, ok := reg[targetType]; ok {
		value, err := parser(raw)
		if err != nil {
			return nil, true, &wrappedError{cause: err}
		}

		checkedValue, err := ensureAssignable(value, targetType)
		if err != nil {
			return nil, true, err
		}

		return checkedValue, true, nil
	}

	if value, handled, err := coercePointerTarget(raw, targetType, reg); handled || err != nil {
		return value, handled, err
	}

	return coerceValueTargetFromPointer(raw, targetType, reg)
}
