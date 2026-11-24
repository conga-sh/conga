package conga_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os/user"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/internal/assertx"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
	"github.com/conga-sh/conga/timezone"
)

// level is a named int with its own word grammar, so a registered parser and
// plain integer parsing give different results.
type level int

func parseLevel(text string) (level, error) {
	switch text {
	case "debug":
		return 0, nil
	case "info":
		return 1, nil
	default:
		return 0, fmt.Errorf("unknown level %q", text)
	}
}

type levelMapCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile                     `help:"Config file"`
	Levels     conga.OptionalFlag[map[string]level] `env:"CONGA_TEST_LEVELS" help:"Per-module levels"`
}

type levelMapArgCLI struct {
	noopRunner

	Levels conga.OptionalArg[map[string]level] `help:"Per-module levels"`
}

type routeMapCLI struct {
	noopRunner

	Routes conga.OptionalFlag[map[string]net.IPNet] `help:"Routes"`
}

func TestTypedMap_RegisteredValueParser(t *testing.T) {
	t.Parallel()

	t.Run("command-line value goes through the registered parser", func(t *testing.T) {
		t.Parallel()

		cli := new(levelMapCLI)
		app := conga.New(cli).
			RegisterType(parseLevel).
			Args([]string{"--levels", "db=info,api=debug"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[string]level{"db": 1, "api": 0}, cli.Levels.Get())
	})

	t.Run("integer text is rejected by the registered parser", func(t *testing.T) {
		t.Parallel()

		app := conga.New[levelMapCLI]().
			RegisterType(parseLevel).
			Args([]string{"--levels", "db=1"})

		err := app.Run()

		assertUsageError(t, err)
		assert.Equal(t, `value for key "db": unknown level "1" for flag --levels`, err.Error())
	})

	t.Run("config value goes through the registered parser", func(t *testing.T) {
		t.Parallel()

		configPath := writeConfigFile(t, `{"levels": {"db": "info"}}`)
		cli := new(levelMapCLI)
		app := conga.New(cli).
			RegisterDecoder(".json", json.Decode).
			RegisterType(parseLevel).
			Args([]string{"--config-file", configPath})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[string]level{"db": 1}, cli.Levels.Get())
		assert.Equal(t, conga.SourceConfig, cli.Levels.Source())
	})

	t.Run("positional map goes through the registered parser", func(t *testing.T) {
		t.Parallel()

		cli := new(levelMapArgCLI)
		app := conga.New(cli).
			RegisterType(parseLevel).
			Args([]string{"db=info"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[string]level{"db": 1}, cli.Levels.Get())
	})

	t.Run("a parser for *T covers a struct map value T", func(t *testing.T) {
		t.Parallel()

		cli := new(routeMapCLI)
		app := conga.New(cli).
			RegisterType(parseCIDR).
			Args([]string{"--routes", "private=10.0.0.0/8"})

		err := app.Run()

		route := cli.Routes.Get()["private"]

		assert.NoError(t, err)
		assert.Equal(t, "10.0.0.0/8", route.String())
	})
}

func TestTypedMap_RegisteredValueParserFromEnv(t *testing.T) {
	t.Setenv("CONGA_TEST_LEVELS", "db=info")

	cli := new(levelMapCLI)
	app := conga.New(cli).
		RegisterType(parseLevel).
		Args(nil)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]level{"db": 1}, cli.Levels.Get())
	assert.Equal(t, conga.SourceEnv, cli.Levels.Source())
}

// streamValue records the standard input its decode context carried.
type streamValue struct {
	stdin io.Reader
}

func (v *streamValue) UnmarshalTextWithContext(_ []byte, decodeContext conga.DecodeContext) error {
	v.stdin = decodeContext.Stdin

	return nil
}

type streamMapCLI struct {
	noopRunner

	Values conga.OptionalFlag[map[string]streamValue] `help:"Values"`
}

