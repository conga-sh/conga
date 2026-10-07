package conga_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
)

type noopRunner struct{}

func (noopRunner) Run(_ context.Context, _ *conga.Session) error { return nil }

// writeConfigFile writes a JSON configuration payload to a temporary file and
// returns its path, standing in for the removed in-memory config map.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	assert.NoError(t, os.WriteFile(cfgPath, []byte(content), 0o600))

	return cfgPath
}

type FullServerCLI struct {
	noopRunner

	Port     conga.OptionalFlag[int]           `config:"server.port" default:"8080"         env:"PORT"    name:"port" short:"p"`
	Timeout  conga.OptionalFlag[time.Duration] `default:"30s"        name:"timeout"`
	Verbose  conga.OptionalFlag[bool]          `name:"verbose"       short:"v"`
	Format   conga.OptionalFlag[string]        `default:"json"       enum:"json,yaml,table" name:"format"`
	TCP      conga.OptionalFlag[bool]          `name:"tcp"           xor:"transport"`
	UDP      conga.OptionalFlag[bool]          `name:"udp"           xor:"transport"`
	User     conga.OptionalFlag[string]        `and:"auth"           name:"user"`
	Password conga.OptionalFlag[string]        `and:"auth"           env:"RT_PASS"          name:"pass"`
	Tags     conga.OptionalFlag[[]string]      `name:"tag"           short:"t"`
	Required conga.RequiredFlag[string]        `name:"api-key"`
	Source   conga.RequiredArg[string]         `name:"<source>"`
	Outputs  conga.OptionalArg[string]         `name:"[output]"`
}

func TestBinder_4TierResolution_CLIFlags(t *testing.T) {
	t.Parallel()

	cli := new(FullServerCLI)
	app := conga.New(cli).
		Name("server").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"server": {"port": "9090"}}`))).
		Args([]string{"--api-key", "secret123", "-p", "7070", "-v", "input.txt", "out.txt"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 7070, cli.Port.Get())
	assert.Equal(t, conga.SourceCLI, cli.Port.Source())
	assert.True(t, cli.Verbose.Get())
	assert.Equal(t, conga.SourceCLI, cli.Verbose.Source())
	assert.Equal(t, "input.txt", cli.Source.Get())
	assert.Equal(t, "out.txt", cli.Outputs.Get())
	assert.Equal(t, "json", cli.Format.Get())
	assert.Equal(t, conga.SourceDefault, cli.Format.Source())
}

func TestBinder_4TierResolution_EnvVar(t *testing.T) {
	t.Setenv("PORT", "5050")

	cli := new(FullServerCLI)
	app := conga.New(cli).
		Name("server").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"server": {"port": "9090"}}`))).
		Args([]string{"--api-key", "secret", "file.txt"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 5050, cli.Port.Get())
	assert.Equal(t, conga.SourceEnv, cli.Port.Source())
}

