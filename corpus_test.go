package conga_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

// corpusFixtureDir is the fixture directory relative to the repository root
// (the working directory when running tests for the root package).
const corpusFixtureDir = "internal/completion/testdata/corpus"

func TestCorpusStable(t *testing.T) {
	t.Setenv("LS_COLORS", "")

	// The fixture directory is resolved before t.Chdir relocates the process;
	// scenarios run inside a disposable working directory so file-based
	// predictors observe deterministic content.
	fixtureDir, err := filepath.Abs(corpusFixtureDir)
	assert.NoError(t, err)

	capture := os.Getenv("CONGA_CAPTURE_CORPUS") == "1"
	scenarios := corpusScenarios()

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

			fixturePath := filepath.Join(fixtureDir, scenario.Name+".json")
			if capture {
				assert.NoError(t, os.MkdirAll(fixtureDir, 0o755))
				assert.NoError(t, os.WriteFile(fixturePath, first, 0o644))

				return
			}

			existing, err := os.ReadFile(fixturePath)
			assert.NoError(t, err, "missing corpus fixture %s; regenerate with CONGA_CAPTURE_CORPUS=1 go test . -run TestCorpusStable", scenario.Name+".json")
			assert.Equal(t, string(existing), string(first), "corpus fixture drift for scenario %s", scenario.Name)
		})
	}
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

	Deploy     conga.Command[corpusDeploy] `aliases:"d"                                 group:"cluster"               summary:"Deploy service to a cluster"`
	Logs       conga.Command[corpusLogs]   `group:"cluster"                             summary:"Stream service logs"`
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
	// predictor (key/value phase and multi-entry dedup semantics).
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
	app.RegisterMap[map[string]int]()

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
	app.RegisterMap[map[string]int]()

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

