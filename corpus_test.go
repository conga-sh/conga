package conga_test

import (
	"context"
	"embed"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/assertx"
	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/predict"
)

//go:embed internal/completion/testdata/corpus/*.json
var corpusFixtures embed.FS

// corpusFixtureDir is the fixture directory relative to the repository root
// (the working directory when running tests for the root package).
const corpusFixtureDir = "internal/completion/testdata/corpus"

// driverEnvVars lists every environment variable read by the completion
// drivers, internal/envx, and internal/style, so the corpus capture starts from
// a neutral environment. internal/completion keeps the same list for its own
// tests, and each package's TestDriverEnvVars_CoverWireConstants pins its copy
// to the wire constants.
//
//nolint:gochecknoglobals // shared list of driver environment variables for test isolation
var driverEnvVars = []string{
	wire.EnvCompLine,
	wire.EnvCompPoint,
	wire.EnvCompType,
	wire.EnvCompWordbreaks,
	wire.EnvColor,
	wire.EnvNoColor,
	wire.EnvCLIColor,
	wire.EnvDescriptionLength,
	wire.EnvMergeFlags,
	wire.EnvShell,
	wire.EnvTooltip,
	wire.EnvZshListSeparator,
	wire.EnvZshQuote,
	wire.EnvZshBanghist,
	wire.EnvZshStyleLimit,
	"LS_COLORS",
}

func TestCorpusStable(t *testing.T) {
	for _, key := range driverEnvVars {
		t.Setenv(key, "")
	}

	// The fixture directory is resolved before t.Chdir relocates the process;
	// scenarios run inside a disposable working directory so file-based
	// predictors observe deterministic content.
	fixtureDir, err := filepath.Abs(corpusFixtureDir)
	assert.NoError(t, err)

	scenarios := corpusScenarios()
	expectedFiles := make(map[string]bool, len(scenarios))

	for _, scenario := range scenarios {
		expectedFiles[scenario.Name+".json"] = true
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			for k, v := range scenario.Env {
				t.Setenv(k, v)
			}

			if scenario.CompLine != "" {
				point := scenario.CompPoint
				if point == 0 {
					point = len(scenario.CompLine)
				}

				t.Setenv("COMP_LINE", scenario.CompLine)
				t.Setenv("COMP_POINT", strconv.Itoa(point))
			}

			for _, name := range scenario.Files {
				assert.NoError(t, os.WriteFile(name, []byte("fixture\n"), 0o600))
			}

			first, second := captureCorpusScenario(scenario)

			// Determinism guard: a second capture through a fresh app must be identical.
			assert.Equal(t, string(first), string(second), "engine output is nondeterministic for scenario %s", scenario.Name)

			if *updateGolden {
				assert.NoError(t, os.MkdirAll(fixtureDir, 0o755))
				assert.NoError(t, os.WriteFile(filepath.Join(fixtureDir, scenario.Name+".json"), first, 0o644))

				return
			}

			existing, err := corpusFixtures.ReadFile(corpusFixtureDir + "/" + scenario.Name + ".json")
			assert.NoError(t, err, "missing corpus fixture %s; regenerate with task generate", scenario.Name+".json")
			assert.Equal(t, string(existing), string(first), "corpus fixture drift for scenario %s", scenario.Name)
		})
	}

	if *updateGolden {
		entries, err := os.ReadDir(fixtureDir)
		assert.NoError(t, err)

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") && !expectedFiles[entry.Name()] {
				assert.NoError(t, os.Remove(filepath.Join(fixtureDir, entry.Name())))
			}
		}

		return
	}

	entries, err := fs.ReadDir(corpusFixtures, corpusFixtureDir)
	assert.NoError(t, err)

	for _, entry := range entries {
		if !entry.IsDir() {
			assert.True(t, expectedFiles[entry.Name()], "stray corpus fixture %s; regenerate with task generate", entry.Name())
		}
	}
}

