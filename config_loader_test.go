package conga_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/internal/assertx"
)

type loaderBasicCLI struct {
	noopRunner

	Host conga.OptionalFlag[string] `default:"localhost"`
	Port conga.OptionalFlag[int]    `default:"8000"`
}

type loaderSliceCLI struct {
	noopRunner

	Tags conga.OptionalFlag[[]string] `default:"default"`
}

type loaderCounterCLI struct {
	noopRunner

	Verbosity conga.Counter `short:"v"`
}

type loaderMapCLI struct {
	noopRunner

	Headers conga.OptionalFlag[map[string]string]
}

type loaderSessionCLI struct {
	Host conga.OptionalFlag[string]
	Port conga.OptionalFlag[int]

	Loaded    []string
	Candidate []string
}

func (c *loaderSessionCLI) Run(_ context.Context, session *conga.Session) error {
	c.Loaded = session.LoadedConfigFiles()
	c.Candidate = session.CandidateConfigFiles()

	return nil
}

type loaderExcludedCLI struct {
	noopRunner

	Secret conga.OptionalFlag[string] `config:"-"`
}

type loaderNamespaceCLI struct {
	noopRunner

	Port conga.OptionalFlag[int] `config:"network.port" default:"80"`
}

type loaderEnumCLI struct {
	noopRunner

	Mode conga.OptionalFlag[string] `default:"dev" enum:"dev,prod"`
}

type loaderDeploySubcommand struct {
	Replicas conga.OptionalFlag[int] `default:"1"`
}

func (s *loaderDeploySubcommand) Run(_ context.Context, _ *conga.Session) error { return nil }

type loaderSubcommandCLI struct {
	noopRunner

	Deploy conga.Command[loaderDeploySubcommand]
}

type loaderExperiments struct {
	ForceAll bool `name:"force-all"`
}

type loaderFeaturesCLI struct {
	noopRunner

	Features conga.Features[loaderExperiments] `config:"experiments" env:"-"`

	ForceAll conga.OptionalFlag[bool] `feature:"force-all"`
}

// loaderNilDocument is a loader that returns no document: the configuration
// tier stays inactive for the run.
func loaderNilDocument[T any](conga.ConfigContext, *T) (*conga.ConfigDocument, error) {
	return nil, nil //nolint:nilnil // the nil document contract: returning no document keeps the tier inactive
}

func loaderDocument(entries ...func(document *conga.ConfigDocument)) *conga.ConfigDocument {
	document := &conga.ConfigDocument{}
	for _, entry := range entries {
		entry(document)
	}

	return document
}

func TestConfigLoader_DocumentOverridesDefault(t *testing.T) {
	t.Parallel()

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("host", "document.example.com", "embedded")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "document.example.com", cli.Host.Get())
	assert.Equal(t, conga.SourceConfig, cli.Host.Source())
	assert.Equal(t, 8000, cli.Port.Get())
	assert.Equal(t, conga.SourceDefault, cli.Port.Source())
}

func TestConfigLoader_EnvOverridesDocument(t *testing.T) {
	t.Setenv("CONGA_TEST_LOADER_HOST", "env.example.com")

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		EnvPrefix("CONGA_TEST_LOADER").
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("host", "document.example.com", "embedded")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "env.example.com", cli.Host.Get())
	assert.Equal(t, conga.SourceEnv, cli.Host.Source())
}

func TestConfigLoader_CLIOverridesDocument(t *testing.T) {
	t.Parallel()

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("host", "document.example.com", "embedded")
			}), nil
		})).
		Args([]string{"--host", "cli.example.com"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "cli.example.com", cli.Host.Get())
	assert.Equal(t, conga.SourceCLI, cli.Host.Source())
}

