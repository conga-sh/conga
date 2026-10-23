package spec_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

type SubcommandTestCLI struct {
	Deploy mockCmd[DeployCmd] `aliases:"d"            summary:"Deploy application"`
	Status mockCmd[StatusCmd] `summary:"Check status"`
	Secret mockCmd[SecretCmd] `hidden:"true"          summary:"Secret subcommand"`
}

type DeployCmd struct {
	Env mockFlag[string] `enum:"staging,production" help:"Target environment"`
}

func (d *DeployCmd) Run(ctx context.Context) error { return nil }

type StatusCmd struct {
	Details mockFlag[bool] `help:"Show details"`
}

func (s *StatusCmd) Run(ctx context.Context) error { return nil }

type SecretCmd struct {
	Key mockFlag[string] `help:"Secret key"`
}

func (s *SecretCmd) Run(ctx context.Context) error { return nil }

type FlagsTestCLI struct {
	Port    mockFlag[int]      `aliases:"p"       help:"Port number" short:"p"`
	Debug   mockFlag[bool]     `help:"Debug mode" negatable:"true"   short:"d"`
	Format  mockFlag[string]   `default:"json"    enum:"json,yaml"   help:"Output format"`
	Private mockFlag[string]   `help:"Internal"   hidden:"true"`
	Tags    mockFlag[[]string] `help:"Tags"`
	Verbose mockFlag[int]      `help:"Verbosity"  short:"v"`
}

type XorGroupCLI struct {
	JSON mockFlag[bool] `help:"Output JSON" xor:"format"`
	YAML mockFlag[bool] `help:"Output YAML" xor:"format"`
	Text mockFlag[bool] `help:"Output Text" xor:"format"`
	Port mockFlag[int]  `help:"Port"`
}

type RepeatableFlagsCLI struct {
	Port    mockFlag[int]      `help:"Port"`
	Tag     mockFlag[[]string] `help:"Tags"`
	Verbose mockCounter        `help:"Verbosity"`
}

type FlagValueCLI struct {
	Format mockFlag[string]   `enum:"json,yaml,xml" help:"Output format"`
	Config mockFlag[string]   `help:"Config file"   predictor:"file:*.json,*.yaml"`
	OutDir mockFlag[string]   `help:"Output dir"    predictor:"dir"`
	Quiet  mockFlag[string]   `help:"Quiet"         predictor:"none"`
	Custom mockFlag[string]   `help:"Custom"        predictor:"custom-branches"`
	Tags   mockFlag[[]string] `enum:"api,cli,docs"  help:"Tags"`
	ShortF mockFlag[string]   `enum:"json,yaml"     help:"Short format"            short:"f"`
	Plain  mockFlag[string]   `help:"Plain string"`
}

// flagValueOptions registers FlagValueCLI's custom predictor so schema build
// accepts the predictor:"custom-branches" tag.
func flagValueOptions() spec.Options {
	return spec.Options{
		RegisteredPredictors: map[string]predict.Predictor{"custom-branches": predict.Values("main")},
	}
}

type PositionalArgsCLI struct {
	Environment mockArg[string]   `enum:"staging,production" help:"Target environment"`
	Directory   mockArg[string]   `help:"Target directory"   predictor:"dir"`
	ExtraFiles  mockArg[[]string] `help:"Extra files"        predictor:"file:*.go"`
}

type PredictableDomainCLI struct {
	Branch   mockFlag[customBranchType]   `help:"Target branch"`
	Branches mockFlag[[]customBranchType] `help:"Branch list"`
	Override mockFlag[customBranchType]   `help:"Override predictor" predictor:"none"`
	PosArg   mockArg[customBranchType]    `help:"Positional branch"`
}

// customTaggedPredictable demonstrates that declared custom field tags reach
// the TagAwarePredictable hook at schema build.
type customTaggedPredictable string

// Predict returns tag-dependent candidates: "tagged" when the field declares customcap.
func (c customTaggedPredictable) Predict(tags spec.FieldTags) predict.Predictor {
	if tags.Bool("customcap") {
		return predict.Values("tagged")
	}

	return predict.Values("plain")
}

// TagSpecs registers the customcap tag accepted by customTaggedPredictable.
func (c customTaggedPredictable) TagSpecs() []spec.TagSpec {
	return []spec.TagSpec{{Name: "customcap", Kind: spec.TagKindBool}}
}

type TaggedPredictableCLI struct {
	Mode mockFlag[customTaggedPredictable] `customcap:"true"     help:"Tag-driven mode"`
	Bare mockFlag[customTaggedPredictable] `help:"Untagged mode"`
}

type PredictableDualFormCLI struct {
	Plain  mockFlag[customBranchType]        `help:"Parameterless form"`
	Tagged mockFlag[customTaggedPredictable] `customcap:"true"          help:"Tag-aware form"`
}