// TestDriverEnvVars_CoverWireConstants pins driverEnvVars to the Env*
// constants declared in internal/completion/wire, so a new driver variable
// cannot leak the caller's shell into the corpus capture. LS_COLORS is read by
// internal/style and has no wire constant.
func TestDriverEnvVars_CoverWireConstants(t *testing.T) {
	t.Parallel()

	envxValues := envConstantValues(t, "internal/envx/envx.go", nil)
	wireValues := envConstantValues(t, "internal/completion/wire/env.go", envxValues)

	expected := append(slices.Collect(maps.Values(wireValues)), "LS_COLORS")
	slices.Sort(expected)

	assert.Equal(t, expected, slices.Sorted(slices.Values(driverEnvVars)))
}

// corpusProfile demonstrates the self-completing Predictable contract on a named type.
type corpusProfile string

// Predict returns the profile candidates with descriptions.
func (c corpusProfile) Predict() predict.Predictor {
	return predict.Items(
		predict.NewItem("production", "Production workloads"),
		predict.NewItem("staging", "Pre-production staging cluster"),
		predict.NewItem("development", "Local sandbox environment"),
	)
}

// corpusRegion demonstrates Predictable with region-shaped values.
type corpusRegion string

// Predict returns the region candidates with descriptions.
func (c corpusRegion) Predict() predict.Predictor {
	return predict.Items(
		predict.NewItem("us-east-1", "North America (N. Virginia)"),
		predict.NewItem("eu-west-1", "Europe (Ireland)"),
		predict.NewItem("ap-southeast-1", "Asia Pacific (Singapore)"),
	)
}

type corpusRoot struct {
	Verbose  conga.OptionalFlag[bool]              `help:"Enable verbose output"             negatable:"true"          short:"v"`
	Profile  conga.OptionalFlag[corpusProfile]     `help:"Cloud profile"                     short:"p"`
	EnvVars  conga.OptionalFlag[map[string]string] `help:"Environment key=value pairs"       short:"e"`
	Shards   conga.OptionalFlag[map[string]int]    `help:"Shard counts, semicolon-separated" sep:";"`
	Features conga.OptionalFlag[[]string]          `help:"Comma-separated feature flags"     short:"f"`
	Secret   conga.OptionalFlag[string]            `help:"Hidden tuning knob"                hidden:"true"`
	Legacy   conga.OptionalFlag[string]            `deprecated:"use --profile"               help:"Legacy tuning knob"`

	Deploy     conga.Command[corpusDeploy] `aliases:"d"                                 section:"cluster"             summary:"Deploy service to a cluster"`
	Logs       conga.Command[corpusLogs]   `section:"cluster"                           summary:"Stream service logs"`
	Exec       conga.Command[corpusExec]   `summary:"Execute a command in a pod"`
	CP         conga.Command[corpusCopy]   `summary:"Copy files"`
	Completion conga.CompletionCommand     `summary:"Generate shell completion scripts"`
}

// Run is the no-op root execution hook.
func (c *corpusRoot) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusDeploy struct {
	Env      conga.OptionalFlag[string]       `enum:"staging,production,qa"    help:"Target environment"`
	Region   conga.OptionalFlag[corpusRegion] `help:"Target region"`
	DryRun   conga.OptionalFlag[bool]         `help:"Preview without applying" negatable:"true"`
	Replicas conga.OptionalFlag[int]          `help:"Replica count"`
	Manifest conga.OptionalFlag[string]       `help:"Deployment manifest"      predictor:"file:*.yaml,*.yml,*.json"`
	Timeout  conga.OptionalFlag[string]       `help:"Wall-clock timeout"       predictor:"none"`
}

// Run is the no-op deploy execution hook.
func (c *corpusDeploy) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusLogs struct {
	Follow  conga.OptionalFlag[bool]     `help:"Follow the log stream"             short:"F"`
	Tail    conga.OptionalFlag[[]string] `help:"Services to tail, comma-separated" predictor:"services"`
	Service conga.RequiredArg[string]    `help:"Primary service"                   name:"<service>"     predictor:"services"`
}

