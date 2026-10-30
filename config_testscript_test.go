package conga_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
)

type TestAppCLI struct {
	ConfigFile  conga.ConfigFile           `help:"Config file path"`
	Port        conga.OptionalFlag[int]    `default:"8080"          env:"CONGA_TEST_APP_PORT" name:"port" short:"p"`
	Host        conga.OptionalFlag[string] `default:"127.0.0.1"     env:"CONGA_TEST_APP_HOST" name:"host" short:"h"`
	Environment conga.OptionalFlag[string] `default:"dev"           name:"env"`
	Debug       conga.OptionalFlag[bool]   `name:"debug"            short:"d"`
	Server      conga.Command[ServerCmd]   `name:"server"`
}

type ServerCmd struct {
	WorkerCount conga.OptionalFlag[int]    `default:"4"         name:"workers" short:"w"`
	Zone        conga.OptionalFlag[string] `default:"us-east-1" name:"zone"`
}

func (s *ServerCmd) Run(_ context.Context, _ *conga.Session) error {
	fmt.Printf("server running: workers=%d zone=%s\n", s.WorkerCount.Get(), s.Zone.Get())

	return nil
}

func (a *TestAppCLI) Run(_ context.Context, _ *conga.Session) error {
	fmt.Printf("app running: port=%d host=%s env=%s debug=%t (config=%s source=%s)\n",
		a.Port.Get(), a.Host.Get(), a.Environment.Get(), a.Debug.Get(), a.ConfigFile.Path(), a.ConfigFile.Source())

	return nil
}

func mainTestApp() {
	app := conga.New[TestAppCLI]().
		Name("test-app").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".json", json.Decode)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}

func TestMain(m *testing.M) {
	// Keep golden help screens at the engine default width: an exported
	// COLUMNS would otherwise wrap every fixture at the developer's terminal
	// width.
	os.Unsetenv("COLUMNS")

	testscript.Main(m, map[string]func(){
		"test-app": mainTestApp,
	})
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir: "testdata/config",
	})
}