type FileTagDomainCLI struct {
	Tool     mockFlag[conga.File] `executable:"true"      help:"Converter binary"`
	Hooks    mockFlag[conga.File] `help:"Hook scripts"    symlink:"true"`
	Plain    mockFlag[conga.File] `help:"Plain file"`
	Manifest mockFlag[conga.File] `glob:"*.yaml,Makefile" help:"Manifest file"`
	Override mockFlag[conga.File] `executable:"true"      help:"Override"         predictor:"dir"`
}

type FilteringDomainCLI struct {
	ComposeFile mockFlag[customComposeFileType] `help:"Compose file"`
	RepoDir     mockFlag[customRepoDirType]     `help:"Repo dir"`
	DataPath    mockFlag[customDatasetPathType] `help:"Dataset path"`
	ExecBinary  mockFlag[string]                `help:"Exec binary"  predictor:"exec"`
	SymlinkPath mockFlag[string]                `help:"Symlink"      predictor:"symlink"`
}

type DeprecatedCLI struct {
	Legacy mockFlag[string] `deprecated:"use --modern instead" help:"Legacy flag"`
	Modern mockFlag[string] `help:"Modern flag"`
	Old    mockCmd[OldCmd]  `deprecated:"use new instead"      summary:"Old command"`
	New    mockCmd[NewCmd]  `summary:"New command"`
}

type OldCmd struct {
	Value mockArg[string] `help:"Old value"`
}

func (o *OldCmd) Run(ctx context.Context) error { return nil }

type NewCmd struct {
	Value mockArg[string] `help:"New value"`
}

func (n *NewCmd) Run(ctx context.Context) error { return nil }

type DeprecatedGroupCLI struct {
	Legacy mockCmd[LegacyCmd] `deprecated:"removed"     group:"legacy" summary:"Legacy command"`
	Modern mockCmd[ModernCmd] `summary:"Modern command"`
}

type LegacyCmd struct{}

func (l *LegacyCmd) Run(ctx context.Context) error { return nil }

type ModernCmd struct{}

func (m *ModernCmd) Run(ctx context.Context) error { return nil }

func TestCompletion_Subcommands(t *testing.T) {
	t.Parallel()

	t.Run("suggests all non-hidden subcommands and flags on empty input", func(t *testing.T) {
		t.Parallel()

		var cli SubcommandTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"deploy", "d", "status", "--help", "--version", "--why"}, values)
	})

	t.Run("filters subcommands by prefix", func(t *testing.T) {
		t.Parallel()

		var cli SubcommandTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"dep"}, spec.CompleteOptions{})

		assert.Equal(t, 1, len(invoked.Items))
		assert.Equal(t, "deploy", invoked.Items[0].Value)
		assert.Equal(t, "Deploy application", invoked.Items[0].Description)
	})

	t.Run("completes flags inside routed subcommand", func(t *testing.T) {
		t.Parallel()

		var cli SubcommandTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"deploy", "--"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--env", "--help", "--why"}, values)
	})
}

func TestCompletion_FlagNames(t *testing.T) {
	t.Parallel()

	t.Run("completes long, short, alias, and negatable flags", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--port", "--p", "--debug", "--no-debug", "--format", "--tags", "--verbose", "--help", "--version", "--why", "--completion"}, values)
	})

	t.Run("filters flags matching prefix", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--no"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--no-debug"}, values)
	})

	t.Run("completes short flags with dash prefix", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"-"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--port", "--p", "--debug", "--no-debug", "--format", "--tags", "--verbose", "--help", "--version", "--why", "--completion", "-p", "-d", "-v"}, values)
	})
}

func TestCompletion_NegationGuard(t *testing.T) {
	t.Parallel()

	t.Run("no-prefixed token does not consume non-negatable flags", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		// Verbose is a value-taking int flag without the negatable tag: the
		// --no-verbose token must be ignored entirely, leaving --verbose
		// available for later completion.
		invoked := spec.Complete(cmdSpec, []string{"--no-verbose", "--"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "--verbose"))
		assert.True(t, slices.Contains(values, "--no-debug"))
	})

	t.Run("negated token consumes its negatable bool flag", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--no-debug", "--"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.False(t, slices.Contains(values, "--debug"))
		assert.False(t, slices.Contains(values, "--no-debug"))
		assert.True(t, slices.Contains(values, "--verbose"))
	})
}