// Run is the no-op logs execution hook.
func (c *corpusLogs) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusExec struct {
	Namespace conga.OptionalFlag[string]  `default:"default"            help:"Kubernetes namespace"`
	Command   conga.RequiredArg[string]   `enum:"sh,bash,python"        help:"Interpreter to run"   name:"<command>"`
	Args      conga.OptionalArg[[]string] `help:"Interpreter arguments" name:"[args...]"`
}

// Run is the no-op exec execution hook.
func (c *corpusExec) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusCopy struct {
	Source conga.RequiredArg[string] `help:"Source path" name:"<source>" predictor:"file"`
	Target conga.RequiredArg[string] `help:"Target path" name:"<target>"`
}

// Run is the no-op copy execution hook.
func (c *corpusCopy) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusItem struct {
	Value       string `json:"value"`
	Display     string `json:"display"`
	Description string `json:"description,omitempty"`
	Style       string `json:"style,omitempty"`
	Tag         string `json:"tag,omitempty"`
}

type corpusDirectives struct {
	Error      bool `json:"error"`
	KeepOrder  bool `json:"keeporder"`
	NoFileComp bool `json:"nofilecomp"`
	NoSpace    bool `json:"nospace"`
}

type corpusResult struct {
	Items      []corpusItem     `json:"items"`
	Directives corpusDirectives `json:"directives"`
	Usage      string           `json:"usage"`
}

