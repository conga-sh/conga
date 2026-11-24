package coerce_test

import (
	"context"
	"net"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/coerce"
)

type supportsNamedInt int

type supportsTextValue struct{ text string }

func (v *supportsTextValue) UnmarshalText(text []byte) error {
	v.text = string(text)

	return nil
}

type supportsContextValue struct{}

func (*supportsContextValue) UnmarshalTextWithContext(_ []byte, _ coerce.DecodeContext) error {
	return nil
}

type supportsRegistered struct{}

type supportsPointerRegistered struct{}

// TestSupports_MatchesScalarWith pins Supports to ScalarWith's dispatch: for
// every type in the table, Supports reports true exactly when ScalarWith
// doesn't fail with an "unsupported type" error.
func TestSupports_MatchesScalarWith(t *testing.T) {
	t.Parallel()

	registry := coerce.Registry{
		reflect.TypeFor[supportsRegistered](): func(string) (any, error) { return supportsRegistered{}, nil },
		reflect.TypeFor[*supportsPointerRegistered](): func(string) (any, error) {
			return &supportsPointerRegistered{}, nil
		},
	}

	testCases := []struct {
		targetType reflect.Type
		supported  bool
	}{
		{reflect.TypeFor[string](), true},
		{reflect.TypeFor[bool](), true},
		{reflect.TypeFor[int8](), true},
		{reflect.TypeFor[uint64](), true},
		{reflect.TypeFor[float32](), true},
		{reflect.TypeFor[supportsNamedInt](), true},
		{reflect.TypeFor[time.Duration](), true},
		{reflect.TypeFor[url.URL](), true},
		{reflect.TypeFor[*url.URL](), true},
		{reflect.TypeFor[supportsTextValue](), true},
		{reflect.TypeFor[*supportsTextValue](), true},
		{reflect.TypeFor[supportsContextValue](), true},
		{reflect.TypeFor[*supportsContextValue](), true},
		{reflect.TypeFor[net.IP](), true},
		{reflect.TypeFor[supportsRegistered](), true},
		{reflect.TypeFor[*supportsRegistered](), true},
		{reflect.TypeFor[supportsPointerRegistered](), true},
		{reflect.TypeFor[*supportsPointerRegistered](), true},
		{reflect.TypeFor[chan int](), false},
		{reflect.TypeFor[*int](), false},
		{reflect.TypeFor[uintptr](), false},
		{reflect.TypeFor[complex128](), false},
		{reflect.TypeFor[any](), false},
		{reflect.TypeFor[[]string](), false},
		{reflect.TypeFor[map[string]string](), false},
		{reflect.TypeFor[net.IPNet](), false},
		{reflect.TypeFor[context.Context](), false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.targetType.String(), func(t *testing.T) {
			t.Parallel()

			_, err := coerce.ScalarWith("1", testCase.targetType, registry, coerce.DecodeContext{})
			scalarSupported := err == nil || !strings.HasPrefix(err.Error(), "unsupported type ")

			supported := coerce.Supports(testCase.targetType, registry)

			assert.Equal(t, testCase.supported, supported)
			assert.Equal(t, scalarSupported, supported)
		})
	}
}

func TestIsPlainStringMap(t *testing.T) {
	t.Parallel()

	type namedString string

	type namedStringMap map[string]string

	testCases := []struct {
		name     string
		mapType  reflect.Type
		registry coerce.Registry
		plain    bool
	}{
		{name: "string to string", mapType: reflect.TypeFor[map[string]string](), plain: true},
		{name: "named map of strings", mapType: reflect.TypeFor[namedStringMap](), plain: true},
		{name: "named string keys", mapType: reflect.TypeFor[map[namedString]string](), plain: false},
		{name: "named string values", mapType: reflect.TypeFor[map[string]namedString](), plain: false},
		{name: "integer values", mapType: reflect.TypeFor[map[string]int](), plain: false},
		{name: "text unmarshaler values", mapType: reflect.TypeFor[map[string]supportsTextValue](), plain: false},
		{
			name:     "registered string values",
			mapType:  reflect.TypeFor[map[string]namedString](),
			registry: coerce.Registry{reflect.TypeFor[namedString](): func(raw string) (any, error) { return namedString(raw), nil }},
			plain:    false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			plain := coerce.IsPlainStringMap(testCase.mapType, testCase.registry)

			assert.Equal(t, testCase.plain, plain)
		})
	}
}
