package spec_test

import (
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

func TestParseStrictBoolTag(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		tag     reflect.StructTag
		value   bool
		present bool
		message string
	}{
		{name: "absent", tag: `help:"x"`, value: false, present: false},
		{name: "bare", tag: `hidden:""`, value: true, present: true},
		{name: "true", tag: `hidden:"true"`, value: true, present: true},
		{name: "false", tag: `hidden:"false"`, value: false, present: true},
		{name: "yes", tag: `hidden:"yes"`, present: true, message: `tag "hidden" expects "true" or "false", got "yes"`},
		{name: "capitalised", tag: `hidden:"True"`, present: true, message: `tag "hidden" expects "true" or "false", got "True"`},
		{name: "numeric", tag: `hidden:"1"`, present: true, message: `tag "hidden" expects "true" or "false", got "1"`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			value, present, err := spec.ParseStrictBoolTag(testCase.tag, "hidden")

			assert.Equal(t, testCase.value, value)
			assert.Equal(t, testCase.present, present)

			if testCase.message == "" {
				assert.NoError(t, err)

				return
			}

			assert.True(t, spec.IsSchemaError(err))
			assert.Equal(t, testCase.message, err.Error())
		})
	}
}
