package conga_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/internal/assertx"
)

type keyCaseDeployCmd struct {
	noopRunner

	MaxConns conga.OptionalFlag[int] `safe:"true"`
}

type keyCaseCLI struct {
	noopRunner

	APIEndpoint conga.OptionalFlag[string] `safe:"true"`
	APIURL      conga.OptionalFlag[string] `config:"api-url" name:"api-url" safe:"true"`

	DeployApp conga.Command[keyCaseDeployCmd]
}

func TestConfigKeyCase_SnakeCaseBindsDerivedKeys(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFile(t, `{"api_endpoint": "https://a.example", "api-url": "https://b.example", "deploy_app": {"max_conns": 5}}`)

	cli := new(keyCaseCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath), conga.ConfigKeyCase(conga.SnakeCase)).
		Args([]string{"deploy-app"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "https://a.example", cli.APIEndpoint.Get())
	assert.Equal(t, "https://b.example", cli.APIURL.Get())
	assert.Equal(t, 5, cli.DeployApp.Get().MaxConns.Get())
}

func TestConfigKeyCase_CamelCaseRejectsKebabKey(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFile(t, `{"api-endpoint": "https://a.example"}`)

	cli := new(keyCaseCLI)
	app := conga.New(cli).
		Name("app").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath), conga.ConfigKeyCase(conga.CamelCase)).
		Args([]string{})

	err := app.Run()

	assert.IsError(t, err, conga.ErrConfig)
	assert.Equal(t, fmt.Sprintf(`config file %q: unknown field "api-endpoint" in command "app" (did you mean "apiEndpoint"?)`, cfgPath), err.Error())
}

func TestConfigKeyCase_CoercionErrorNamesCasedKey(t *testing.T) {
	t.Parallel()

	cfgPath := writeConfigFile(t, `{"deploy_app": {"max_conns": "many"}}`)

	cli := new(keyCaseCLI)
	app := conga.New(cli).
		Name("app").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath), conga.ConfigKeyCase(conga.SnakeCase)).
		Args([]string{"deploy-app"})

	err := app.Run()

	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, `invalid integer value "many" for flag --max-conns (from config key "deploy_app.max_conns")`, err.Error())
}

func TestConfigKeyCase_WhyShowsCasedConfigPath(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	cli := new(keyCaseCLI)
	app := conga.New(cli).
		Name("app").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{}`)), conga.ConfigKeyCase(conga.SnakeCase)).
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"deploy-app", "--why"})

	err := app.Run()

	assert.NoError(t, err)

	rows := map[string]string{}

	for line := range strings.SplitSeq(buffer.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && strings.HasPrefix(fields[0], "--") {
			rows[fields[0]] = fields[len(fields)-1]
		}
	}

	assert.Equal(t, "deploy_app.max_conns", rows["--max-conns"])
	assert.Equal(t, "api_endpoint", rows["--api-endpoint"])
	assert.Equal(t, "api-url", rows["--api-url"])
}

func TestConfigKeyCase_UnknownKeyCasePanics(t *testing.T) {
	t.Parallel()

	assertx.PanicsWithValue(t, "conga: app.ConfigOptions(conga.ConfigKeyCase(...)): unknown key case", func() {
		conga.ConfigKeyCase(conga.KeyCase(9))
	})
}