func TestConfigLoader_SliceFlagFromDocument(t *testing.T) {
	t.Parallel()

	cli := new(loaderSliceCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderSliceCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("tags", []string{"alpha", "beta"}, "embedded")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta"}, cli.Tags.Get())
	assert.Equal(t, conga.SourceConfig, cli.Tags.Source())
}

func TestConfigLoader_SliceFlagCLIOverridesDocument(t *testing.T) {
	t.Parallel()

	cli := new(loaderSliceCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderSliceCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("tags", []string{"alpha", "beta"}, "embedded")
			}), nil
		})).
		Args([]string{"--tags", "cli-one", "--tags", "cli-two"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"cli-one", "cli-two"}, cli.Tags.Get())
	assert.Equal(t, conga.SourceCLI, cli.Tags.Source())
}

func TestConfigLoader_CounterFromDocument(t *testing.T) {
	t.Parallel()

	cli := new(loaderCounterCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderCounterCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("verbosity", "2", "embedded")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 2, cli.Verbosity.Get())
	assert.Equal(t, conga.SourceConfig, cli.Verbosity.Source())
}

func TestConfigLoader_MapFlagMergesAcrossTiers(t *testing.T) {
	t.Parallel()

	cli := new(loaderMapCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderMapCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("headers", map[string]string{"region": "eu", "shared": "document"}, "taskrc")
			}), nil
		})).
		Args([]string{"--headers", "shared=cli", "--headers", "team=core"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"region": "eu", "shared": "cli", "team": "core"}, cli.Headers.Get())
	assert.Equal(t, conga.SourceCLI, cli.Headers.Source())
}

func TestConfigLoader_MapFlagNestedAnySection(t *testing.T) {
	t.Parallel()

	cli := new(loaderMapCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderMapCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("headers", map[string]any{"Region": "eu", "team": 3}, "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"Region": "eu", "team": "3"}, cli.Headers.Get())
	assert.Equal(t, conga.SourceConfig, cli.Headers.Source())
}

func TestConfigLoader_NilDocumentLeavesTierInactive(t *testing.T) {
	t.Parallel()

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(loaderNilDocument[loaderBasicCLI])).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", cli.Host.Get())
	assert.Equal(t, conga.SourceDefault, cli.Host.Source())
}

func TestConfigLoader_EmptyDocumentLeavesDefaults(t *testing.T) {
	t.Parallel()

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return &conga.ConfigDocument{}, nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", cli.Host.Get())
	assert.Equal(t, conga.SourceDefault, cli.Host.Source())
}

func TestConfigLoader_LoaderErrorClassifiedUnderErrConfig(t *testing.T) {
	t.Parallel()

	var stderrBuffer bytes.Buffer

	exitCode := -1

	app := conga.New[loaderBasicCLI]().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return nil, errors.New("taskrc unreadable")
		})).
		ExitFunc(func(code int) { exitCode = code }).
		Stderr(&stderrBuffer).
		Args([]string{})

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.False(t, conga.IsUsageError(err))
	assert.Equal(t, "config loader failed: taskrc unreadable", err.Error())

	app.Exit(err)

	assert.Equal(t, 1, exitCode)
}

func TestConfigLoader_UnknownKeyFailsWithSuggestion(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderBasicCLI]().
		Name("app").
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("hots", "typo.example.com", "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `config file "taskrc"`)
	assert.Contains(t, err.Error(), `unknown field "hots" in command "app"`)
	assert.Contains(t, err.Error(), `did you mean "host"?`)
}

func TestConfigLoader_ExcludedFlagRejected(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderExcludedCLI]().
		Name("app").
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderExcludedCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("secret", "leaked", "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `field "secret" is excluded from config files in command "app"`)
}

func TestConfigLoader_SubcommandSection(t *testing.T) {
	t.Parallel()

	cli := new(loaderSubcommandCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderSubcommandCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("deploy", map[string]any{"replicas": 3}, "taskrc")
			}), nil
		})).
		Args([]string{"deploy"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 3, cli.Deploy.Get().Replicas.Get())
	assert.Equal(t, conga.SourceConfig, cli.Deploy.Get().Replicas.Source())
}