func TestTypedMap_ValuesReceiveDecodeContext(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("")
	cli := new(streamMapCLI)
	app := conga.New(cli).
		Stdin(stdin).
		Args([]string{"--values", "a=x"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal[io.Reader](t, stdin, cli.Values.Get()["a"].stdin)
}

type ipCLI struct {
	noopRunner

	IP    conga.OptionalFlag[net.IP]   `help:"Address"`
	Peers conga.OptionalFlag[[]net.IP] `help:"Peer addresses"`
}

func TestSelfParsingSliceType(t *testing.T) {
	t.Parallel()

	t.Run("net.IP binds as one value", func(t *testing.T) {
		t.Parallel()

		cli := new(ipCLI)
		app := conga.New(cli).
			Args([]string{"--ip", "10.0.0.1"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "10.0.0.1", cli.IP.Get().String())
	})

	t.Run("[]net.IP binds as a list of addresses", func(t *testing.T) {
		t.Parallel()

		cli := new(ipCLI)
		app := conga.New(cli).
			Args([]string{"--peers", "10.0.0.1,10.0.0.2"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, []string{cli.Peers.Get()[0].String(), cli.Peers.Get()[1].String()})
	})

	t.Run("help shows one value for net.IP", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[ipCLI]().
			Name("ipcli").
			Stdout(&buffer).
			Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--ip=IP ")
		assert.Contains(t, buffer.String(), "--peers=IP,...")
		assert.NotContains(t, buffer.String(), "UINT8")
	})
}

type locationCLI struct {
	noopRunner

	Zone conga.OptionalFlag[*time.Location] `help:"Zone"`
}

type networkCLI struct {
	noopRunner

	Allow conga.OptionalFlag[net.IPNet] `help:"Allowed network"`
}

type locationSliceCLI struct {
	noopRunner

	Zones conga.OptionalFlag[[]*time.Location] `help:"Zones"`
}

type networkMapCLI struct {
	noopRunner

	Routes conga.OptionalFlag[map[string]net.IPNet] `help:"Routes"`
}

type nestedSliceMapCLI struct {
	noopRunner

	Groups conga.OptionalFlag[map[string][]string] `help:"Groups"`
}

type channelArgCLI struct {
	noopRunner

	Channel conga.RequiredArg[chan int] `help:"Channel"`
}

type hiddenUserCLI struct {
	noopRunner

	As conga.OptionalFlag[*user.User] `help:"User" hidden:"true"`
}

type gatedGroupFeatures struct {
	Admin bool `help:"Admin tools"`
}

type gatedGroupCLI struct {
	noopRunner

	Features conga.Features[gatedGroupFeatures] `flag:"feature-gates"`
	Group    conga.OptionalFlag[*user.Group]    `feature:"admin"      help:"Group"`
}

type interfaceCLI struct {
	noopRunner

	Via conga.OptionalFlag[*net.Interface] `help:"Interface"`
}

func TestSchema_UnsupportedValueType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		run      func() error
		expected string
	}{
		{
			name:     "standard library type with a kit binding",
			run:      func() error { return conga.New[locationCLI]().Args(nil).Run() },
			expected: "struct locationCLI field Zone: unsupported type *time.Location (hint: register a parser with app.Use(timezone.RegisterLocation))",
		},
		{
			name:     "type without a kit binding",
			run:      func() error { return conga.New[networkCLI]().Args(nil).Run() },
			expected: "struct networkCLI field Allow: unsupported type net.IPNet (hint: register a parser with app.RegisterType)",
		},
		{
			name:     "slice element",
			run:      func() error { return conga.New[locationSliceCLI]().Args(nil).Run() },
			expected: "struct locationSliceCLI field Zones: unsupported element type *time.Location (hint: register a parser with app.Use(timezone.RegisterLocation))",
		},
		{
			name:     "map value",
			run:      func() error { return conga.New[networkMapCLI]().Args(nil).Run() },
			expected: "struct networkMapCLI field Routes: unsupported map value type net.IPNet (hint: register a parser with app.RegisterType)",
		},
		{
			name:     "nested collection",
			run:      func() error { return conga.New[nestedSliceMapCLI]().Args(nil).Run() },
			expected: "struct nestedSliceMapCLI field Groups: unsupported map value type []string (hint: give the slice a named type that implements encoding.TextUnmarshaler)",
		},
		{
			name:     "positional argument",
			run:      func() error { return conga.New[channelArgCLI]().Args(nil).Run() },
			expected: "struct channelArgCLI field Channel: unsupported type chan int (hint: register a parser with app.RegisterType)",
		},
		{
			name:     "hidden flag",
			run:      func() error { return conga.New[hiddenUserCLI]().Args(nil).Run() },
			expected: "struct hiddenUserCLI field As: unsupported type *user.User (hint: register a parser with app.Use(system.RegisterUser))",
		},
		{
			name:     "feature-gated flag",
			run:      func() error { return conga.New[gatedGroupCLI]().EnableFeatures().Args(nil).Run() },
			expected: "struct gatedGroupCLI field Group: unsupported type *user.Group (hint: register a parser with app.Use(system.RegisterGroup))",
		},
		{
			name:     "network interface",
			run:      func() error { return conga.New[interfaceCLI]().Args(nil).Run() },
			expected: "struct interfaceCLI field Via: unsupported type *net.Interface (hint: register a parser with app.Use(network.RegisterInterface))",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.run()

			assert.Error(t, err)
			assert.Equal(t, testCase.expected, err.Error())
		})
	}
}

type stringMapsCLI struct {
	noopRunner

	Labels conga.OptionalFlag[map[string]string] `help:"Labels"`
	Names  conga.OptionalFlag[namedStringMap]    `help:"Names"`
}

