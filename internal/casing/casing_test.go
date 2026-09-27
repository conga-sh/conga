package casing_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/casing"
)

func TestKebab(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected string
	}{
		{"ServerPort", "server-port"},
		{"APIKey", "api-key"},
		{"NodeID", "node-id"},
		{"Simple", "simple"},
		{"with_underscores", "with-underscores"},
		{"camelCase", "camel-case"},
		{"already-kebab", "already-kebab"},
		{"with.dots", "with-dots"},
		{"version2Beta", "version2-beta"},
		{"SHA256Hash", "sha256-hash"},
		{"MD5Checksum", "md5-checksum"},
		{"__init__", "init"},
		{"--foo--bar--", "foo-bar"},
		{"..path..", "path"},
		{"ÜberFluss", "über-fluss"},
		{"café", "café"},
		// Known heuristic limitations (pinned, not aspirational): an acronym
		// followed by a lowercase suffix is split, and a trailing upper-case
		// rune after a digit is not split.
		{"OAuth2Client", "o-auth2-client"},
		{"IPv4", "i-pv4"},
		{"version2B", "version2b"},
		{"Step1A", "step1a"},
		{"", ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.input, func(t *testing.T) {
			t.Parallel()

			got := casing.Kebab(testCase.input)

			assert.Equal(t, testCase.expected, got)
		})
	}
}

func TestSnake(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected string
	}{
		{"ServerPort", "server_port"},
		{"APIKey", "api_key"},
		{"NodeID", "node_id"},
		{"camelCase", "camel_case"},
		{"already-kebab", "already_kebab"},
		{"with_underscores", "with_underscores"},
		{"with.dots", "with_dots"},
		{"version2Beta", "version2_beta"},
		{"SHA256Hash", "sha256_hash"},
		{"MD5Checksum", "md5_checksum"},
		{"__init__", "init"},
		{"--foo--bar--", "foo_bar"},
		{"..path..", "path"},
		{"ÜberFluss", "über_fluss"},
		{"café", "café"},
		// Known heuristic limitations (pinned, not aspirational).
		{"OAuth2Client", "o_auth2_client"},
		{"IPv4", "i_pv4"},
		{"version2B", "version2b"},
		{"Step1A", "step1a"},
		{"", ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.input, func(t *testing.T) {
			t.Parallel()

			got := casing.Snake(testCase.input)

			assert.Equal(t, testCase.expected, got)
		})
	}
}

func TestUpperSnake(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected string
	}{
		{"server-port", "SERVER_PORT"},
		{"already-kebab", "ALREADY_KEBAB"},
		{"with.dots", "WITH_DOTS"},
		{"with_underscores", "WITH_UNDERSCORES"},
		{"APIKey", "API_KEY"},
		{"NodeID", "NODE_ID"},
		{"SHA256Hash", "SHA256_HASH"},
		{"MD5Checksum", "MD5_CHECKSUM"},
		{"__init__", "INIT"},
		{"--foo--bar--", "FOO_BAR"},
		{"..path..", "PATH"},
		{"ÜberFluss", "ÜBER_FLUSS"},
		{"café", "CAFÉ"},
		{"İstanbul", "İSTANBUL"},
		{"", ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.input, func(t *testing.T) {
			t.Parallel()

			got := casing.UpperSnake(testCase.input)

			assert.Equal(t, testCase.expected, got)
		})
	}
}

func TestUppercaseWords(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected string
	}{
		{"HTTPMethod", "HTTP-METHOD"},
		{"ExistingFile", "EXISTING-FILE"},
		{"ServerPort", "SERVER-PORT"},
		{"APIKey", "API-KEY"},
		{"NodeID", "NODE-ID"},
		{"version2Beta", "VERSION2-BETA"},
		{"SHA256Hash", "SHA256-HASH"},
		{"MD5Checksum", "MD5-CHECKSUM"},
		{"ÜberFluss", "ÜBER-FLUSS"},
		{"café", "CAFÉ"},
		{"İstanbul", "İSTANBUL"},
		// Known heuristic limitations (pinned, not aspirational).
		{"OAuth2Client", "O-AUTH2-CLIENT"},
		{"IPv4", "I-PV4"},
		{"version2B", "VERSION2B"},
		{"Step1A", "STEP1A"},
		{"", ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.input, func(t *testing.T) {
			t.Parallel()

			got := casing.UppercaseWords(testCase.input)

			assert.Equal(t, testCase.expected, got)
		})
	}
}