func TestConfigLoader_NamespaceSection(t *testing.T) {
	t.Parallel()

	cli := new(loaderNamespaceCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderNamespaceCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("network", map[string]any{"port": 9090}, "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9090, cli.Port.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
}

func TestConfigLoader_EnumViolationMatchesFileMessageShape(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderEnumCLI]().
		Name("app").
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderEnumCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("mode", "staging", "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Equal(t, `invalid value "staging" (must be one of: dev, prod) for flag --mode`, err.Error())
}

func TestConfigLoader_RangeViolationMatchesFileMessageShape(t *testing.T) {
	t.Parallel()

	type RangeCLI struct {
		noopRunner

		Workers conga.OptionalFlag[int] `default:"2" range:"1..8"`
	}

	app := conga.New[RangeCLI]().
		Name("app").
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *RangeCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("workers", 64, "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Contains(t, err.Error(), "for flag --workers")
}

func TestConfigLoader_CoercionViolationAttributesToConfig(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderBasicCLI]().
		Name("app").
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("port", "not-a-number", "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.Contains(t, err.Error(), `for flag --port (from config key "port")`)
}

func TestConfigLoader_WhyReportsOrigins(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[loaderMapCLI]().
		EnableWhy().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderMapCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("headers", map[string]string{"region": "eu"}, "taskrc")
			}), nil
		})).
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	output := stdoutBuffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "--headers")
	assert.Contains(t, output, "config (taskrc)")
	assert.Contains(t, output, "loaded")
	assert.NotContains(t, output, "loaded: taskrc")
}

// TestConfigLoader_SourceDetailReportsOrigin pins that a document-supplied
// scalar records the application-supplied origin label as its source detail,
// matching the map-flag behavior.
func TestConfigLoader_SourceDetailReportsOrigin(t *testing.T) {
	t.Parallel()

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("port", 7100, "profile dev")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
	assert.Equal(t, "profile dev", cli.Port.SourceDetail())
}

// TestConfigLoader_SourceDetailFallsBackToConfigKey pins the documented
// fallback: when the loader supplies no origin for a key, the source detail
// reports the matched configuration key.
func TestConfigLoader_SourceDetailFallsBackToConfigKey(t *testing.T) {
	t.Parallel()

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("port", 7100, "")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
	assert.Equal(t, "port", cli.Port.SourceDetail())
}

func TestConfigLoader_ScopedOriginSeparatesDetailFromConfigPath(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		Name("app").
		EnableWhy().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.SetScoped("port", 9200, "pipeline.local.yaml", "pipeline deploy")
			}), nil
		})).
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "pipeline.local.yaml › pipeline deploy", cli.Port.SourceDetail())

	rows, configPaths, found := strings.Cut(stdoutBuffer.String(), "Config paths:")

	assert.True(t, found)
	assert.Contains(t, rows, "config (pipeline.local.yaml › pipeline deploy)")
	assert.Contains(t, configPaths, "pipeline.local.yaml")
	assert.Contains(t, configPaths, "loaded")
	assert.NotContains(t, configPaths, "pipeline deploy")
}

func TestConfigLoader_SetKeyScopedSeparatesDetailFromConfigPath(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[loaderMapCLI]().
		Name("app").
		EnableWhy().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderMapCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.SetKeyScoped("headers", "region", "eu", "taskrc", "profile dev")
			}), nil
		})).
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	rows, configPaths, found := strings.Cut(stdoutBuffer.String(), "Config paths:")

	assert.True(t, found)
	assert.Contains(t, rows, "region=██████")
	assert.Contains(t, rows, "config (taskrc › profile dev)")
	assert.Contains(t, configPaths, "taskrc")
	assert.NotContains(t, configPaths, "profile dev")
}