type corpusScenario struct {
	Name      string            `json:"name"`
	Shell     string            `json:"shell"`
	Words     []string          `json:"words,omitempty"`
	Files     []string          `json:"files,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	CompLine  string            `json:"compline,omitempty"`
	CompPoint int               `json:"comppoint,omitempty"`
	// App selects the fixture application: "" is the main corpus app, "map"
	// the dedicated map app whose labels flag carries a registered map value
	// predictor (key/value phase and multi-entry dedup semantics), "trailing"
	// the dedicated trailing-collector app.
	App string `json:"app,omitempty"`
}

func newCorpusApp() *conga.App[corpusRoot] {
	app := conga.New[corpusRoot]().
		EnableCompletion().
		Name("corpusctl").
		Description("Corpus fixture application for completion driver conformance").
		Version("1.2.3").
		EnableWhy()

	app.Predictor("services", predict.Items(
		predict.NewItem("api", "REST API service"),
		predict.NewItem("scheduler", "Cron scheduler"),
		predict.NewItem("worker", "Background worker"),
	))

	return app
}

// newCorpusWhyJSONApp mirrors newCorpusApp with the JSON --why format enabled,
// exercising the value-bearing --why candidates.
func newCorpusWhyJSONApp() *conga.App[corpusRoot] {
	app := conga.New[corpusRoot]().
		EnableCompletion().
		Name("corpusctl").
		Description("Corpus fixture application for completion driver conformance").
		Version("1.2.3").
		EnableWhy(conga.WhyOptionJSON())

	app.Predictor("services", predict.Items(
		predict.NewItem("api", "REST API service"),
		predict.NewItem("scheduler", "Cron scheduler"),
		predict.NewItem("worker", "Background worker"),
	))

	return app
}

// corpusMapRoot is the root of the dedicated map fixture application.
type corpusMapRoot struct {
	Deploy conga.Command[corpusMapDeploy] `summary:"Deploy service"`
}

// Run is the no-op map root execution hook.
func (c *corpusMapRoot) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusMapDeploy struct {
	Labels conga.OptionalFlag[map[string]string] `help:"Metadata labels in key=value format" predictor:"labels" short:"l"`
}

// Run is the no-op map deploy execution hook.
func (c *corpusMapDeploy) Run(ctx context.Context, session *conga.Session) error { return nil }

func newCorpusMapApp() *conga.App[corpusMapRoot] {
	app := conga.New[corpusMapRoot]().
		EnableCompletion().
		Name("corpusctl").
		Description("Corpus map fixture application for completion driver conformance")

	app.Predictor("labels", predict.KeyValue(map[string][]string{
		"team":  {"core", "frontend", "infra"},
		"tier":  {"frontend", "backend", "database"},
		"owner": {"alice", "bob", "carol"},
	}))

	return app
}

// corpusTrailingRoot is the root of the dedicated trailing-collector fixture
// application: DashArgs with and without a predictor, RawArgs after overflow,
// and a command declaring both collectors.
type corpusTrailingRoot struct {
	Verbose conga.OptionalFlag[bool] `help:"Enable verbose output" short:"v"`

	Forward conga.Command[corpusTrailingRun]   `summary:"Forward via tagged DashArgs"`
	Quiet   conga.Command[corpusTrailingQuiet] `summary:"Forward via untagged DashArgs"`
	Exec    conga.Command[corpusTrailingExec]  `summary:"Forward via tagged RawArgs"`
	Both    conga.Command[corpusTrailingBoth]  `summary:"Declare both collectors"`
}

// Run is the no-op trailing root execution hook.
func (c *corpusTrailingRoot) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusTrailingRun struct {
	Target conga.RequiredArg[string] `name:"<target>"`
	Dash   conga.DashArgs            `predictor:"file"`
}

// Run is the no-op tagged dash execution hook.
func (c *corpusTrailingRun) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusTrailingQuiet struct {
	Target conga.RequiredArg[string] `name:"<target>"`
	Dash   conga.DashArgs
}

// Run is the no-op untagged dash execution hook.
func (c *corpusTrailingQuiet) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusTrailingExec struct {
	Image conga.RequiredArg[string] `name:"<image>"`
	Raw   conga.RawArgs             `predictor:"file"`
}

// Run is the no-op raw execution hook.
func (c *corpusTrailingExec) Run(ctx context.Context, session *conga.Session) error { return nil }

type corpusTrailingBoth struct {
	Target conga.RequiredArg[string] `name:"<target>"`
	Dash   conga.DashArgs
	Raw    conga.RawArgs `predictor:"file"`
}

// Run is the no-op combined-collector execution hook.
func (c *corpusTrailingBoth) Run(ctx context.Context, session *conga.Session) error { return nil }

func newCorpusTrailingApp() *conga.App[corpusTrailingRoot] {
	return conga.New[corpusTrailingRoot]().
		EnableCompletion().
		Name("corpusctl").
		Description("Corpus trailing-argument fixture application for completion driver conformance")
}

func corpusScenarios() []corpusScenario {
	return []corpusScenario{
		{Name: "root_bare_export", Shell: "export", Words: []string{""}},
		{Name: "root_bare_flags_enabled", Shell: "export", Words: []string{""}, Env: map[string]string{"CORPUSCTL_COMPLETION_BARE_SPACE_FLAGS": "1"}},
		{Name: "root_bare_bash", Shell: "bash", Words: []string{""}},
		{Name: "root_subcommand_prefix", Shell: "export", Words: []string{"dep"}},
		{Name: "root_completion_candidate", Shell: "export", Words: []string{"comp"}},
		{Name: "root_flag_prefix", Shell: "export", Words: []string{"--"}},
		{Name: "root_negatable_prefix", Shell: "export", Words: []string{"--no"}},
		{Name: "root_shorthand_prefix", Shell: "export", Words: []string{"-"}},
		{Name: "root_bool_attached", Shell: "export", Words: []string{"--verbose="}},
		{Name: "root_profile_predictable", Shell: "export", Words: []string{"--profile", ""}},
		{Name: "root_map_usage", Shell: "export", Words: []string{"--env-vars", ""}},
		{Name: "root_slice_continuation_usage", Shell: "export", Words: []string{"--features", "net,"}},
		{Name: "deploy_bare", Shell: "export", Words: []string{"deploy", ""}},
		{Name: "deploy_alias", Shell: "export", Words: []string{"d", ""}},
		{Name: "deploy_enum_attached_bare", Shell: "export", Words: []string{"deploy", "--env="}},
		{Name: "deploy_enum_attached_filtered", Shell: "export", Words: []string{"deploy", "--env=pr"}},
		{Name: "deploy_enum_separate", Shell: "export", Words: []string{"deploy", "--env", "st"}},
		{Name: "deploy_negatable_prefix", Shell: "export", Words: []string{"deploy", "--no"}},
		{Name: "deploy_region_descriptions", Shell: "export", Words: []string{"deploy", "--region", ""}},
		{Name: "deploy_predictor_none_usage", Shell: "export", Words: []string{"deploy", "--timeout", ""}},
		{Name: "deploy_manifest_patterns", Shell: "export", Words: []string{"deploy", "--manifest", ""}, Files: []string{"deploy.yaml", "values.json", "readme.txt"}},
		{Name: "logs_bare", Shell: "export", Words: []string{"logs", ""}},
		{Name: "logs_slice_exclusion", Shell: "export", Words: []string{"logs", "--tail", "api,"}},
		{Name: "exec_enum_positional", Shell: "export", Words: []string{"exec", ""}},
		{Name: "exec_terminator_args", Shell: "export", Words: []string{"exec", "--", ""}},
		{Name: "copy_file_positional", Shell: "export", Words: []string{"cp", ""}, Files: []string{"a.yaml", "b.json", "notes.md"}},
		{Name: "copy_second_usage", Shell: "export", Words: []string{"cp", "a.yaml", ""}, Files: []string{"a.yaml"}},
		{Name: "exec_single_quoted_prefix", Shell: "export", Words: []string{"exec", "'s"}},
		{Name: "exec_double_quoted_prefix", Shell: "export", Words: []string{"exec", "\"b"}},
		{Name: "deploy_attached_quoted_enum", Shell: "export", Words: []string{"deploy", "--env=\"pro"}},
		{Name: "logs_quoted_positional_and_flags", Shell: "export", Words: []string{"logs", "'api'", "--"}},
		{Name: "root_redirect_out", Shell: "bash", CompLine: "corpusctl deploy > /tmp/output.log --env ", Words: []string{"deploy", "--env", ""}},
		{Name: "root_pipeline_isolate", Shell: "bash", CompLine: "echo foo | corpusctl dep", Words: []string{"dep"}},
		{Name: "deploy_successive_tabs", Shell: "bash", Words: []string{"deploy", "--"}, Env: map[string]string{"COMP_TYPE": "63"}},
		{Name: "root_map_key_prefix", Shell: "export", Words: []string{"--env-vars", "FO"}},
		{Name: "root_map_attached_key", Shell: "export", Words: []string{"--env-vars=FOO="}},
		{Name: "root_map_custom_sep", Shell: "export", Words: []string{"--shards", "cache="}},
		{Name: "map_keys_bare", Shell: "export", Words: []string{"deploy", "--labels", ""}, App: "map"},
		{Name: "map_values_separate", Shell: "export", Words: []string{"deploy", "--labels", "team="}, App: "map"},
		{Name: "map_multi_entry_key_dedup", Shell: "export", Words: []string{"deploy", "--labels", "team=core,"}, App: "map"},
		{Name: "map_repeated_key_value_dedup", Shell: "export", Words: []string{"deploy", "--labels", "owner=alice,owner="}, App: "map"},
		{Name: "trailing_dash_files", Shell: "export", Words: []string{"forward", "svc", "--", ""}, Files: []string{"manifest.yaml"}, App: "trailing"},
		{Name: "trailing_dash_none", Shell: "export", Words: []string{"quiet", "svc", "--", ""}, App: "trailing"},
		{Name: "trailing_both_dash_wins", Shell: "export", Words: []string{"both", "svc", "--", ""}, App: "trailing"},
		{Name: "trailing_both_raw_swallows", Shell: "export", Words: []string{"both", "svc", "extra", "--", ""}, Files: []string{"forwarded.txt"}, App: "trailing"},
		{Name: "trailing_raw_files", Shell: "export", Words: []string{"exec", "img", ""}, Files: []string{"script.sh"}, App: "trailing"},
		{Name: "trailing_raw_known_flag", Shell: "export", Words: []string{"exec", "img", "-"}, App: "trailing"},
		{Name: "trailing_raw_active_suppresses_flags", Shell: "export", Words: []string{"exec", "img", "extra", "-"}, App: "trailing"},
		{Name: "root_hidden_flag_excluded", Shell: "export", Words: []string{"--sec"}},
		{Name: "root_deprecated_flag", Shell: "export", Words: []string{"--leg"}},
		{Name: "exec_default_flag", Shell: "export", Words: []string{"exec", "--namespace", ""}},
		{Name: "root_why_attached_bare", Shell: "export", Words: []string{"--why="}},
		{Name: "whyjson_flag_prefix", Shell: "export", Words: []string{"--why"}, App: "whyjson"},
		{Name: "whyjson_attached_bare", Shell: "export", Words: []string{"--why="}, App: "whyjson"},
		{Name: "whyjson_attached_filtered", Shell: "export", Words: []string{"--why=j"}, App: "whyjson"},
		{Name: "root_bare_zsh", Shell: "zsh", Words: []string{""}},
		{Name: "root_bare_fish", Shell: "fish", Words: []string{""}},
		{Name: "root_bare_powershell", Shell: "powershell", Words: []string{""}},
		{Name: "root_bare_elvish", Shell: "elvish", Words: []string{""}},
		{Name: "root_bare_nushell", Shell: "nushell", Words: []string{""}},
	}
}

// captureCorpusScenario captures the scenario's engine result twice through
// fresh applications of the scenario's fixture app; the pair lets the caller
// assert determinism.
func captureCorpusScenario(scenario corpusScenario) ([]byte, []byte) {
	capture := func() []byte {
		if scenario.App == "map" {
			return captureCorpusResult(newCorpusMapApp(), scenario)
		}

		if scenario.App == "whyjson" {
			return captureCorpusResult(newCorpusWhyJSONApp(), scenario)
		}

		if scenario.App == "trailing" {
			return captureCorpusResult(newCorpusTrailingApp(), scenario)
		}

		return captureCorpusResult(newCorpusApp(), scenario)
	}

	return capture(), capture()
}

func captureCorpusResult[T any](app *conga.App[T], scenario corpusScenario) []byte {
	var result predict.Result

	if scenario.CompLine != "" {
		point := scenario.CompPoint
		if point == 0 {
			point = len(scenario.CompLine)
		}

		result, _ = app.CompleteCompLineForShell(scenario.Shell, scenario.CompLine, point)
	} else {
		result, _ = app.CompleteForShell(scenario.Shell, scenario.Words)
	}

	items := make([]corpusItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, corpusItem{
			Value:       item.Value,
			Display:     item.Display,
			Description: item.Description,
			Style:       item.Style,
			Tag:         item.Tag,
		})
	}

	recorded := corpusResult{
		Items: items,
		Directives: corpusDirectives{
			Error:      result.Directive.Has(predict.DirectiveError),
			KeepOrder:  result.Directive.Has(predict.DirectiveKeepOrder),
			NoFileComp: result.Directive.Has(predict.DirectiveNoFileCompletion),
			NoSpace:    result.Directive.Has(predict.DirectiveNoSpace),
		},
		Usage: result.Usage,
	}

	data, err := json.MarshalIndent(recorded, "", "  ")
	if err != nil {
		panic(err)
	}

	return append(data, '\n')
}

// envConstantValues parses the Go file at path and returns the string value of
// every package-level constant whose name starts with Env, keyed by name. A
// constant defined as envx.<Name> resolves through imported.
func envConstantValues(t *testing.T, path string, imported map[string]string) map[string]string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	assert.NoError(t, err)

	values := map[string]string{}

	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}

		for _, item := range general.Specs {
			valueSpec := item.(*ast.ValueSpec)

			for i, name := range valueSpec.Names {
				if !strings.HasPrefix(name.Name, "Env") {
					continue
				}

				switch expression := valueSpec.Values[i].(type) {
				case *ast.BasicLit:
					unquoted, err := strconv.Unquote(expression.Value)
					assert.NoError(t, err)

					values[name.Name] = unquoted
				case *ast.SelectorExpr:
					value, ok := imported[expression.Sel.Name]
					assert.True(t, ok, "constant %s refers to unknown %s", name.Name, expression.Sel.Name)

					values[name.Name] = value
				default:
					assertx.Unreachable(t, "constant "+name.Name+" has an unsupported definition")
				}
			}
		}
	}

	return values
}