func TestCompletion_WhyFormats(t *testing.T) {
	t.Parallel()

	t.Run("json disabled offers the plain flag", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--why"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--why"}, values)
	})

	t.Run("json disabled offers the single text value for the attached flag", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--why="}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--why=text"}, values)
	})

	t.Run("json enabled replaces the plain flag with format candidates", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{WhyJSONEnabled: true})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--why"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--why=text", "--why=json"}, values)
		assert.Equal(t, "Show resolved field values and where they came from (values redacted unless tagged safe)", invoked.Items[0].Description)
	})

	t.Run("json enabled offers both formats for the attached flag", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{WhyJSONEnabled: true})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--why="}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--why=text", "--why=json"}, values)
	})

	t.Run("json enabled filters formats by value prefix", func(t *testing.T) {
		t.Parallel()

		var cli FlagsTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{WhyJSONEnabled: true})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--why=j"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--why=json"}, values)
	})

	t.Run("synthesized subcommand inherits root formats for flag names", func(t *testing.T) {
		t.Parallel()

		var cli BuiltinTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{WhyJSONEnabled: true})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"completion", "--why"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--why=text", "--why=json"}, values)
	})

	t.Run("synthesized subcommand inherits root formats for attached values", func(t *testing.T) {
		t.Parallel()

		var cli BuiltinTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{WhyJSONEnabled: true})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"completion", "--why="}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--why=text", "--why=json"}, values)
	})

	t.Run("disabled why hides the flag from synthesized subcommand names", func(t *testing.T) {
		t.Parallel()

		var cli BuiltinTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{DisableWhy: true})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"completion", "--why"}, spec.CompleteOptions{})

		assert.False(t, slices.ContainsFunc(invoked.Items, func(item predict.Item) bool {
			return strings.Contains(item.Value, "why")
		}))
	})

	t.Run("disabled why yields no attached value candidates on synthesized subcommand", func(t *testing.T) {
		t.Parallel()

		var cli BuiltinTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{DisableWhy: true})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"completion", "--why="}, spec.CompleteOptions{})

		assert.False(t, slices.ContainsFunc(invoked.Items, func(item predict.Item) bool {
			return strings.Contains(item.Value, "why")
		}))
	})
}

func TestCompletion_XorSuppression(t *testing.T) {
	t.Parallel()

	t.Run("shows all XOR options before selection", func(t *testing.T) {
		t.Parallel()

		var cli XorGroupCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--json", "--yaml", "--text", "--port", "--help", "--version", "--why", "--completion"}, values)
	})

	t.Run("suppresses conflicting XOR flags when one is supplied", func(t *testing.T) {
		t.Parallel()

		var cli XorGroupCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--json", "--"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--port", "--help", "--version", "--why", "--completion"}, values)
	})
}

func TestCompletion_NonRepeatableSuppression(t *testing.T) {
	t.Parallel()

	t.Run("suppresses scalar flag but preserves slice and counter flags", func(t *testing.T) {
		t.Parallel()

		var cli RepeatableFlagsCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--port", "8080", "--tag", "api", "--verbose", "--"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--tag", "--verbose", "--help", "--version", "--why", "--completion"}, values)
	})
}

func TestCompletion_DeprecatedFiltering(t *testing.T) {
	t.Parallel()

	t.Run("excludes deprecated flags from flag name suggestions", func(t *testing.T) {
		t.Parallel()

		var cli DeprecatedCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--modern", "--help", "--version", "--why"}, values)
	})

	t.Run("excludes deprecated subcommands from suggestions", func(t *testing.T) {
		t.Parallel()

		var cli DeprecatedCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"new", "--modern", "--help", "--version", "--why"}, values)
	})

	t.Run("deprecated grouped subcommand does not skew sibling tags", func(t *testing.T) {
		t.Parallel()

		var cli DeprecatedGroupCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"modern", "--help", "--version", "--why"}, values)

		for _, item := range invoked.Items {
			if item.Value == "modern" {
				assert.Equal(t, "commands", item.Tag)
			}
		}
	})
}