func corpusScenarios() []corpusScenario {
	return []corpusScenario{
		{Name: "root-bare-export", Shell: "export", Words: []string{""}},
		{Name: "root-bare-flags-enabled", Shell: "export", Words: []string{""}, Env: map[string]string{"CORPUSCTL_COMPLETION_BARE_SPACE_FLAGS": "1"}},
		{Name: "root-bare-bash", Shell: "bash", Words: []string{""}},
		{Name: "root-subcommand-prefix", Shell: "export", Words: []string{"dep"}},
		{Name: "root-completion-candidate", Shell: "export", Words: []string{"comp"}},
		{Name: "root-flag-prefix", Shell: "export", Words: []string{"--"}},
		{Name: "root-negatable-prefix", Shell: "export", Words: []string{"--no"}},
		{Name: "root-shorthand-prefix", Shell: "export", Words: []string{"-"}},
		{Name: "root-bool-attached", Shell: "export", Words: []string{"--verbose="}},
		{Name: "root-profile-predictable", Shell: "export", Words: []string{"--profile", ""}},
		{Name: "root-map-usage", Shell: "export", Words: []string{"--env-vars", ""}},
		{Name: "root-slice-nospace-usage", Shell: "export", Words: []string{"--features", "net,"}},
		{Name: "deploy-bare", Shell: "export", Words: []string{"deploy", ""}},
		{Name: "deploy-alias", Shell: "export", Words: []string{"d", ""}},
		{Name: "deploy-enum-attached-bare", Shell: "export", Words: []string{"deploy", "--env="}},
		{Name: "deploy-enum-attached-filtered", Shell: "export", Words: []string{"deploy", "--env=pr"}},
		{Name: "deploy-enum-separate", Shell: "export", Words: []string{"deploy", "--env", "st"}},
		{Name: "deploy-negatable-prefix", Shell: "export", Words: []string{"deploy", "--no"}},
		{Name: "deploy-region-descriptions", Shell: "export", Words: []string{"deploy", "--region", ""}},
		{Name: "deploy-predictor-none-usage", Shell: "export", Words: []string{"deploy", "--timeout", ""}},
		{Name: "deploy-manifest-patterns", Shell: "export", Words: []string{"deploy", "--manifest", ""}, Files: []string{"deploy.yaml", "values.json", "readme.txt"}},
		{Name: "logs-bare", Shell: "export", Words: []string{"logs", ""}},
		{Name: "logs-slice-exclusion", Shell: "export", Words: []string{"logs", "--tail", "api,"}},
		{Name: "exec-enum-positional", Shell: "export", Words: []string{"exec", ""}},
		{Name: "exec-terminator-args", Shell: "export", Words: []string{"exec", "--", ""}},
		{Name: "copy-file-positional", Shell: "export", Words: []string{"cp", ""}, Files: []string{"a.yaml", "b.json", "notes.md"}},
		{Name: "copy-second-usage", Shell: "export", Words: []string{"cp", "a.yaml", ""}, Files: []string{"a.yaml"}},
		{Name: "exec-single-quoted-prefix", Shell: "export", Words: []string{"exec", "'s"}},
		{Name: "exec-double-quoted-prefix", Shell: "export", Words: []string{"exec", "\"b"}},
		{Name: "deploy-attached-quoted-enum", Shell: "export", Words: []string{"deploy", "--env=\"pro"}},
		{Name: "logs-quoted-positional-and-flags", Shell: "export", Words: []string{"logs", "'api'", "--"}},
		{Name: "root-redirect-out", Shell: "bash", CompLine: "corpusctl deploy > /tmp/output.log --env ", Words: []string{"deploy", "--env", ""}},
		{Name: "root-pipeline-isolate", Shell: "bash", CompLine: "echo foo | corpusctl dep", Words: []string{"dep"}},
		{Name: "deploy-successive-tabs", Shell: "bash", Words: []string{"deploy", "--"}, Env: map[string]string{"COMP_TYPE": "63"}},
		{Name: "root-map-key-prefix", Shell: "export", Words: []string{"--env-vars", "FO"}},
		{Name: "root-map-attached-key", Shell: "export", Words: []string{"--env-vars=FOO="}},
		{Name: "root-map-custom-sep", Shell: "export", Words: []string{"--shards", "cache="}},
		{Name: "map-keys-bare", Shell: "export", Words: []string{"deploy", "--labels", ""}, App: "map"},
		{Name: "map-values-separate", Shell: "export", Words: []string{"deploy", "--labels", "team="}, App: "map"},
		{Name: "map-multi-entry-key-dedup", Shell: "export", Words: []string{"deploy", "--labels", "team=core,"}, App: "map"},
		{Name: "map-repeated-key-value-dedup", Shell: "export", Words: []string{"deploy", "--labels", "owner=alice,owner="}, App: "map"},
		{Name: "root-hidden-flag-excluded", Shell: "export", Words: []string{"--sec"}},
		{Name: "root-deprecated-flag", Shell: "export", Words: []string{"--leg"}},
		{Name: "exec-default-flag", Shell: "export", Words: []string{"exec", "--namespace", ""}},
		{Name: "root-why-attached-bare", Shell: "export", Words: []string{"--why="}},
		{Name: "whyjson-flag-prefix", Shell: "export", Words: []string{"--why"}, App: "whyjson"},
		{Name: "whyjson-attached-bare", Shell: "export", Words: []string{"--why="}, App: "whyjson"},
		{Name: "whyjson-attached-filtered", Shell: "export", Words: []string{"--why=j"}, App: "whyjson"},
		{Name: "root-bare-zsh", Shell: "zsh", Words: []string{""}},
		{Name: "root-bare-fish", Shell: "fish", Words: []string{""}},
		{Name: "root-bare-powershell", Shell: "powershell", Words: []string{""}},
		{Name: "root-bare-elvish", Shell: "elvish", Words: []string{""}},
		{Name: "root-bare-nushell", Shell: "nushell", Words: []string{""}},
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