func TestConfigLoader_SessionAccessorsExcludeScope(t *testing.T) {
	t.Parallel()

	cli := new(loaderSessionCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderSessionCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(
				func(document *conga.ConfigDocument) {
					document.SetScoped("host", "document.example.com", "taskrc", "profile dev")
				},
				func(document *conga.ConfigDocument) {
					document.SetScoped("port", 9200, "taskrc", "profile prod")
				},
			), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"taskrc"}, cli.Loaded)
	assert.Equal(t, []string{"taskrc"}, cli.Candidate)
}

func TestConfigLoader_WhyAttributesAppError(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[loaderBasicCLI]().
		Name("app").
		EnableWhy().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return nil, errors.New("taskrc unreadable")
		})).
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "taskrc unreadable")
}

func TestConfigLoader_WhyValidationFailureReportsUniqueCandidate(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[loaderBasicCLI]().
		Name("app").
		EnableWhy().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("hots", "typo.example.com", "taskrc")
			}), nil
		})).
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `unknown field "hots"`)
}

func TestConfigLoader_SessionAccessorsReportOrigins(t *testing.T) {
	t.Parallel()

	cli := new(loaderSessionCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderSessionCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(
				func(document *conga.ConfigDocument) {
					document.Set("host", "document.example.com", "taskrc")
				},
				func(document *conga.ConfigDocument) {
					document.Set("port", 9200, "embedded")
				},
			), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"taskrc", "embedded"}, cli.Loaded)
	assert.Equal(t, []string{"taskrc", "embedded"}, cli.Candidate)
}

func TestConfigLoader_FeatureGateFromDocument(t *testing.T) {
	t.Parallel()

	cli := new(loaderFeaturesCLI)
	app := conga.New(cli).
		EnableFeatures().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("experiments", map[string]string{"force-all": "true"}, "taskrc")
			}), nil
		})).
		Args([]string{"--force-all"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Features.Get().ForceAll)
	assert.Equal(t, conga.SourceConfig, cli.Features.Source())
	assert.True(t, cli.ForceAll.Get())
	assert.Equal(t, conga.SourceCLI, cli.ForceAll.Source())
}

func TestConfigLoader_FeatureGateOffPrunesFlag(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			return &conga.ConfigDocument{}, nil
		})).
		Args([]string{"--force-all"})

	err := app.Run()

	assert.Error(t, err)
	assert.True(t, conga.IsUsageError(err))
}

func TestConfigLoader_FeatureEnvChannelExcluded(t *testing.T) {
	t.Setenv("CONGA_TEST_LOADER_FEATURES", "force-all=true")

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		EnvPrefix("CONGA_TEST_LOADER").
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			return &conga.ConfigDocument{}, nil
		})).
		Args([]string{"--force-all"})

	err := app.Run()

	assert.Error(t, err)
	assert.True(t, conga.IsUsageError(err))
}

func TestConfigLoader_FeatureInvalidValueFailsUnderErrConfig(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("experiments", map[string]string{"force-all": "maybe"}, "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
}

func TestConfigLoader_CompletionReflectsDocumentGate(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		EnableCompletion().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("experiments", map[string]string{"force-all": "true"}, "taskrc")
			}), nil
		}))

	invoked, _ := app.CompleteForShell("fish", []string{"--"})

	values := make(map[string]bool, len(invoked.Items))

	for _, item := range invoked.Items {
		values[item.Value] = true
	}

	assert.True(t, values["--force-all"])
}

func TestConfigLoader_CompletionPrunesDisabledGate(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		EnableCompletion().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			return &conga.ConfigDocument{}, nil
		}))

	invoked, _ := app.CompleteForShell("fish", []string{"--"})

	values := make(map[string]bool, len(invoked.Items))

	for _, item := range invoked.Items {
		values[item.Value] = true
	}

	assert.False(t, values["--force-all"])
}