func TestCompletion_FlagValues(t *testing.T) {
	t.Parallel()

	t.Run("completes enum values after separate flag", func(t *testing.T) {
		t.Parallel()

		var cli FlagValueCLI

		cmdSpec, err := buildSchema(&cli, "app", flagValueOptions())
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--format", ""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"json", "yaml", "xml"}, values)
	})

	t.Run("completes attached enum values with prefix", func(t *testing.T) {
		t.Parallel()

		var cli FlagValueCLI

		cmdSpec, err := buildSchema(&cli, "app", flagValueOptions())
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--format=y"}, spec.CompleteOptions{})

		assert.Equal(t, 1, len(invoked.Items))
		assert.Equal(t, "--format=yaml", invoked.Items[0].Value)
	})

	t.Run("completes short attached enum values with prefix", func(t *testing.T) {
		t.Parallel()

		var cli FlagValueCLI

		cmdSpec, err := buildSchema(&cli, "app", flagValueOptions())
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"-f=j"}, spec.CompleteOptions{})

		assert.Equal(t, 1, len(invoked.Items))
		assert.Equal(t, "-f=json", invoked.Items[0].Value)
	})

	t.Run("accesses multi-value flag slices in CompletionContext", func(t *testing.T) {
		t.Parallel()

		var cli FlagValueCLI

		var recordedTags []string

		customAction := predict.Callback(func(c predict.Context) predict.Predictor {
			recordedTags = c.FlagValues("tags")

			return predict.Values("done")
		})

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{
			RegisteredPredictors: map[string]predict.Predictor{"custom-branches": customAction},
		})
		assert.NoError(t, err)

		_ = spec.Complete(cmdSpec, []string{"--tags", "api", "--tags", "cli", "--custom", ""}, spec.CompleteOptions{
			RegisteredPredictors: map[string]predict.Predictor{"custom-branches": customAction},
		})

		assert.Equal(t, []string{"api", "cli"}, recordedTags)
	})

	t.Run("accesses short flags and aliases in CompletionContext", func(t *testing.T) {
		t.Parallel()

		type AliasCLI struct {
			Env    mockFlag[string] `aliases:"environment,target-env" help:"Env"             short:"e"`
			Custom mockFlag[string] `help:"Custom"                    predictor:"custom-act"`
		}

		var cli AliasCLI

		var (
			envByCanonical string
			envByAlias     string
			envByShort     string
			hasShort       bool
			hasAlias       bool
		)

		customAction := predict.Callback(func(c predict.Context) predict.Predictor {
			envByCanonical = c.FlagValue("env", "")
			envByAlias = c.FlagValue("target-env", "")
			envByShort = c.FlagValue("e", "")
			hasShort = c.HasFlag("e")
			hasAlias = c.HasFlag("environment")

			return predict.Values("ok")
		})

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{
			RegisteredPredictors: map[string]predict.Predictor{"custom-act": customAction},
		})
		assert.NoError(t, err)

		_ = spec.Complete(cmdSpec, []string{"-e", "staging", "--custom", ""}, spec.CompleteOptions{
			RegisteredPredictors: map[string]predict.Predictor{"custom-act": customAction},
		})

		assert.Equal(t, "staging", envByCanonical)
		assert.Equal(t, "staging", envByAlias)
		assert.Equal(t, "staging", envByShort)
		assert.True(t, hasShort)
		assert.True(t, hasAlias)
	})

	t.Run("completes built-in file filter directive", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte("{}"), 0600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.yaml"), []byte("name: app"), 0600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# Readme"), 0600))

		var cli FlagValueCLI

		cmdSpec, err := buildSchema(&cli, "app", flagValueOptions())
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--config", ""}, spec.CompleteOptions{Dir: tmpDir})

		assert.True(t, invoked.Directive.Has(predict.DirectiveNoFileCompletion))

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "config.json"))
		assert.True(t, slices.Contains(values, "app.yaml"))
		assert.False(t, slices.Contains(values, "readme.md"))
	})

	t.Run("completes built-in directory directive", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "output-build"), 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("data"), 0600))

		var cli FlagValueCLI

		cmdSpec, err := buildSchema(&cli, "app", flagValueOptions())
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--out-dir", ""}, spec.CompleteOptions{Dir: tmpDir})

		assert.True(t, invoked.Directive.Has(predict.DirectiveNoSpace))

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "output-build/"))
		assert.False(t, slices.Contains(values, "file.txt"))
	})

	t.Run("completes built-in none directive", func(t *testing.T) {
		t.Parallel()

		var cli FlagValueCLI

		cmdSpec, err := buildSchema(&cli, "app", flagValueOptions())
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--quiet", ""}, spec.CompleteOptions{})

		assert.Equal(t, predict.DirectiveNoFileCompletion, invoked.Directive)
		assert.Zero(t, invoked.Items)
	})

	t.Run("unannotated flag defaults to none without file completion", func(t *testing.T) {
		t.Parallel()

		var cli FlagValueCLI

		cmdSpec, err := buildSchema(&cli, "app", flagValueOptions())
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--plain", ""}, spec.CompleteOptions{})

		assert.Equal(t, predict.DirectiveNoFileCompletion, invoked.Directive)
		assert.Zero(t, invoked.Items)
	})

	t.Run("invokes custom dynamic predictor with context", func(t *testing.T) {
		t.Parallel()

		var cli FlagValueCLI

		customAction := predict.Callback(func(c predict.Context) predict.Predictor {
			return predict.Values("main", "feature-"+c.FlagValue("format", "default"))
		})

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{
			RegisteredPredictors: map[string]predict.Predictor{"custom-branches": customAction},
		})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--format", "json", "--custom", ""}, spec.CompleteOptions{
			RegisteredPredictors: map[string]predict.Predictor{"custom-branches": customAction},
		})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"main", "feature-json"}, values)
	})

	t.Run("incrementally completes comma-separated slice flag values", func(t *testing.T) {
		t.Parallel()

		var cli FlagValueCLI

		cmdSpec, err := buildSchema(&cli, "app", flagValueOptions())
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--tags", "api,"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"api,cli", "api,docs"}, values)
		assert.True(t, invoked.Directive.Has(predict.DirectiveNoSpace))
	})

	t.Run("deduplicates repeated slice flag invocations", func(t *testing.T) {
		t.Parallel()

		var cli FlagValueCLI

		cmdSpec, err := buildSchema(&cli, "app", flagValueOptions())
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"--tags", "api", "--tags", ""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"cli", "docs"}, values)
	})
}