func TestBinder_4TierResolution_ConfigFile(t *testing.T) {
	t.Parallel()

	cli := new(FullServerCLI)
	app := conga.New(cli).
		Name("server").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"server": {"port": "9090"}}`))).
		Args([]string{"--api-key", "secret", "file.txt"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 9090, cli.Port.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
}

func TestBinder_Constraints(t *testing.T) {
	t.Parallel()

	t.Run("missing required flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().
			Name("server").
			Args([]string{"file.txt"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "missing required flag: --api-key", err.Error())
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("XOR mutually exclusive", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().
			Name("server").
			Args([]string{"--api-key", "key", "--tcp", "--udp", "file.txt"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("and group violation", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().
			Name("server").
			Args([]string{"--api-key", "key", "--user", "admin", "file.txt"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("invalid enum", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().
			Name("server").
			Args([]string{"--api-key", "key", "--format", "invalid", "file.txt"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, `invalid value "invalid" (must be one of: json, yaml, table) for flag --format`, err.Error())
	})
}

// Subcommands & Lifecycle Hooks.
type RootApp struct {
	Verbose  conga.OptionalFlag[bool]  `short:"v"`
	Cluster  conga.Command[ClusterApp] `summary:"Cluster commands"`
	hookLogs []string
}

func (r *RootApp) PreRun(ctx context.Context, session *conga.Session) error {
	r.hookLogs = append(r.hookLogs, "Root.PreRun")

	return nil
}

type ClusterApp struct {
	Node conga.Command[NodeApp] `summary:"Node commands"`
}

type NodeApp struct {
	Drain conga.Command[DrainApp] `summary:"Drain node"`
}

type DrainApp struct {
	NodeID conga.RequiredArg[string] `name:"node-id"`
	Force  conga.OptionalFlag[bool]  `short:"f"`
	ran    bool
}

func (d *DrainApp) Run(ctx context.Context, session *conga.Session) error {
	d.ran = true

	return nil
}

func TestSubcommandRoutingAndLifecycle(t *testing.T) {
	t.Parallel()

	cli := new(RootApp)
	app := conga.New(cli).
		Name("myapp").
		Args([]string{"cluster", "node", "drain", "worker-1", "-f"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, []string{"Root.PreRun"}, cli.hookLogs)
	assert.True(t, cli.Cluster.IsSelected())
	assert.True(t, cli.Cluster.Get().Node.IsSelected())

	drain := cli.Cluster.Get().Node.Get().Drain
	assert.True(t, drain.IsSelected())
	assert.Equal(t, "worker-1", drain.Get().NodeID.Get())
	assert.True(t, drain.Get().Force.Get())
	assert.True(t, drain.Get().ran)
}

func TestDashArgs(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Target  conga.RequiredArg[string] `name:"<target>"`
		Verbose conga.OptionalFlag[bool]  `short:"v"`
		Extra   conga.DashArgs
	}

	t.Run("success with explicit double dash", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("test").
			Args([]string{"./pkg/...", "-v", "--", "-run", "TestFoo", "-count=1"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "./pkg/...", cli.Target.Get())
		assert.True(t, cli.Verbose.Get())
		assert.True(t, cli.Extra.IsSet())
		assert.Equal(t, 3, cli.Extra.Len())
		assert.Equal(t, "-run TestFoo -count=1", cli.Extra.String())
	})

	t.Run("failure with hint when unknown flag passed without double dash", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CLI]().
			Name("test").
			Args([]string{"./pkg/...", "--run", "TestFoo"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})
}

func TestRawArgs(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		User    conga.OptionalFlag[string] `default:"root" short:"u"`
		Command conga.RawArgs
	}

	cli := new(CLI)
	app := conga.New(cli).
		Name("sudo").
		Args([]string{"-u", "postgres", "psql", "-h", "localhost", "-U", "postgres"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "postgres", cli.User.Get())
	assert.True(t, cli.Command.IsSet())
	assert.Equal(t, 5, cli.Command.Len())
	assert.Equal(t, "psql -h localhost -U postgres", cli.Command.String())
}

func TestAppConfigFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	cfgJSON := `{"server": {"port": 9999, "timeout": "10s"}}`
	err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644)
	assert.NoError(t, err)

	type ConfigApp struct {
		noopRunner

		Port    conga.OptionalFlag[int]           `config:"server.port"    name:"port"`
		Timeout conga.OptionalFlag[time.Duration] `config:"server.timeout" name:"timeout"`
	}

	cli := new(ConfigApp)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9999, cli.Port.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
	assert.Equal(t, 10*time.Second, cli.Timeout.Get())
	assert.Equal(t, conga.SourceConfig, cli.Timeout.Source())
}

func TestBinder_MapFlag(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Tags    conga.OptionalFlag[map[string]string] `help:"Tags map"     short:"t"`
		GenTags conga.OptionalFlag[map[string]string] `help:"Generic tags" short:"g"`
	}

	t.Run("map multiple flag occurrences", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("mapapp").
			Args([]string{"-t", "category=work", "-t", "priority=high"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Tags.IsSet())
		assert.Equal(t, 2, len(cli.Tags.Get()))
		assert.Equal(t, "work", cli.Tags.Get()["category"])
		assert.Equal(t, "high", cli.Tags.Get()["priority"])
		assert.Equal(t, conga.SourceCLI, cli.Tags.Source())
	})

	t.Run("map comma separated pairs", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("mapapp").
			Args([]string{"-t", "env=prod,region=us-east"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Tags.IsSet())
		assert.Equal(t, 2, len(cli.Tags.Get()))
		assert.Equal(t, "prod", cli.Tags.Get()["env"])
		assert.Equal(t, "us-east", cli.Tags.Get()["region"])
	})

	t.Run("conga.Flag[map[string]string] support", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("mapapp").
			Args([]string{"-g", "k1=v1", "-g", "k2=v2"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.GenTags.IsSet())
		assert.Equal(t, map[string]string{"k1": "v1", "k2": "v2"}, cli.GenTags.Get())
	})
}

func TestBinder_CounterFlag(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Verbose conga.Counter `help:"Verbosity level" short:"v"`
	}

	t.Run("single short flag -v", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("counterapp").
			Args([]string{"-v"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Verbose.IsSet())
		assert.Equal(t, 1, cli.Verbose.Get())
		assert.Equal(t, conga.SourceCLI, cli.Verbose.Source())
	})

	t.Run("clustered short flags -vvv", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("counterapp").
			Args([]string{"-vvv"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Verbose.IsSet())
		assert.Equal(t, 3, cli.Verbose.Get())
	})

	t.Run("repeated short flags -v -v", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("counterapp").
			Args([]string{"-v", "-v"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Verbose.IsSet())
		assert.Equal(t, 2, cli.Verbose.Get())
	})

	t.Run("repeated long flags --verbose --verbose", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("counterapp").
			Args([]string{"--verbose", "--verbose"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Verbose.IsSet())
		assert.Equal(t, 2, cli.Verbose.Get())
	})

	t.Run("long flag with attached value --verbose=2", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("counterapp").
			Args([]string{"--verbose=2"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Verbose.IsSet())
		assert.Equal(t, 2, cli.Verbose.Get())
		assert.Equal(t, conga.SourceCLI, cli.Verbose.Source())
	})

	t.Run("short flag with attached value -v=2", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("counterapp").
			Args([]string{"-v=2"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Verbose.IsSet())
		assert.Equal(t, 2, cli.Verbose.Get())
		assert.Equal(t, conga.SourceCLI, cli.Verbose.Source())
	})

	t.Run("invalid attached value --verbose=abc returns coercion error", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CLI]().
			Name("counterapp").
			Args([]string{"--verbose=abc"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
	})
}

func TestBinder_CounterEnvSeed(t *testing.T) {
	t.Setenv("VERBOSE_LEVEL", "3")

	type CLI struct {
		noopRunner

		Verbose conga.Counter `env:"VERBOSE_LEVEL" help:"Verbosity level" short:"v"`
	}

	cli := new(CLI)
	app := conga.New(cli).
		Name("counterapp").
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Verbose.IsSet())
	assert.Equal(t, 3, cli.Verbose.Get())
	assert.True(t, cli.Verbose.Changed())
	assert.Equal(t, conga.SourceEnv, cli.Verbose.Source())
}

func TestBinder_CounterCLIOverridesEnv(t *testing.T) {
	t.Setenv("VERBOSE_LEVEL", "3")

	type CLI struct {
		noopRunner

		Verbose conga.Counter `env:"VERBOSE_LEVEL" help:"Verbosity level" short:"v"`
	}

	cli := new(CLI)
	app := conga.New(cli).
		Name("counterapp").
		Args([]string{"-vv"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Verbose.IsSet())
	assert.Equal(t, 2, cli.Verbose.Get())
	assert.Equal(t, conga.SourceCLI, cli.Verbose.Source())
	assert.True(t, cli.Verbose.Changed())
}

func TestBinder_UnknownShortFlag(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Debug conga.OptionalFlag[bool] `help:"Debug mode" short:"d"`
	}

	t.Run("single unknown short flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CLI]().
			Name("unknownapp").
			Args([]string{"-Z"})

		err := app.Run()

		assert.IsError(t, err, conga.ErrParse)
		assert.Equal(t, "unknown flag: -Z", err.Error())
	})

	t.Run("unknown short flag in cluster", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CLI]().
			Name("unknownapp").
			Args([]string{"-dZ"})

		err := app.Run()

		assert.IsError(t, err, conga.ErrParse)
		assert.Equal(t, "unknown shorthand flag: 'Z' in -dZ", err.Error())
	})

	t.Run("unknown short flag leading cluster", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CLI]().
			Name("unknownapp").
			Args([]string{"-Zd"})

		err := app.Run()

		assert.IsError(t, err, conga.ErrParse)
		assert.Equal(t, "unknown shorthand flag: 'Z' in -Zd", err.Error())
	})
}

func TestBinder_SliceSepNone(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Headers conga.OptionalFlag[[]string] `help:"Headers" name:"header" sep:"none" short:"H"`
	}

	cli := new(CLI)
	app := conga.New(cli).
		Name("sliceapp").
		Args([]string{
			"-H", "Accept: text/html,application/xhtml+xml",
			"-H", "Authorization: Bearer token123",
		})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Headers.IsSet())
	assert.Equal(t, []string{
		"Accept: text/html,application/xhtml+xml",
		"Authorization: Bearer token123",
	}, cli.Headers.Get())
}

func TestBinder_NegatableBool(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Location conga.OptionalFlag[bool] `help:"Follow redirects" negatable:"true" short:"L"`
	}

	t.Run("positive long flag --location", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("negapp").
			Args([]string{"--location"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Location.IsSet())
		assert.True(t, cli.Location.Get())
	})

	t.Run("negative long flag --no-location", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("negapp").
			Args([]string{"--no-location"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Location.IsSet())
		assert.False(t, cli.Location.Get())
	})

	t.Run("last flag wins positive then negative", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("negapp").
			Args([]string{"-L", "--no-location"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Location.IsSet())
		assert.False(t, cli.Location.Get())
	})

	t.Run("last flag wins negative then positive", func(t *testing.T) {
		t.Parallel()

		cli := new(CLI)
		app := conga.New(cli).
			Name("negapp").
			Args([]string{"--no-location", "-L"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Location.IsSet())
		assert.True(t, cli.Location.Get())
	})
}

type NegatedTokenCLI struct {
	noopRunner

	Port    conga.OptionalFlag[int]  `name:"port"     short:"p"`
	NetPort conga.OptionalFlag[int]  `name:"net-port"`
	Debug   conga.OptionalFlag[bool] `name:"debug"    negatable:"true" short:"d"`
	Verbose conga.OptionalFlag[bool] `name:"verbose"  short:"v"`
	NoCache conga.OptionalFlag[bool] `name:"no-cache"`
}

func TestBinder_NegatedTokenRouting(t *testing.T) {
	t.Parallel()

	t.Run("negated form of non-negatable flag is an unknown flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NegatedTokenCLI]().Name("negapp").Args([]string{"--no-port"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
		assert.Equal(t, "unknown flag: --no-port", err.Error())
	})

	t.Run("negated form of non-negatable bool flag is an unknown flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NegatedTokenCLI]().Name("negapp").Args([]string{"--no-verbose"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
		assert.Equal(t, "unknown flag: --no-verbose", err.Error())
	})

	t.Run("exact match on flag starting with no- binds without negation", func(t *testing.T) {
		t.Parallel()

		cli := new(NegatedTokenCLI)
		app := conga.New(cli).Name("negapp").Args([]string{"--no-cache"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.True(t, cli.NoCache.Get())
	})

	t.Run("negated form of negatable bool flag still binds", func(t *testing.T) {
		t.Parallel()

		cli := new(NegatedTokenCLI)
		app := conga.New(cli).Name("negapp").Args([]string{"--no-debug"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.False(t, cli.Debug.Get())
	})
}

func TestBinder_NegatedTokenSuggestionSuppression(t *testing.T) {
	t.Parallel()

	t.Run("no suggestion on negated non-negatable flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NegatedTokenCLI]().Name("negapp").Args([]string{"--no-port"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "Did you mean")
	})
}

func TestBinder_CaseInsensitiveFlagSuggestion(t *testing.T) {
	t.Parallel()

	app := conga.New[NegatedTokenCLI]().Name("negapp").Args([]string{"--PORT"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `Did you mean "port"?`)
}

func TestBinder_ShortFlagClusteringWithEqual(t *testing.T) {
	t.Parallel()

	type ShortClusterCLI struct {
		noopRunner

		All   conga.OptionalFlag[bool] `short:"a"`
		Quiet conga.OptionalFlag[bool] `short:"q"`
		Port  conga.OptionalFlag[int]  `short:"p"`
	}

	t.Run("single short flag with equal sign", func(t *testing.T) {
		t.Parallel()

		cli := new(ShortClusterCLI)
		app := conga.New(cli).Args([]string{"-p=8080"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, 8080, cli.Port.Get())
	})

	t.Run("clustered boolean flags and final flag with equal sign", func(t *testing.T) {
		t.Parallel()

		cli := new(ShortClusterCLI)
		app := conga.New(cli).Args([]string{"-aqp=9090"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.True(t, cli.All.Get())
		assert.True(t, cli.Quiet.Get())
		assert.Equal(t, 9090, cli.Port.Get())
	})
}

func TestBinder_UnicodeAndEmptyPositionalArgs(t *testing.T) {
	t.Parallel()

	type PosArgsCLI struct {
		noopRunner

		Arg1 conga.RequiredArg[string] `name:"<arg1>"`
		Arg2 conga.RequiredArg[string] `name:"<arg2>"`
	}

	cli := new(PosArgsCLI)
	app := conga.New(cli).Args([]string{"", "こんにちは世界 🚀"})
	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "", cli.Arg1.Get())
	assert.True(t, cli.Arg1.IsSet())
	assert.Equal(t, "こんにちは世界 🚀", cli.Arg2.Get())
}

func TestBinder_SubcommandRequired(t *testing.T) {
	t.Parallel()

	type RequiredSubParentCLI struct {
		Subcommand conga.Command[SubcommandChildCLI] `name:"sub"`
	}

	app := conga.New[RequiredSubParentCLI]().Name("app").Args([]string{})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
}

func TestBinder_FlagMissingValue(t *testing.T) {
	t.Parallel()

	type MissingValCLI struct {
		Port conga.OptionalFlag[int]    `name:"port" short:"p"`
		Host conga.OptionalFlag[string] `name:"host" short:"h"`
	}

	t.Run("long flag at end of args", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MissingValCLI]().Args([]string{"--port"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})

	t.Run("long flag followed by another flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MissingValCLI]().Args([]string{"--port", "--host", "localhost"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})

	t.Run("short flag at end of args", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MissingValCLI]().Args([]string{"-p"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})

	t.Run("short flag cluster followed by terminator", func(t *testing.T) {
		t.Parallel()

		app := conga.New[MissingValCLI]().Args([]string{"-p", "--"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})
}

func TestBinder_SliceValidationDelegation(t *testing.T) {
	t.Parallel()

	type SliceValCLI struct {
		noopRunner

		Ports conga.OptionalFlag[[]int] `items:"1..3" name:"ports" range:"1024..65535"`
	}

	t.Run("valid slice elements and count", func(t *testing.T) {
		t.Parallel()

		cli := new(SliceValCLI)
		app := conga.New(cli).Args([]string{"--ports", "8080,9090"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, []int{8080, 9090}, cli.Ports.Get())
	})

	t.Run("element out of range", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceValCLI]().Args([]string{"--ports", "8080,80"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("too many items", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceValCLI]().Args([]string{"--ports", "1024,1025,1026,1027"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestBinder_ArgCoercionError(t *testing.T) {
	t.Parallel()

	type ArgRangeCLI struct {
		Count conga.RequiredArg[int] `name:"<count>"`
	}

	app := conga.New[ArgRangeCLI]().Args([]string{"not-a-number"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
}

func TestBinder_PositionalMap(t *testing.T) {
	t.Parallel()

	type MapArgCLI struct {
		noopRunner

		Labels conga.RequiredArg[map[string]string] `name:"<labels...>"`
	}

	cli := new(MapArgCLI)
	app := conga.New(cli).Args([]string{"env=prod", "team=infra"})
	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 2, len(cli.Labels.Get()))
	assert.Equal(t, "prod", cli.Labels.Get()["env"])
	assert.Equal(t, "infra", cli.Labels.Get()["team"])
}

func TestBinder_ShortClusterAttachedValues(t *testing.T) {
	t.Parallel()

	type FlagsCLI struct {
		noopRunner

		Verbose conga.OptionalFlag[bool]   `name:"verbose" negatable:"" short:"v"`
		Output  conga.OptionalFlag[string] `name:"output"  short:"o"`
		Format  conga.OptionalFlag[string] `name:"format"  short:"f"`
	}

	t.Run("cluster preserves attached value on embedded non-bool flag", func(t *testing.T) {
		t.Parallel()

		cli := new(FlagsCLI)
		app := conga.New(cli).Args([]string{"-vof=json"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.True(t, cli.Verbose.Get())
		assert.Equal(t, "f=json", cli.Output.Get())
	})

	t.Run("cluster assigns attached value to terminal non-bool flag", func(t *testing.T) {
		t.Parallel()

		cli := new(FlagsCLI)
		app := conga.New(cli).Args([]string{"-vo=file.txt"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.True(t, cli.Verbose.Get())
		assert.Equal(t, "file.txt", cli.Output.Get())
	})

	t.Run("short bool flag with attached boolean value", func(t *testing.T) {
		t.Parallel()

		cli := new(FlagsCLI)
		app := conga.New(cli).Args([]string{"-v=false"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.False(t, cli.Verbose.Get())
		assert.True(t, cli.Verbose.IsSet())
	})

	t.Run("short bool flag with invalid attached value returns ErrCoerce", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FlagsCLI]().Args([]string{"-v=notabool"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
	})

	t.Run("negated flag with attached value returns ErrCoerce", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FlagsCLI]().Args([]string{"--no-verbose=true"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
	})
}

func TestBinder_DashArgsBare(t *testing.T) {
	t.Parallel()

	type BareDashCLI struct {
		noopRunner

		Extra conga.DashArgs `name:"options"`
	}

	t.Run("bare double dash marks DashArgs as set with empty slice", func(t *testing.T) {
		t.Parallel()

		cli := new(BareDashCLI)
		app := conga.New(cli).Args([]string{"--"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.True(t, cli.Extra.IsSet())
		assert.Equal(t, 0, cli.Extra.Len())
	})

	t.Run("omitted double dash leaves DashArgs unset", func(t *testing.T) {
		t.Parallel()

		cli := new(BareDashCLI)
		app := conga.New(cli).Args([]string{})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.False(t, cli.Extra.IsSet())
	})
}

func TestBinder_RawArgsWithDoubleDash(t *testing.T) {
	t.Parallel()

	type RawWithDashCLI struct {
		noopRunner

		Target conga.RequiredArg[string] `name:"<target>"`
		Rest   conga.RawArgs             `name:"[rest...]"`
	}

	cli := new(RawWithDashCLI)
	app := conga.New(cli).Args([]string{"mytarget", "--", "arg1", "--flag", "arg2"})
	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "mytarget", cli.Target.Get())
	assert.True(t, cli.Rest.IsSet())
	assert.Equal(t, []string{"arg1", "--flag", "arg2"}, cli.Rest.Get())
}

func TestBinder_EmptyStringEnv(t *testing.T) {
	type StringEnvCLI struct {
		noopRunner

		Name conga.OptionalFlag[string] `default:"default-name" env:"APP_NAME" name:"name"`
	}

	t.Setenv("APP_NAME", "")

	cli := new(StringEnvCLI)
	app := conga.New(cli).Args([]string{})
	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "", cli.Name.Get())
	assert.True(t, cli.Name.IsSet())
	assert.Equal(t, conga.SourceEnv, cli.Name.Source())
}

func TestBinder_EmptyStringConfig(t *testing.T) {
	t.Parallel()

	type StringConfigCLI struct {
		noopRunner

		Name conga.OptionalFlag[string] `config:"name" default:"default-name" name:"name"`
	}

	cli := new(StringConfigCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"name": ""}`))).
		Args([]string{})
	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "", cli.Name.Get())
	assert.True(t, cli.Name.IsSet())
	assert.Equal(t, conga.SourceConfig, cli.Name.Source())
}

func TestBinder_CounterCoercionError(t *testing.T) {
	t.Parallel()

	type CounterCLI struct {
		noopRunner

		Verbosity conga.Counter `name:"v" short:"v"`
	}

	app := conga.New[CounterCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"v": "not-a-number"}`))).
		Args([]string{})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
}