func TestConfigLoader_CompletionInvokesLoaderOnce(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		EnableCompletion().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			invocations.Add(1)

			return &conga.ConfigDocument{}, nil
		}))

	app.CompleteForShell("fish", []string{"--"})

	assert.Equal(t, int32(1), invocations.Load())
}

func TestConfigLoader_RunsOncePerRun(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			invocations.Add(1)

			return &conga.ConfigDocument{}, nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, int32(1), invocations.Load())
}

func TestConfigLoader_RunsOnceAcrossWhy(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	var stdoutBuffer bytes.Buffer

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		EnableWhy().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			invocations.Add(1)

			return &conga.ConfigDocument{}, nil
		})).
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, int32(1), invocations.Load())
}

func TestConfigLoader_RunsForHelpWhenFeaturesEnabled(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	var stdoutBuffer bytes.Buffer

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			invocations.Add(1)

			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("experiments", map[string]string{"force-all": "true"}, "taskrc")
			}), nil
		})).
		Stdout(&stdoutBuffer).
		Args([]string{"--help"})

	err := app.Run()

	output := stdoutBuffer.String()

	assert.NoError(t, err)
	assert.Equal(t, int32(1), invocations.Load())
	assert.Contains(t, output, "--force-all")
}

func TestConfigLoader_HelpWithoutFeaturesSkipsLoader(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	var stdoutBuffer bytes.Buffer

	app := conga.New[loaderBasicCLI]().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			invocations.Add(1)

			return &conga.ConfigDocument{}, nil
		})).
		Stdout(&stdoutBuffer).
		Args([]string{"--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, int32(0), invocations.Load())
}

func TestConfigLoader_VersionSkipsLoader(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	app := conga.New[loaderFeaturesCLI]().
		EnableFeatures().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			invocations.Add(1)

			return &conga.ConfigDocument{}, nil
		})).
		Args([]string{"--version"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, int32(0), invocations.Load())
}

func TestConfigLoader_RepeatedOptionLastDeclarationWins(t *testing.T) {
	t.Parallel()

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("host", "first.example.com", "first")
			}), nil
		})).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.Set("host", "second.example.com", "second")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "second.example.com", cli.Host.Get())
}

func TestConfigLoader_NilLoaderPanics(t *testing.T) {
	t.Parallel()

	assertx.PanicsWithValue(t, "conga: app.ConfigOptions(conga.ConfigLoader(...)): the loader function cannot be nil", func() {
		conga.ConfigLoader[struct{}](nil)
	})
}

func TestConfigLoader_RootTypeMismatchPanics(t *testing.T) {
	t.Parallel()

	type mismatchCLI struct{}

	app := conga.New[loaderBasicCLI]()

	assertx.PanicsWithValue(t, "conga: app.ConfigOptions(conga.ConfigLoader(...)): loader root type *conga_test.mismatchCLI does not match application root type *conga_test.loaderBasicCLI", func() {
		app.ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *mismatchCLI) (*conga.ConfigDocument, error) {
			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		}))
	})
}

func TestConfigDocument_SetOnNilPanics(t *testing.T) {
	t.Parallel()

	var document *conga.ConfigDocument

	assertx.PanicsWithValue(t, "conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)", func() {
		document.Set("host", "value", "origin")
	})
}

func TestConfigDocument_RepeatedSetLastValueWins(t *testing.T) {
	t.Parallel()

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(
				func(document *conga.ConfigDocument) {
					document.Set("host", "first.example.com", "first")
				},
				func(document *conga.ConfigDocument) {
					document.Set("host", "second.example.com", "second")
				},
			), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "second.example.com", cli.Host.Get())
}

func TestConfigDocument_SetKeyRecordsMapEntry(t *testing.T) {
	t.Parallel()

	cli := new(loaderMapCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderMapCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.SetKey("headers", "region", "eu", "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"region": "eu"}, cli.Headers.Get())
	assert.Equal(t, conga.SourceConfig, cli.Headers.Source())
}

func TestConfigDocument_SetKeyAcceptsTypedValue(t *testing.T) {
	t.Parallel()

	cli := new(loaderFeaturesCLI)
	app := conga.New(cli).
		EnableFeatures().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderFeaturesCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.SetKey("experiments", "force-all", true, "taskrc")
			}), nil
		})).
		Args([]string{"--force-all"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Features.Get().ForceAll)
	assert.True(t, cli.ForceAll.Get())
}