func TestCompletion_PositionalArguments(t *testing.T) {
	t.Parallel()

	t.Run("completes first positional argument from enum and flags", func(t *testing.T) {
		t.Parallel()

		var cli PositionalArgsCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"staging", "production", "--help", "--version", "--why", "--completion"}, values)
	})

	t.Run("completes second positional argument with directory predictor", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.Mkdir(filepath.Join(tmpDir, "target-dir"), 0755))

		var cli PositionalArgsCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"staging", ""}, spec.CompleteOptions{Dir: tmpDir})

		assert.True(t, invoked.Directive.Has(predict.DirectiveNoSpace))

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "target-dir/"))
	})

	t.Run("completes variadic trailing slice positional arguments", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "extra1.go"), []byte("package main"), 0600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "extra2.go"), []byte("package main"), 0600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "data.txt"), []byte("data"), 0600))

		var cli PositionalArgsCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{"staging", "/tmp", "extra1.go", ""}, spec.CompleteOptions{Dir: tmpDir})

		assert.True(t, invoked.Directive.Has(predict.DirectiveNoFileCompletion))

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "extra1.go"))
		assert.True(t, slices.Contains(values, "extra2.go"))
		assert.False(t, slices.Contains(values, "data.txt"))
	})

	t.Run("completes unconstrained positional argument with active help hint and flags", func(t *testing.T) {
		t.Parallel()

		type FreeArgCLI struct {
			Env     mockFlag[string] `help:"Environment"            short:"e"`
			Service mockArg[string]  `help:"Service name to deploy"`
		}

		var cli FreeArgCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		invoked := spec.Complete(cmdSpec, []string{""}, spec.CompleteOptions{})

		assert.Equal(t, "<service>: Service name to deploy", invoked.Usage)
		assert.True(t, len(invoked.Items) > 0)

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"--env", "--help", "--version", "--why", "--completion", "-e"}, values)
	})

	t.Run("completes subcommand positional argument evaluating both root and subcommand flags", func(t *testing.T) {
		t.Parallel()

		type SubCmd struct {
			Namespace mockFlag[string] `default:"backend" help:"Namespace" short:"n"`
			Target    mockArg[string]  `help:"Target pod" predictor:"pods"`
		}

		type RootCLI struct {
			Profile mockFlag[string] `default:"staging"      help:"Profile" short:"p"`
			Exec    mockCmd[SubCmd]  `summary:"Exec command"`
		}

		podRegistry := map[string]map[string][]string{
			"production": {
				"frontend": {"web-prod-1", "web-prod-2"},
				"backend":  {"api-prod-8a", "auth-prod-7c"},
				"database": {"pg-primary-0", "redis-prod-node-1"},
			},
			"staging": {
				"frontend": {"web-stage-1", "web-stage-2"},
				"backend":  {"api-stage-alpha", "auth-stage-beta"},
				"database": {"pg-stage-0", "redis-stage-0"},
			},
		}

		opts := spec.CompleteOptions{
			RegisteredPredictors: map[string]predict.Predictor{
				"pods": predict.Callback(func(c predict.Context) predict.Predictor {
					prof := c.FlagValue("profile", "staging")
					ns := c.FlagValue("namespace", "backend")

					if nsMap, ok := podRegistry[prof]; ok {
						if pods, ok := nsMap[ns]; ok {
							return predict.Values(pods...)
						}
					}

					return predict.None()
				}),
			},
		}

		var cli RootCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{
			RegisteredPredictors: opts.RegisteredPredictors,
		})
		assert.NoError(t, err)

		// 1. Default flags (staging + backend)
		invoked := spec.Complete(cmdSpec, []string{"exec", ""}, opts)

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "api-stage-alpha"))
		assert.True(t, slices.Contains(values, "auth-stage-beta"))
		assert.False(t, slices.Contains(values, "api-prod-8a"))

		// 2. Overridden subcommand flag (--namespace frontend)
		invoked = spec.Complete(cmdSpec, []string{"exec", "--namespace", "frontend", ""}, opts)

		values = slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "web-stage-1"))
		assert.True(t, slices.Contains(values, "web-stage-2"))
		assert.False(t, slices.Contains(values, "api-stage-alpha"))

		// 3. Overridden root flag before subcommand (--profile production)
		invoked = spec.Complete(cmdSpec, []string{"--profile", "production", "exec", ""}, opts)

		values = slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "api-prod-8a"))
		assert.True(t, slices.Contains(values, "auth-prod-7c"))
		assert.False(t, slices.Contains(values, "api-stage-alpha"))

		// 4. Overridden root and subcommand flags together
		invoked = spec.Complete(cmdSpec, []string{"exec", "--namespace", "database", "--profile", "production", ""}, opts)

		values = slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "pg-primary-0"))
		assert.True(t, slices.Contains(values, "redis-prod-node-1"))
		assert.False(t, slices.Contains(values, "pg-stage-0"))
	})
}

