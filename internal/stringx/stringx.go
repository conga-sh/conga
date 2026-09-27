// Package stringx provides string formatting and stringification primitives for CLI values.
package stringx

import (
	"fmt"
	"reflect"
	"strconv"
)

// stringerType is the fmt.Stringer interface type used for reflection-based
// method-set checks on the slow path.
//
//nolint:gochecknoglobals // immutable interface type shared by stringify calls
var stringerType = reflect.TypeFor[fmt.Stringer]()

// Stringify converts any value into its canonical string representation,
// applying zero-allocation and reflection-free fast paths for string,
// fmt.Stringer, boolean, numeric, and standard CLI types.
func Stringify(value any) string {
	switch typedValue := value.(type) {
	case string:
		return typedValue
	case fmt.Stringer:
		if typedValue != nil {
			return typedValue.String()
		}

		return ""
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
	default:
		return stringifyReflect(typedValue)
	}
}

// stringifyReflect renders values without a fast path. Pointer-receiver
// fmt.Stringer implementations (e.g. url.URL) are reached through an
// addressable copy, so types whose String method has a pointer receiver
// render their canonical form instead of a struct dump; everything else
// falls back to fmt.Sprint.
func stringifyReflect(value any) string {
	reflected := reflect.ValueOf(value)

	if reflected.Kind() == reflect.Struct &&
		!reflected.Type().Implements(stringerType) &&
		reflect.PointerTo(reflected.Type()).Implements(stringerType) {
		addressable := reflect.New(reflected.Type())
		addressable.Elem().Set(reflected)

		if stringer, ok := reflect.TypeAssert[fmt.Stringer](addressable); ok {
			return stringer.String()
		}
	}

	return fmt.Sprint(value)
}
