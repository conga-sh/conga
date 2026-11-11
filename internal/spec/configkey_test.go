package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

func TestConfigKeyPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		keyCase  spec.KeyCase
		flag     *spec.Flag
		nested   bool
		expected string
	}{
		{"root flag, kebab", spec.KeyCaseKebab, &spec.Flag{Name: "api-endpoint"}, false, "api-endpoint"},
		{"root flag, snake", spec.KeyCaseSnake, &spec.Flag{Name: "api-endpoint"}, false, "api_endpoint"},
		{"root flag, camel", spec.KeyCaseCamel, &spec.Flag{Name: "api-endpoint"}, false, "apiEndpoint"},
		{"tag as written", spec.KeyCaseSnake, &spec.Flag{Name: "api-endpoint", ConfigKey: "db.host-name"}, false, "db.host-name"},
		{"subcommand flag, snake", spec.KeyCaseSnake, &spec.Flag{Name: "max-conns"}, true, "deploy_app.max_conns"},
		{"subcommand tag", spec.KeyCaseSnake, &spec.Flag{Name: "max-conns", ConfigKey: "pool.size"}, true, "deploy_app.pool.size"},
		{"subcommand tag naming its command", spec.KeyCaseKebab, &spec.Flag{Name: "max-conns", ConfigKey: "deploy-app.size"}, true, "deploy-app.size"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			chain := []*spec.Command{{Name: "app", ConfigSettings: spec.ConfigSettings{KeyCase: testCase.keyCase}}}
			if testCase.nested {
				chain = append(chain, &spec.Command{Name: "deploy-app"})
			}

			assert.Equal(t, testCase.expected, spec.ConfigKeyPath(testCase.flag, chain))
		})
	}
}
