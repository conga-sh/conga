package network

import (
	"bytes"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"
)

//go:embed testdata/fixtures/hosts
var fixtureHosts []byte

func TestParseHostsReader(t *testing.T) {
	t.Parallel()

	items := parseHostsReader(bytes.NewReader(fixtureHosts))

	byValue := map[string][]string{}
	for _, item := range items {
		byValue[item.Value] = append(byValue[item.Value], item.Description)
	}

	assert.Equal(t, 10, len(items))
	assert.Equal(t, map[string][]string{
		"localhost":             {"127.0.0.1", "::1"},
		"localhost.localdomain": {"127.0.0.1"},
		"ip6-localhost":         {"::1"},
		"ip6-loopback":          {"::1"},
		"devbox.local":          {"192.168.1.100"},
		"devbox":                {"192.168.1.100"},
		"db-master.prod":        {"10.0.0.50"},
		"db-replica.prod":       {"10.0.0.51"},
		"db-slave":              {"10.0.0.51"},
	}, byValue)

	assert.Equal(t, "hosts", items[0].Tag)
	assert.Equal(t, "blue", items[0].Style)
}