func TestCompletion_PredictableDomainTypes(t *testing.T) {
	t.Parallel()

	var cli PredictableDomainCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
	assert.NoError(t, err)

	t.Run("flag with Predictable type completes candidates", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--branch", ""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"main", "feature/login", "bugfix/123"}, values)
	})

	t.Run("slice flag with Predictable type completes comma-separated candidates", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--branches", "main,"}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"main,feature/login", "main,bugfix/123"}, values)
	})

	t.Run("explicit predictor tag overrides Predictable interface", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--override", ""}, spec.CompleteOptions{})

		assert.Equal(t, predict.DirectiveNoFileCompletion, invoked.Directive)
		assert.Zero(t, invoked.Items)
	})

	t.Run("positional argument with Predictable type completes candidates", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "main"))
		assert.True(t, slices.Contains(values, "feature/login"))
	})
}

// TestCompletion_FileCapabilityTagsTailorPredictor verifies that File fields
// complete according to their declared capability tags: executables for
// executable fields, symlinks for symlink fields, glob-filtered file paths
// for glob fields, and file paths otherwise, with explicit predictor tags
// keeping precedence.
func TestCompletion_FileCapabilityTagsTailorPredictor(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "tool.sh"), []byte("#!/bin/sh\n"), 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "plain.txt"), []byte("x"), 0o644))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "deploy.yaml"), []byte("x"), 0o644))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("all:"), 0o644))
	assert.NoError(t, os.Symlink("plain.txt", filepath.Join(tmpDir, "link.txt")))
	assert.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0o755))

	var cli FileTagDomainCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
	assert.NoError(t, err)

	dirPrefix := tmpDir + "/"

	t.Run("executable tag completes executables", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--tool", dirPrefix}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, dirPrefix+"tool.sh"))
		assert.False(t, slices.Contains(values, dirPrefix+"plain.txt"))
	})

	t.Run("symlink tag completes symlinks", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--hooks", dirPrefix}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, dirPrefix+"link.txt"))
		assert.False(t, slices.Contains(values, dirPrefix+"plain.txt"))
	})

	t.Run("plain file tag completes files", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--plain", dirPrefix}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, dirPrefix+"plain.txt"))
	})

	t.Run("glob tag filters file candidates and keeps directories", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--manifest", dirPrefix}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, dirPrefix+"deploy.yaml"))
		assert.True(t, slices.Contains(values, dirPrefix+"Makefile"))
		assert.True(t, slices.Contains(values, dirPrefix+"subdir/"), "directories must be kept for traversal")
		assert.False(t, slices.Contains(values, dirPrefix+"plain.txt"))
		assert.False(t, slices.Contains(values, dirPrefix+"tool.sh"))
		assert.False(t, slices.Contains(values, dirPrefix+"link.txt"))
	})

	t.Run("explicit predictor tag keeps precedence over capability tags", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--override", dirPrefix}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, dirPrefix+"subdir/"))
		assert.False(t, slices.Contains(values, dirPrefix+"tool.sh"))
	})
}

// TestCompletion_TaggedPredictableReceivesFieldTags verifies that declared
// custom field tags reach the TagAwarePredictable hook at schema build, so
// custom types can tailor completion to their registered capabilities.
func TestCompletion_TaggedPredictableReceivesFieldTags(t *testing.T) {
	t.Parallel()

	var cli TaggedPredictableCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
	assert.NoError(t, err)

	t.Run("field declaring the custom tag receives it", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--mode", ""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"tagged"}, values)
	})

	t.Run("field without the tag receives empty tags", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--bare", ""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"plain"}, values)
	})
}

// TestCompletion_PredictableDualForms verifies that the engine honors both
// Predictable hook shapes in one schema: the parameterless conga.Predictable
// form and the tags-tailored conga.TagAwarePredictable form.
func TestCompletion_PredictableDualForms(t *testing.T) {
	t.Parallel()

	var cli PredictableDualFormCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
	assert.NoError(t, err)

	t.Run("parameterless Predictable form completes candidates", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--plain", ""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"main", "feature/login", "bugfix/123"}, values)
	})

	t.Run("tag-aware TagAwarePredictable form completes candidates", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--tagged", ""}, spec.CompleteOptions{})

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.Equal(t, []string{"tagged"}, values)
	})
}