func TestConfigDocument_SetKeyOverlaysEarlierSet(t *testing.T) {
	t.Parallel()

	cli := new(loaderMapCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderMapCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(
				func(document *conga.ConfigDocument) {
					document.Set("headers", map[string]string{"region": "eu", "shared": "base"}, "base")
				},
				func(document *conga.ConfigDocument) {
					document.SetKey("headers", "shared", "overlay", "workspace")
				},
			), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"region": "eu", "shared": "overlay"}, cli.Headers.Get())
}

func TestConfigDocument_SetReplacesEarlierSetKeyOverlays(t *testing.T) {
	t.Parallel()

	cli := new(loaderMapCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderMapCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(
				func(document *conga.ConfigDocument) {
					document.SetKey("headers", "region", "eu", "overlay")
				},
				func(document *conga.ConfigDocument) {
					document.Set("headers", map[string]string{"team": "core"}, "replacement")
				},
			), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"team": "core"}, cli.Headers.Get())
}

func TestConfigDocument_RepeatedSetKeyLastValueWins(t *testing.T) {
	t.Parallel()

	cli := new(loaderMapCLI)
	app := conga.New(cli).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderMapCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(
				func(document *conga.ConfigDocument) {
					document.SetKey("headers", "region", "eu", "first")
				},
				func(document *conga.ConfigDocument) {
					document.SetKey("headers", "region", "us", "second")
				},
			), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"region": "us"}, cli.Headers.Get())
}

func TestConfigDocument_SetKeyOnNilPanics(t *testing.T) {
	t.Parallel()

	var document *conga.ConfigDocument

	assertx.PanicsWithValue(t, "conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)", func() {
		document.SetKey("headers", "region", "eu", "origin")
	})
}

func TestConfigDocument_SetKeyUnknownKeyFailsUnderErrConfig(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderMapCLI]().
		Name("app").
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderMapCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.SetKey("header", "region", "eu", "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `unknown section "header" in command "app"`)
	assert.Contains(t, err.Error(), `did you mean "headers"?`)
}

func TestConfigLoader_ContextCommandPath(t *testing.T) {
	t.Parallel()

	var commandPath string

	app := conga.New[loaderSubcommandCLI]().
		Name("app").
		ConfigOptions(conga.ConfigLoader(func(ctx conga.ConfigContext, root *loaderSubcommandCLI) (*conga.ConfigDocument, error) {
			commandPath = ctx.CommandPath()

			return &conga.ConfigDocument{}, nil
		})).
		Args([]string{"deploy"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "app deploy", commandPath)
}

func TestConfigLoader_ContextLookupEnv(t *testing.T) {
	t.Setenv("CONGA_TEST_LOADER_CONTEXT_KEY", "context-value")

	var (
		value   string
		found   bool
		missing bool
	)

	app := conga.New[loaderBasicCLI]().
		ConfigOptions(conga.ConfigLoader(func(ctx conga.ConfigContext, root *loaderBasicCLI) (*conga.ConfigDocument, error) {
			value, found = ctx.LookupEnv("CONGA_TEST_LOADER_CONTEXT_KEY")
			_, missing = ctx.LookupEnv("CONGA_TEST_LOADER_CONTEXT_MISSING")

			return &conga.ConfigDocument{}, nil
		})).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "context-value", value)
	assert.True(t, found)
	assert.False(t, missing)
}