// mapKey and mapValue are named string types without a parser of their own.
type (
	mapKey   string
	mapValue string
)

type namedStringKindMapsCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile                        `help:"Config file"`
	Keys       conga.OptionalFlag[map[mapKey]string]   `help:"Keys"`
	Values     conga.OptionalFlag[map[string]mapValue] `help:"Values"`
	Both       conga.OptionalFlag[map[mapKey]mapValue] `default:"d=1"      help:"Both"`
}

type namedStringKeyArgCLI struct {
	noopRunner

	Pairs conga.RequiredArg[map[mapKey]string] `help:"Pairs"`
}

type signalCLI struct {
	noopRunner

	Signal conga.OptionalFlag[syscall.Signal] `help:"Signal"`
}

type pointerRegisteredCLI struct {
	noopRunner

	Level conga.OptionalFlag[*level] `help:"Level"`
}

func TestSchema_SupportedValueTypes(t *testing.T) {
	t.Parallel()

	t.Run("string maps need no registration", func(t *testing.T) {
		t.Parallel()

		cli := new(stringMapsCLI)
		app := conga.New(cli).
			Args([]string{"--labels", "a=1", "--names", "b=2"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"a": "1"}, cli.Labels.Get())
		assert.Equal(t, namedStringMap{"b": "2"}, cli.Names.Get())
	})

	t.Run("maps keyed or valued by named string types bind from the command line", func(t *testing.T) {
		t.Parallel()

		cli := new(namedStringKindMapsCLI)
		app := conga.New(cli).
			Args([]string{"--keys", "a=1", "--values", "b=2", "--both", "c=3"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[mapKey]string{"a": "1"}, cli.Keys.Get())
		assert.Equal(t, map[string]mapValue{"b": "2"}, cli.Values.Get())
		assert.Equal(t, map[mapKey]mapValue{"d": "1", "c": "3"}, cli.Both.Get())
	})

	t.Run("maps keyed or valued by named string types bind from config", func(t *testing.T) {
		t.Parallel()

		configPath := writeConfigFile(t, `{"keys": {"a": "1"}, "values": {"b": "2"}, "both": {"c": "3"}}`)
		cli := new(namedStringKindMapsCLI)
		app := conga.New(cli).
			RegisterDecoder(".json", json.Decode).
			Args([]string{"--config-file", configPath})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[mapKey]string{"a": "1"}, cli.Keys.Get())
		assert.Equal(t, map[string]mapValue{"b": "2"}, cli.Values.Get())
		assert.Equal(t, map[mapKey]mapValue{"d": "1", "c": "3"}, cli.Both.Get())
	})

	t.Run("a map positional keyed by a named string type binds", func(t *testing.T) {
		t.Parallel()

		cli := new(namedStringKeyArgCLI)
		app := conga.New(cli).
			Args([]string{"a=1"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[mapKey]string{"a": "1"}, cli.Pairs.Get())
	})

	t.Run("integer-kind standard library type needs no registration", func(t *testing.T) {
		t.Parallel()

		cli := new(signalCLI)
		app := conga.New(cli).
			Args([]string{"--signal", "15"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, syscall.SIGTERM, cli.Signal.Get())
	})

	t.Run("a parser for T covers a *T field", func(t *testing.T) {
		t.Parallel()

		cli := new(pointerRegisteredCLI)
		app := conga.New(cli).
			RegisterType(parseLevel).
			Args([]string{"--level", "info"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, level(1), *cli.Level.Get())
	})

	t.Run("a parser for *T covers a T field", func(t *testing.T) {
		t.Parallel()

		cli := new(levelMapCLI)
		app := conga.New(cli).
			RegisterType(func(text string) (*level, error) {
				parsed, err := parseLevel(text)

				return &parsed, err
			}).
			Args([]string{"--levels", "db=info"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, map[string]level{"db": 1}, cli.Levels.Get())
	})
}

func TestRegisterType_CollectionPanics(t *testing.T) {
	t.Parallel()

	t.Run("map type", func(t *testing.T) {
		t.Parallel()

		assertx.PanicsWithValue(t, "conga: RegisterType does not accept map type map[string]int; maps bind entry by entry, so register the key or value type instead", func() {
			conga.New[noopRunner]().RegisterType(func(string) (map[string]int, error) { return nil, errors.New("unused") })
		})
	})

	t.Run("slice type", func(t *testing.T) {
		t.Parallel()

		assertx.PanicsWithValue(t, "conga: RegisterType does not accept slice type []string; slices bind element by element, so register the element type instead", func() {
			conga.New[noopRunner]().RegisterType(func(string) ([]string, error) { return nil, errors.New("unused") })
		})
	})

	t.Run("slice type with context-aware unmarshaling", func(t *testing.T) {
		t.Parallel()

		assertx.PanicsWithValue(t, "conga: RegisterType does not accept conga_test.contextHosts; it implements conga.ContextAwareTextUnmarshaler, so it binds as one value without registration", func() {
			conga.New[noopRunner]().RegisterType(func(string) (contextHosts, error) { return nil, errors.New("unused") })
		})
	})

	t.Run("slice type that parses itself", func(t *testing.T) {
		t.Parallel()

		assertx.PanicsWithValue(t, "conga: RegisterType does not accept net.IP; it implements encoding.TextUnmarshaler, so it binds as one value without registration", func() {
			conga.New[noopRunner]().RegisterType(func(string) (net.IP, error) { return nil, errors.New("unused") })
		})
	})
}

// tier has its own predictor. A map of tiers completes it after "key=".
type tier string

func (t *tier) UnmarshalText(text []byte) error {
	*t = tier(text)

	return nil
}

func (tier) Predict() predict.Predictor { return predict.Values("web", "db") }

type tierMapCLI struct {
	noopRunner

	Tiers conga.OptionalFlag[map[string]tier] `help:"Tier per service"`
}

func TestComplete_TypedMapValueCompletesAfterKey(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[tierMapCLI]().
		EnableCompletion().
		Stdout(&buffer).
		Args([]string{"__complete", "powershell", "--tiers", "api="})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t,
		`{"candidates":[{"value":"api=web ","display":"web","description":" ","tag":"values"},{"value":"api=db ","display":"db","description":" ","tag":"values"}],"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":"Tier per service"}`,
		buffer.String())
}

func TestComplete_TypedMapValueOffersNothingBeforeEquals(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[tierMapCLI]().
		EnableCompletion().
		Stdout(&buffer).
		Args([]string{"__complete", "powershell", "--tiers", "ap"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t,
		`{"candidates":[],"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":"Tier per service"}`,
		buffer.String())
}

type locationMapCLI struct {
	noopRunner

	Zones conga.OptionalFlag[map[string]*time.Location]
}

func TestComplete_MapOfLocationsCompletesTimeZones(t *testing.T) {
	t.Parallel()

	app := conga.New[locationMapCLI]().
		Use(timezone.RegisterLocation).
		EnableCompletion()

	result, _ := app.CompleteForShell("bash", []string{"--zones", "eu=Europe/Lo"})

	assert.Equal(t, []string{"eu=Europe/London"}, slicex.Map(result.Items, func(item predict.Item) string { return item.Value }))
}

type fileMapCLI struct {
	noopRunner

	Inputs conga.OptionalFlag[map[string]conga.File]
}

func TestComplete_MapOfFilesNamesFileHelpInHint(t *testing.T) {
	t.Parallel()

	app := conga.New[fileMapCLI]().EnableCompletion()

	result, _ := app.CompleteForShell("bash", []string{"--inputs", ""})

	assert.Equal(t, 0, len(result.Items))
	assert.Equal(t, "<key>=<Path to a file>", result.Usage)
}

// lowerKey lowercases its text, so the decoded key differs from the typed one.
type lowerKey string

func (k *lowerKey) UnmarshalText(text []byte) error {
	*k = lowerKey(strings.ToLower(string(text)))

	return nil
}

type lowerKeyMapCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile                     `help:"Config file"`
	Ports      conga.OptionalFlag[map[lowerKey]int] `help:"Ports"       safe:"true"`
}

func TestWhy_TypedMapKeysKeepTheirSource(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	configPath := writeConfigFile(t, `{"ports": {" Web ": 80}}`)
	app := conga.New[lowerKeyMapCLI]().
		Name("ports").
		RegisterDecoder(".json", json.Decode).
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--config-file", configPath, "--ports", "API=1", "--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "├─ api=1    cli\n")
	assert.Contains(t, buffer.String(), "└─ web=80   config (config.json)\n")
}

// hostList is a slice-kind type with its own grammar and a pointer-receiver
// String, so --why must reach String through an addressable copy.
type hostList []string

func (h *hostList) UnmarshalText(text []byte) error {
	*h = strings.Split(string(text), "+")

	return nil
}

func (h *hostList) String() string { return strings.Join(*h, "+") }

type hostListCLI struct {
	noopRunner

	Hosts conga.OptionalFlag[hostList] `help:"Hosts" safe:"true"`
}

func TestWhy_SelfParsingSliceUsesItsString(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[hostListCLI]().
		Name("hosts").
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--hosts", "a+b", "--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "--hosts   a+b     cli")
}

// contextHosts is a slice-kind type that decodes itself through
// ContextAwareTextUnmarshaler only.
type contextHosts []string

func (h *contextHosts) UnmarshalTextWithContext(text []byte, _ conga.DecodeContext) error {
	*h = strings.Split(string(text), "+")

	return nil
}