func TestCompletion_FilteringInterfacesAndBuiltinPredictors(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte("services: {}"), 0600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("txt"), 0600))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "table.csv"), []byte("a,b"), 0600))

	gitDir := filepath.Join(tmpDir, "myrepo")
	assert.NoError(t, os.MkdirAll(filepath.Join(gitDir, ".git"), 0755))

	plainDir := filepath.Join(tmpDir, "plaindir")
	assert.NoError(t, os.MkdirAll(plainDir, 0755))

	datasetDir := filepath.Join(tmpDir, "dataset")
	assert.NoError(t, os.MkdirAll(datasetDir, 0755))

	execBin := filepath.Join(tmpDir, "runner.sh")
	assert.NoError(t, os.WriteFile(execBin, []byte("#!/bin/sh\n"), 0755))

	linkPath := filepath.Join(tmpDir, "link.yaml")
	if err := os.Symlink(filepath.Join(tmpDir, "compose.yaml"), linkPath); err != nil {
		t.Skip("symlinks not supported")
	}

	repoLink := filepath.Join(tmpDir, "repo-link")
	if err := os.Symlink(gitDir, repoLink); err != nil {
		t.Skip("symlinks not supported")
	}

	var cli FilteringDomainCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
	assert.NoError(t, err)

	opts := spec.CompleteOptions{
		Dir: tmpDir,
		Env: map[string]string{
			"PATH": tmpDir,
		},
	}

	t.Run("FileFilterable completes matching files and keeps directories", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--compose-file", ""}, opts)

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "compose.yaml"))
		assert.True(t, slices.Contains(values, "myrepo/"))
		assert.False(t, slices.Contains(values, "other.txt"))
	})

	t.Run("DirFilterable completes matching directories only", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--repo-dir", ""}, opts)

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "myrepo/"))
		assert.True(t, slices.Contains(values, "repo-link/"), "symlinked directories must be followed")
		assert.False(t, slices.Contains(values, "plaindir/"))
		assert.False(t, slices.Contains(values, "compose.yaml"))
		assert.True(t, invoked.Directive.Has(predict.DirectiveNoFileCompletion))
		assert.True(t, invoked.Directive.Has(predict.DirectiveNoSpace))
	})

	t.Run("PathFilterable completes evaluated files and directories", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--data-path", ""}, opts)

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "table.csv"))
		assert.True(t, slices.Contains(values, "dataset/"))
		assert.False(t, slices.Contains(values, "plaindir/"))
		assert.False(t, slices.Contains(values, "compose.yaml"))
	})

	t.Run("predictor:exec completes executables from PATH", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--exec-binary", ""}, opts)

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "runner.sh"))
		assert.False(t, slices.Contains(values, "other.txt"))
	})

	t.Run("predictor:symlink completes symlinks", func(t *testing.T) {
		t.Parallel()

		invoked := spec.Complete(cmdSpec, []string{"--symlink-path", ""}, opts)

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.True(t, slices.Contains(values, "link.yaml"))
		assert.False(t, slices.Contains(values, "other.txt"))
	})

	t.Run("built-in predictors cannot be shadowed by registered predictors", func(t *testing.T) {
		t.Parallel()

		optsWithShadow := spec.CompleteOptions{
			Dir: tmpDir,
			RegisteredPredictors: map[string]predict.Predictor{
				"dir":     predict.Values("shadowed_dir"),
				"symlink": predict.Values("shadowed_symlink"),
			},
		}

		invoked := spec.Complete(cmdSpec, []string{"--repo-dir", ""}, optsWithShadow)

		values := slicex.Map(invoked.Items, func(item predict.Item) string { return item.Value })

		assert.False(t, slices.Contains(values, "shadowed_dir"))
		assert.True(t, slices.Contains(values, "myrepo/"))
	})
}

type customBranchType string

func (c customBranchType) Predict() predict.Predictor {
	return predict.Values("main", "feature/login", "bugfix/123")
}

type customComposeFileType string

func (c *customComposeFileType) MatchFile(path string, entry os.DirEntry) bool {
	return strings.HasPrefix(entry.Name(), "compose") && strings.HasSuffix(entry.Name(), ".yaml")
}

type customRepoDirType string

func (r *customRepoDirType) MatchDir(path string, entry os.DirEntry) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))

	return err == nil
}

type customDatasetPathType string

func (d *customDatasetPathType) MatchPath(path string, entry os.DirEntry) bool {
	return strings.HasSuffix(entry.Name(), ".csv") || (entry.IsDir() && entry.Name() == "dataset")
}

type analysisCLI struct {
	A       mockFlag[bool]   `name:"alpha"   short:"a"`
	Dir     mockFlag[string] `name:"dir"     short:"d"`
	Verbose mockCounter      `name:"verbose" short:"v"`
	Control mockFlag[string] `name:"control"`
	Trail   mockDashArgs     `name:"trail"`

	Deploy mockCmd[analysisDeployCLI] `summary:"Deploy"`
}

type analysisDeployCLI struct {
	Env mockFlag[string] `name:"env"`
}

type analysisRawCLI struct {
	First mockArg[string]
	Raw   mockRawArgs
}

type analysisOverflowCLI struct {
	Verbose mockFlag[bool] `name:"verbose"`
	Raw     mockRawArgs
}