func TestConfigLoader_GuardCombinedWithPaths(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderBasicCLI]().
		ConfigOptions(conga.ConfigLoader(loaderNilDocument[loaderBasicCLI]), conga.ConfigPaths("app.json")).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, "schema error: conga.ConfigLoader() supplies the configuration document programmatically and cannot be combined with conga.ConfigPaths(), conga.ConfigExplicit(), or conga.ConfigCascading()", err.Error())
}

func TestConfigLoader_GuardCombinedWithExplicit(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderBasicCLI]().
		ConfigOptions(conga.ConfigLoader(loaderNilDocument[loaderBasicCLI]), conga.ConfigExplicit("app.json")).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, "schema error: conga.ConfigLoader() supplies the configuration document programmatically and cannot be combined with conga.ConfigPaths(), conga.ConfigExplicit(), or conga.ConfigCascading()", err.Error())
}

func TestConfigLoader_GuardCombinedWithCascading(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderBasicCLI]().
		ConfigOptions(conga.ConfigLoader(loaderNilDocument[loaderBasicCLI]), conga.ConfigCascading()).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, "schema error: conga.ConfigLoader() supplies the configuration document programmatically and cannot be combined with conga.ConfigPaths(), conga.ConfigExplicit(), or conga.ConfigCascading()", err.Error())
}

func TestConfigLoader_GuardCombinedWithDecoder(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderBasicCLI]().
		ConfigOptions(conga.ConfigLoader(loaderNilDocument[loaderBasicCLI])).
		RegisterDecoder(".json", json.Decode).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, "schema error: conga.ConfigLoader() replaces file loading and cannot be combined with app.RegisterDecoder (remove the decoder registrations or the loader)", err.Error())
}

func TestConfigLoader_GuardCombinedWithConfigFileField(t *testing.T) {
	t.Parallel()

	type ConfigFileCLI struct {
		noopRunner

		Config conga.ConfigFile `name:"config-file"`
	}

	app := conga.New[ConfigFileCLI]().
		ConfigOptions(conga.ConfigLoader(loaderNilDocument[ConfigFileCLI])).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, `schema error: the root command declares config-file flag "config-file", but conga.ConfigLoader() replaces file loading and never loads a file (remove the conga.ConfigFile field or the loader)`, err.Error())
}

func TestConfigDocument_SetKeyOnScalarFlagReportsNotAMap(t *testing.T) {
	t.Parallel()

	app := conga.New[loaderBasicCLI]().
		Name("app").
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.SetKey("port", "http", 8080, "taskrc")
			}), nil
		})).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Equal(t, `config file "taskrc": field "port" in command "app" is not a map`, err.Error())
}

func TestConfigLoader_ScopedOriginWithSlashAndCollidingBasenames(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := new(loaderBasicCLI)
	app := conga.New(cli).
		Name("app").
		EnableWhy().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, _ *loaderBasicCLI) (*conga.ConfigDocument, error) {
			return loaderDocument(func(document *conga.ConfigDocument) {
				document.SetScoped("host", "prod.example.com", "/srv/prod/pipelines/pipeline.yaml", "jobs/deploy")
				document.SetScoped("port", 9200, "/srv/dev/pipelines/pipeline.yaml", "jobs/release")
			}), nil
		})).
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "/srv/prod/pipelines/pipeline.yaml › jobs/deploy", cli.Host.SourceDetail())
	assert.Equal(t, "/srv/dev/pipelines/pipeline.yaml › jobs/release", cli.Port.SourceDetail())

	output := stdoutBuffer.String()

	assert.Contains(t, output, "config (/srv/prod/.../pipeline.yaml › jobs/deploy)")
	assert.Contains(t, output, "config (/srv/dev/.../pipeline.yaml › jobs/release)")
	assert.NotContains(t, output, "config (deploy)")
	assert.NotContains(t, output, "config (release)")
	assert.Contains(t, output, "/srv/prod/pipelines/pipeline.yaml")
	assert.Contains(t, output, "/srv/dev/pipelines/pipeline.yaml")
}
