package network_test

import (
	"encoding"
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/errlines"
	"github.com/conga-sh/conga/network"
)

// decodeInto returns a function that decodes text into target.
func decodeInto(target encoding.TextUnmarshaler, text string) func() error {
	return func() error {
		return target.UnmarshalText([]byte(text))
	}
}

func TestUnmarshalText_RejectionsAreRootErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		decode  func() error
		message string
		note    string
	}{
		{
			name:    "HTTP method",
			decode:  decodeInto(new(network.HTTPMethod), "FETCH"),
			message: `unrecognized HTTP method "FETCH"`,
		},
		{
			name:    "HTTP status out of range",
			decode:  decodeInto(new(network.HTTPStatus), "99"),
			message: "invalid HTTP status 99",
			note:    "must be between 100 and 599",
		},
		{
			name:    "HTTP status text",
			decode:  decodeInto(new(network.HTTPStatus), "No Such Status"),
			message: `invalid HTTP status: unknown HTTP status text "No Such Status"`,
		},
		{
			name:    "MIME type",
			decode:  decodeInto(new(network.MIMEType), "json"),
			message: `invalid MIME type "json"`,
			note:    "expected format type/subtype, e.g. application/json",
		},
		{
			name:    "port out of range",
			decode:  decodeInto(new(network.Port), "70000"),
			message: "invalid port 70000",
			note:    "must be between 1 and 65535",
		},
		{
			name:    "port service name",
			decode:  decodeInto(new(network.Port), "no-such-service"),
			message: `invalid port: unknown service name "no-such-service"`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.decode()

			assert.Error(t, err)
			assert.Equal(t, testCase.message, err.Error())
			assert.Equal(t, testCase.note, errlines.Note(err))
			assert.Equal(t, nil, errors.Unwrap(err))
		})
	}
}