type analysisLeafRawCLI struct {
	Deploy mockCmd[analysisLeafRawDeployCLI]
}

type analysisLeafRawDeployCLI struct {
	Raw mockRawArgs
}

func analysisFlagValues(t *testing.T, values map[*spec.Flag][]string, name string) []string {
	t.Helper()

	for flag, flagValues := range values {
		if flag.Name == name {
			return flagValues
		}
	}

	return nil
}

func TestAnalyzeCompletion(t *testing.T) {
	t.Parallel()

	t.Run("short cluster attached value", func(t *testing.T) {
		t.Parallel()

		var cli analysisCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"-vd./service", "deploy", ""})

		assert.Equal(t, []string{"1"}, analysisFlagValues(t, analysis.FlagValues, "verbose"))
		assert.Equal(t, []string{"./service"}, analysisFlagValues(t, analysis.FlagValues, "dir"))
		assert.Equal(t, "deploy", analysis.CommandChain[len(analysis.CommandChain)-1].Name)
		assert.Equal(t, 0, len(analysis.PositionalArgs))
	})

	t.Run("bool does not consume positional", func(t *testing.T) {
		t.Parallel()

		var cli analysisCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"-a", "deploy", "--env", "dev", ""})

		assert.Equal(t, []string{"true"}, analysisFlagValues(t, analysis.FlagValues, "alpha"))
		assert.Equal(t, []string{"dev"}, analysisFlagValues(t, analysis.FlagValues, "env"))
		assert.Equal(t, "deploy", analysis.CommandChain[len(analysis.CommandChain)-1].Name)
	})

	t.Run("unknown flag is ignored", func(t *testing.T) {
		t.Parallel()

		var cli analysisCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"--no-aliases", "deploy", ""})

		assert.Equal(t, "deploy", analysis.CommandChain[len(analysis.CommandChain)-1].Name)
		assert.Equal(t, 0, len(analysis.FlagValues))
	})

	t.Run("terminator separates dash args", func(t *testing.T) {
		t.Parallel()

		var cli analysisCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"--dir", "/tmp", "build", "--", "one", "two", ""})

		assert.Equal(t, []string{"build"}, analysis.PositionalArgs)
		assert.Equal(t, []string{"one", "two"}, analysis.DashArgs)
		assert.Equal(t, []string{"/tmp"}, analysisFlagValues(t, analysis.FlagValues, "dir"))
	})

	t.Run("raw args absorb positional overflow", func(t *testing.T) {
		t.Parallel()

		var cli analysisOverflowCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"run", "extra", ""})

		assert.Equal(t, 0, len(analysis.PositionalArgs))
		assert.Equal(t, []string{"run", "extra"}, analysis.RawArgs)
	})

	t.Run("raw fallback captures later flags", func(t *testing.T) {
		t.Parallel()

		var cli analysisOverflowCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"run", "--verbose", ""})

		assert.Equal(t, []string{"run", "--verbose"}, analysis.RawArgs)
	})

	t.Run("declared arguments fill before raw overflow", func(t *testing.T) {
		t.Parallel()

		var cli analysisRawCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"one", "two", "three", ""})

		assert.Equal(t, []string{"one"}, analysis.PositionalArgs)
		assert.Equal(t, []string{"two", "three"}, analysis.RawArgs)
	})

	t.Run("terminator fills declared arguments before raw overflow", func(t *testing.T) {
		t.Parallel()

		var cli analysisRawCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"--", "one", "two", "three", ""})

		assert.Equal(t, []string{"one"}, analysis.PositionalArgs)
		assert.Equal(t, []string{"two", "three"}, analysis.RawArgs)
	})

	t.Run("negative number binds as positional value", func(t *testing.T) {
		t.Parallel()

		var cli analysisRawCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"-10", ""})

		assert.Equal(t, []string{"-10"}, analysis.PositionalArgs)
		assert.Equal(t, 0, len(analysis.RawArgs))
	})

	t.Run("negative number overflows to raw args", func(t *testing.T) {
		t.Parallel()

		var cli analysisOverflowCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"-10", ""})

		assert.Equal(t, 0, len(analysis.PositionalArgs))
		assert.Equal(t, []string{"-10"}, analysis.RawArgs)
	})

	t.Run("unknown shorthand overflows to raw args", func(t *testing.T) {
		t.Parallel()

		var cli analysisOverflowCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"-x", ""})

		assert.Equal(t, []string{"-x"}, analysis.RawArgs)
	})

	t.Run("raw fallback uses routed leaf", func(t *testing.T) {
		t.Parallel()

		var cli analysisLeafRawCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(cmdSpec, []string{"--unknown-flag", "deploy", ""})

		assert.Equal(t, "deploy", analysis.CommandChain[len(analysis.CommandChain)-1].Name)
		assert.Equal(t, []string{"--unknown-flag"}, analysis.RawArgs)
		assert.Equal(t, 0, len(analysis.PositionalArgs))
	})
}
