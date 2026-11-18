package spec_test

import (
	"net"
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/spec"
)

type kindTextSlice []string

func (s *kindTextSlice) UnmarshalText(text []byte) error {
	*s = kindTextSlice{string(text)}

	return nil
}

type kindContextMap map[string]string

func (*kindContextMap) UnmarshalTextWithContext(_ []byte, _ coerce.DecodeContext) error {
	return nil
}

func TestIsCollection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		targetType   reflect.Type
		parsesItself bool
		collection   bool
	}{
		{reflect.TypeFor[[]string](), false, true},
		{reflect.TypeFor[map[string]int](), false, true},
		{reflect.TypeFor[[]net.IP](), false, true},
		{reflect.TypeFor[net.IP](), true, false},
		{reflect.TypeFor[kindTextSlice](), true, false},
		{reflect.TypeFor[kindContextMap](), true, false},
		{reflect.TypeFor[string](), false, false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.targetType.String(), func(t *testing.T) {
			t.Parallel()

			parsesItself := spec.ParsesItself(testCase.targetType)
			collection := spec.IsCollection(testCase.targetType)

			assert.Equal(t, testCase.parsesItself, parsesItself)
			assert.Equal(t, testCase.collection, collection)
		})
	}
}
