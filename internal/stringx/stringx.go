// Package stringx provides string formatting and stringification primitives for CLI values.
package stringx

import (
	"encoding"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Stringify converts any value into its canonical string representation,
// applying zero-allocation and reflection-free fast paths for string,
// fmt.Stringer, encoding.TextMarshaler, boolean, numeric, and standard CLI types.
func Stringify(value any) string {
	switch typedValue := value.(type) {
	case string:
		return typedValue
	case fmt.Stringer:
		if typedValue != nil {
			return typedValue.String()
		}

		return ""
	case encoding.TextMarshaler:
		return stringifyTextMarshaler(typedValue)
	case bool:
		return strconv.FormatBool(typedValue)
	case int:
		return strconv.Itoa(typedValue)
	case int64:
		return strconv.FormatInt(typedValue, 10)
	case uint:
		return strconv.FormatUint(uint64(typedValue), 10)
	case uint64:
		return strconv.FormatUint(typedValue, 10)
	case float64:
		return strconv.FormatFloat(typedValue, 'g', -1, 64)
	case []string:
		if len(typedValue) == 0 {
			return ""
		}

		return "[" + strings.Join(typedValue, ", ") + "]"
	default:
		return stringifyReflect(typedValue)
	}
}

// stringifyReflect renders values without a fast path. Pointer-receiver
// fmt.Stringer and encoding.TextMarshaler implementations are reached
// through an addressable copy; maps and slices render in canonical CLI format
// (sorted key=value pairs and comma-joined brackets); everything else falls
// back to fmt.Sprint.
func stringifyReflect(value any) string {
	if value == nil {
		return "<nil>"
	}

	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return ""
	}

	if text, ok := stringifyAddressable(reflected); ok {
		return text
	}

	switch reflected.Kind() {
	case reflect.Map:
		return stringifyMap(reflected)
	case reflect.Slice:
		return stringifySlice(reflected, value)
	default:
		return fmt.Sprint(value)
	}
}

func stringifyTextMarshaler(marshaler encoding.TextMarshaler) string {
	if marshaler == nil {
		return ""
	}

	if text, err := marshaler.MarshalText(); err == nil {
		return string(text)
	}

	return ""
}

func stringifyAddressable(reflected reflect.Value) (string, bool) {
	if reflected.Kind() != reflect.Struct {
		return "", false
	}

	stringerType := reflect.TypeFor[fmt.Stringer]()
	if !reflected.Type().Implements(stringerType) &&
		reflect.PointerTo(reflected.Type()).Implements(stringerType) {
		addressable := reflect.New(reflected.Type())
		addressable.Elem().Set(reflected)

		if stringer, ok := reflect.TypeAssert[fmt.Stringer](addressable); ok {
			return stringer.String(), true
		}
	}

	textMarshalerType := reflect.TypeFor[encoding.TextMarshaler]()
	if !reflected.Type().Implements(textMarshalerType) &&
		reflect.PointerTo(reflected.Type()).Implements(textMarshalerType) {
		addressable := reflect.New(reflected.Type())
		addressable.Elem().Set(reflected)

		if marshaler, ok := reflect.TypeAssert[encoding.TextMarshaler](addressable); ok {
			if text, err := marshaler.MarshalText(); err == nil {
				return string(text), true
			}
		}
	}

	return "", false
}

func stringifyMap(reflected reflect.Value) string {
	if reflected.Len() == 0 {
		return ""
	}

	keys := reflected.MapKeys()
	pairs := make([]string, 0, len(keys))

	for _, key := range keys {
		kStr := Stringify(key.Interface())
		vStr := Stringify(reflected.MapIndex(key).Interface())
		pairs = append(pairs, kStr+"="+vStr)
	}

	slices.Sort(pairs)

	return strings.Join(pairs, ", ")
}

func stringifySlice(reflected reflect.Value, value any) string {
	if reflected.Type().Elem().Kind() == reflect.Uint8 {
		return string(reflected.Bytes())
	}

	if reflected.Len() == 0 {
		return ""
	}

	if reflected.Type().Elem().Kind() == reflect.String {
		items := make([]string, reflected.Len())
		for i := range reflected.Len() {
			items[i] = reflected.Index(i).String()
		}

		return "[" + strings.Join(items, ", ") + "]"
	}

	return fmt.Sprint(value)
}
