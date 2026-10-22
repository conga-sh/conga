package conga_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/hcl"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
)

var (
	// Cascading tier payloads: the same server key set split across system,
	// user, and local files.
	//go:embed testdata/fixtures/cascade_local.toml
	fixtureCascadeLocal []byte

	//go:embed testdata/fixtures/cascade_sys.toml
	fixtureCascadeSys []byte

	//go:embed testdata/fixtures/cascade_user.toml
	fixtureCascadeUser []byte

	// The same server payload in four encodings; the decoder matrix asserts
	// each decoder yields identical bindings.
	//go:embed testdata/fixtures/decoder_test.hcl
	fixtureDecoderHCL []byte

	//go:embed testdata/fixtures/decoder_test.json
	fixtureDecoderJSON []byte

	//go:embed testdata/fixtures/decoder_test.toml
	fixtureDecoderTOML []byte

	//go:embed testdata/fixtures/decoder_test.yaml
	fixtureDecoderYAML []byte

	//go:embed testdata/fixtures/excluded_field.toml
	fixtureExcludedField []byte

	// Cascading map payloads: the tags map split across system, user, and
	// local tiers, plus the subcommand-scoped tier pair.
	//go:embed testdata/fixtures/map_cascade_local.toml
	fixtureMapCascadeLocal []byte

	//go:embed testdata/fixtures/map_cascade_sub_local.toml
	fixtureMapCascadeSubLocal []byte

	//go:embed testdata/fixtures/map_cascade_sub_sys.toml
	fixtureMapCascadeSubSys []byte

	//go:embed testdata/fixtures/map_cascade_sys.toml
	fixtureMapCascadeSys []byte

	//go:embed testdata/fixtures/map_cascade_user.toml
	fixtureMapCascadeUser []byte

	// Multi-flag first-hit payloads for the lower- and higher-precedence tiers.
	//go:embed testdata/fixtures/multi_flag_first.toml
	fixtureMultiFlagFirst []byte

	//go:embed testdata/fixtures/multi_flag_second.toml
	fixtureMultiFlagSecond []byte

	// The same key set in four encodings; native map binding must agree
	// across all decoders.
	//go:embed testdata/fixtures/native_map.hcl
	fixtureNativeMapHCL []byte

	//go:embed testdata/fixtures/native_map.json
	fixtureNativeMapJSON []byte

	//go:embed testdata/fixtures/native_map.toml
	fixtureNativeMapTOML []byte

	//go:embed testdata/fixtures/native_map.yaml
	fixtureNativeMapYAML []byte

	//go:embed testdata/fixtures/nested_namespace_map.yaml
	fixtureNestedNamespaceMap []byte

	//go:embed testdata/fixtures/nested_native_map.toml
	fixtureNestedNativeMap []byte

	//go:embed testdata/fixtures/nested_subcommands.toml
	fixtureNestedSubcommands []byte

	//go:embed testdata/fixtures/preparsed.toml
	fixturePreparsed []byte

	// Cascading tier payloads shared by the multi-flag tier tests.
	//go:embed testdata/fixtures/tier_local.toml
	fixtureTierLocal []byte

	//go:embed testdata/fixtures/tier_sys.toml
	fixtureTierSys []byte

	//go:embed testdata/fixtures/unknown_field.toml
	fixtureUnknownField []byte
)

type MultiFormatServerCLI struct {
	ConfigFile  conga.ConfigFile                  `multi:"true"`
	Port        conga.OptionalFlag[int]           `default:"8080"      help:"Server port"  name:"port"`
	Host        conga.OptionalFlag[string]        `default:"0.0.0.0"   help:"Host bind"    name:"host"`
	ClusterName conga.OptionalFlag[string]        `help:"Cluster name" name:"cluster-name"`
	Timeout     conga.OptionalFlag[time.Duration] `default:"5s"        help:"Timeout"      name:"timeout"`
	Debug       conga.OptionalFlag[bool]          `help:"Debug mode"   name:"debug"`
	SecretToken conga.OptionalFlag[string]        `config:"api_secret" help:"Secret token" name:"token"`
	InternalKey conga.OptionalFlag[string]        `config:"-"          help:"CLI only"     name:"internal-key"`
}

func (m *MultiFormatServerCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

// Subcommands hierarchy for testing nested config tables.
type HierarchicalRootCLI struct {
	Verbose conga.OptionalFlag[bool]        `help:"Global verbose"     short:"v"`
	Cluster conga.Command[ClusterConfigCmd] `summary:"Cluster command"`
}

type ClusterConfigCmd struct {
	Region conga.OptionalFlag[string]   `default:"us-east-1"    help:"Region"`
	Node   conga.Command[NodeConfigCmd] `summary:"Node command"`
}

type NodeConfigCmd struct {
	Zone  conga.OptionalFlag[string] `help:"Zone"`
	Count conga.OptionalFlag[int]    `default:"1" help:"Count"`
}

func (n *NodeConfigCmd) Run(_ context.Context, _ *conga.Session) error { return nil }

type MultiConfigFileTestCLI struct {
	Configs conga.ConfigFile           `multi:"true"        short:"c"`
	Port    conga.OptionalFlag[int]    `default:"8080"`
	Host    conga.OptionalFlag[string] `default:"localhost"`
	Env     conga.OptionalFlag[string] `default:"dev"`
	Loaded  []string
}

func (m *MultiConfigFileTestCLI) Run(_ context.Context, session *conga.Session) error {
	m.Loaded = session.LoadedConfigFiles()

	return nil
}

func TestConfig_UnregisteredDecoderRejection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.customext")
	err := os.WriteFile(cfgPath, []byte("port = 9000\n"), 0o600)
	assert.NoError(t, err)

	app := conga.New[MultiFormatServerCLI]().
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
}

func TestConfig_JSONDecoder(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(cfgPath, fixtureDecoderJSON, 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9001, cli.Port.Get())
	assert.Equal(t, "127.0.0.1", cli.Host.Get())
	assert.Equal(t, "prod-east", cli.ClusterName.Get())
	assert.Equal(t, 15*time.Second, cli.Timeout.Get())
	assert.True(t, cli.Debug.Get())
	assert.Equal(t, "tok_json_123", cli.SecretToken.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
}

func TestConfig_TOMLDecoder(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(cfgPath, fixtureDecoderTOML, 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9002, cli.Port.Get())
	assert.Equal(t, "10.0.0.1", cli.Host.Get())
	assert.Equal(t, "prod-west", cli.ClusterName.Get())
	assert.Equal(t, 30*time.Second, cli.Timeout.Get())
	assert.True(t, cli.Debug.Get())
	assert.Equal(t, "tok_toml_123", cli.SecretToken.Get())
	assert.Equal(t, conga.SourceConfig, cli.ClusterName.Source())
}

func TestConfig_YAMLDecoder(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(cfgPath, fixtureDecoderYAML, 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		RegisterDecoder(".yaml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9003, cli.Port.Get())
	assert.Equal(t, "192.168.1.1", cli.Host.Get())
	assert.Equal(t, "prod-central", cli.ClusterName.Get())
	assert.Equal(t, 45*time.Second, cli.Timeout.Get())
	assert.True(t, cli.Debug.Get())
	assert.Equal(t, "tok_yaml_123", cli.SecretToken.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
}

func TestConfig_HCLDecoder(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.hcl")
	err := os.WriteFile(cfgPath, fixtureDecoderHCL, 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		RegisterDecoder(".hcl", hcl.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9004, cli.Port.Get())
	assert.Equal(t, "127.0.0.2", cli.Host.Get())
	assert.Equal(t, "prod-hcl", cli.ClusterName.Get())
	assert.Equal(t, time.Minute, cli.Timeout.Get())
	assert.True(t, cli.Debug.Get())
	assert.Equal(t, "tok_hcl_123", cli.SecretToken.Get())
	assert.Equal(t, conga.SourceConfig, cli.Host.Source())
}

func TestConfig_StrictUnknownFieldRejection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(cfgPath, fixtureUnknownField, 0o600)
	assert.NoError(t, err)

	app := conga.New[MultiFormatServerCLI]().
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
}

func TestConfig_ExcludedFieldRejection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(cfgPath, fixtureExcludedField, 0o600)
	assert.NoError(t, err)

	app := conga.New[MultiFormatServerCLI]().
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
}

func TestConfig_NestedSubcommandTables(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(cfgPath, fixtureNestedSubcommands, 0o600)
	assert.NoError(t, err)

	cli := new(HierarchicalRootCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{"cluster", "node"})

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Verbose.Get())
	assert.Equal(t, "eu-west-1", cli.Cluster.Get().Region.Get())
	assert.Equal(t, "eu-west-1a", cli.Cluster.Get().Node.Get().Zone.Get())
	assert.Equal(t, 5, cli.Cluster.Get().Node.Get().Count.Get())
	assert.Equal(t, conga.SourceConfig, cli.Cluster.Get().Node.Get().Zone.Source())
}

func TestConfig_PreParsedFlag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "dynamic.toml")
	err := os.WriteFile(cfgPath, fixturePreparsed, 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		Args([]string{"--config-file", cfgPath, "--debug"})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 7777, cli.Port.Get())
	assert.Equal(t, "10.10.10.10", cli.Host.Get())
	assert.True(t, cli.Debug.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
	assert.Equal(t, conga.SourceCLI, cli.Debug.Source())
}

// TestConfig_MultiFlagTierFirstHitWithoutCascading pins the de-entanglement of
// flag arity and tier mode: a multi:"true" config-file flag falls back to the
// first existing tier file unless Cascading() is declared — arity never
// implies merge mode.
func TestConfig_MultiFlagTierFirstHitWithoutCascading(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	firstCfg := filepath.Join(tmpDir, "first.toml")
	secondCfg := filepath.Join(tmpDir, "second.toml")

	err := os.WriteFile(firstCfg, fixtureMultiFlagFirst, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(secondCfg, fixtureMultiFlagSecond, 0o600)
	assert.NoError(t, err)

	type MultiTierCLI struct {
		noopRunner

		Config conga.ConfigFile        `multi:"true"   short:"c"`
		Port   conga.OptionalFlag[int] `default:"8080"`
	}

	cli := new(MultiTierCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(firstCfg, secondCfg)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 7000, cli.Port.Get())
	assert.Equal(t, []string{firstCfg}, cli.Config.Paths())
	assert.Equal(t, firstCfg, cli.Config.Path())
}

// TestConfig_MultiFlagTierCascadingMerge pins that ConfigCascading() merges every
// existing tier file into the multi flag in load order (lowest precedence
// first), and that Paths() reports exactly the loaded files. The tier list is
// precedence-ordered highest first — the same convention as first-hit mode.
func TestConfig_MultiFlagTierCascadingMerge(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "sys.toml")
	localCfg := filepath.Join(tmpDir, "local.toml")

	err := os.WriteFile(sysCfg, fixtureTierSys, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(localCfg, fixtureTierLocal, 0o600)
	assert.NoError(t, err)

	type MultiCascadeCLI struct {
		noopRunner

		Config conga.ConfigFile        `multi:"true"   short:"c"`
		Port   conga.OptionalFlag[int] `default:"8080"`
		Host   conga.OptionalFlag[string]
	}

	cli := new(MultiCascadeCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, sysCfg), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 8001, cli.Port.Get())
	assert.Equal(t, "sys-host", cli.Host.Get())
	assert.Equal(t, []string{sysCfg, localCfg}, cli.Config.Paths())
	assert.Equal(t, localCfg, cli.Config.Path())
}

// TestConfig_CascadingScalarSourceDetailNamesFile pins that a config-sourced
// scalar records the contributing file as its source detail: the nearest file
// for a key set by several files, and the lower file for a key it alone sets.
func TestConfig_CascadingScalarSourceDetailNamesFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "sys.toml")
	localCfg := filepath.Join(tmpDir, "local.toml")

	err := os.WriteFile(sysCfg, fixtureTierSys, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(localCfg, fixtureTierLocal, 0o600)
	assert.NoError(t, err)

	type ScalarSourceDetailCLI struct {
		noopRunner

		Config conga.ConfigFile        `multi:"true"`
		Port   conga.OptionalFlag[int] `default:"8080"`
		Host   conga.OptionalFlag[string]
	}

	cli := new(ScalarSourceDetailCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, sysCfg), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
	assert.Equal(t, localCfg, cli.Port.SourceDetail())
	assert.Equal(t, conga.SourceConfig, cli.Host.Source())
	assert.Equal(t, sysCfg, cli.Host.SourceDetail())
}

// TestConfigOptions_MultipleCalls_AccumulateSlots verifies that separate
// app.ConfigOptions calls accumulate independent option slots instead of
// replacing the settings declared by earlier calls.
func TestConfigOptions_MultipleCalls_AccumulateSlots(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "sys.toml")
	localCfg := filepath.Join(tmpDir, "local.toml")

	err := os.WriteFile(sysCfg, fixtureTierSys, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(localCfg, fixtureTierLocal, 0o600)
	assert.NoError(t, err)

	type SplitOptionsCLI struct {
		noopRunner

		Config conga.ConfigFile        `multi:"true"   short:"c"`
		Port   conga.OptionalFlag[int] `default:"8080"`
		Host   conga.OptionalFlag[string]
	}

	cli := new(SplitOptionsCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, sysCfg)).
		ConfigOptions(conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 8001, cli.Port.Get())
	assert.Equal(t, "sys-host", cli.Host.Get())
	assert.Equal(t, []string{sysCfg, localCfg}, cli.Config.Paths())
}

func TestConfig_SearchPathsSingleFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "missing.toml")
	file2 := filepath.Join(tmpDir, "found.toml")
	file3 := filepath.Join(tmpDir, "later.toml")

	err := os.WriteFile(file2, []byte("port = 5555\n"), 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(file3, []byte("port = 6666\n"), 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(file1, file2, file3)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 5555, cli.Port.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
}

func TestConfig_CascadingResolution(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "sys.toml")
	userCfg := filepath.Join(tmpDir, "user.toml")
	localCfg := filepath.Join(tmpDir, "local.toml")

	// System defaults
	err := os.WriteFile(sysCfg, fixtureCascadeSys, 0o600)
	assert.NoError(t, err)

	// User overrides
	err = os.WriteFile(userCfg, fixtureCascadeUser, 0o600)
	assert.NoError(t, err)

	// Local overrides
	err = os.WriteFile(localCfg, fixtureCascadeLocal, 0o600)
	assert.NoError(t, err)

	// Precedence order: local > user > sys
	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, userCfg, sysCfg), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9500, cli.Port.Get())              // from local.toml (highest precedence)
	assert.Equal(t, 20*time.Second, cli.Timeout.Get()) // from user.toml
	assert.Equal(t, "sys-host", cli.Host.Get())        // from sys.toml
}

// MapCascadeCLI exercises map and slice flags against cascading config files.
type MapCascadeCLI struct {
	noopRunner

	Config conga.ConfigFile                      `help:"Config file" multi:"true"`
	Tags   conga.OptionalFlag[map[string]string] `help:"Tags map"`
	Hosts  conga.OptionalFlag[[]string]          `help:"Host list"`
}

// TestConfig_CascadingMapKeyMerge pins that map flags merge key by key across
// cascading files: disjoint keys from every file survive, while scalar and
// slice flags keep nearest-file-wins semantics.
func TestConfig_CascadingMapKeyMerge(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "sys.toml")
	localCfg := filepath.Join(tmpDir, "local.toml")

	err := os.WriteFile(sysCfg, fixtureMapCascadeSys, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(localCfg, fixtureMapCascadeLocal, 0o600)
	assert.NoError(t, err)

	cli := new(MapCascadeCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, sysCfg), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"base": "sys", "shared": "sys", "local": "local"}, cli.Tags.Get())
	assert.Equal(t, []string{"local-a"}, cli.Hosts.Get())
	assert.Equal(t, conga.SourceConfig, cli.Tags.Source())
}

// TestConfig_CascadingMapSourceDetailListsFiles pins that a config-sourced map
// flag records every contributing file as its source detail, in configuration
// load order (lowest precedence first).
func TestConfig_CascadingMapSourceDetailListsFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "sys.toml")
	localCfg := filepath.Join(tmpDir, "local.toml")

	err := os.WriteFile(sysCfg, fixtureMapCascadeSys, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(localCfg, fixtureMapCascadeLocal, 0o600)
	assert.NoError(t, err)

	cli := new(MapCascadeCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, sysCfg), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, conga.SourceConfig, cli.Tags.Source())
	assert.Equal(t, sysCfg+", "+localCfg, cli.Tags.SourceDetail())
}

// TestConfig_CascadingMapKeyNearestFileWins pins the per-key file-order rule
// across three cascading files: an overlapping key resolves to the nearest
// file and disjoint keys from lower files survive.
func TestConfig_CascadingMapKeyNearestFileWins(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "sys.toml")
	userCfg := filepath.Join(tmpDir, "user.toml")
	localCfg := filepath.Join(tmpDir, "local.toml")

	err := os.WriteFile(sysCfg, fixtureMapCascadeSys, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(userCfg, fixtureMapCascadeUser, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(localCfg, fixtureMapCascadeLocal, 0o600)
	assert.NoError(t, err)

	cli := new(MapCascadeCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, userCfg, sysCfg), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"base": "sys", "shared": "user", "extra": "user", "local": "local"}, cli.Tags.Get())
	assert.Equal(t, []string{"local-a"}, cli.Hosts.Get())
}

// MapCascadeDeployCLI exercises a map flag declared on a subcommand against
// cascading config files nesting the section under the command path.
type MapCascadeDeployCLI struct {
	noopRunner

	Config conga.ConfigFile                `help:"Config file"       multi:"true"`
	Deploy conga.Command[MapCascadeDeploy] `summary:"Deploy command"`
}

type MapCascadeDeploy struct {
	Tags conga.OptionalFlag[map[string]string] `help:"Deploy tags"`
}

func (deploy *MapCascadeDeploy) Run(_ context.Context, _ *conga.Session) error { return nil }

// TestConfig_CascadingMapSubcommandKeyMerge pins that a subcommand-owned map
// flag merges key by key across cascading files addressing it under the
// command path.
func TestConfig_CascadingMapSubcommandKeyMerge(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "sys.toml")
	localCfg := filepath.Join(tmpDir, "local.toml")

	err := os.WriteFile(sysCfg, fixtureMapCascadeSubSys, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(localCfg, fixtureMapCascadeSubLocal, 0o600)
	assert.NoError(t, err)

	cli := new(MapCascadeDeployCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, sysCfg), conga.ConfigCascading()).
		Args([]string{"deploy"})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"base": "sys", "shared": "sys", "local": "local"}, cli.Deploy.Get().Tags.Get())
}

// TestConfig_FirstHitMapKeepsFirstFileValue pins that without cascading a map
// flag takes its whole value from the first existing tier file.
func TestConfig_FirstHitMapKeepsFirstFileValue(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "sys.toml")
	localCfg := filepath.Join(tmpDir, "local.toml")

	err := os.WriteFile(sysCfg, fixtureMapCascadeSys, 0o600)
	assert.NoError(t, err)
	err = os.WriteFile(localCfg, fixtureMapCascadeLocal, 0o600)
	assert.NoError(t, err)

	cli := new(MapCascadeCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, sysCfg)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"local": "local"}, cli.Tags.Get())
	assert.Equal(t, []string{"local-a"}, cli.Hosts.Get())
	assert.Equal(t, []string{localCfg}, cli.Config.Paths())
}

// NamespacedMapCLI exercises a map flag addressed through a dotted config
// namespace alongside a scalar sibling.
type NamespacedMapCLI struct {
	noopRunner

	Config   conga.ConfigFile                      `help:"Config file"       multi:"true"`
	Insecure conga.OptionalFlag[bool]              `config:"remote.insecure" help:"Insecure mode"`
	Pairs    conga.OptionalFlag[map[string]string] `config:"remote.pairs"    help:"Remote pairs"`
}

// TestConfig_NamespacedMapFlagBinding pins that a map flag behind a dotted
// config namespace binds its nested section, alongside the scalar sibling in
// the same namespace.
func TestConfig_NamespacedMapFlagBinding(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "remote.yaml")
	err := os.WriteFile(cfgPath, fixtureNestedNamespaceMap, 0o600)
	assert.NoError(t, err)

	cli := new(NamespacedMapCLI)
	app := conga.New(cli).
		RegisterDecoder(".yaml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Insecure.Get())
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, cli.Pairs.Get())
	assert.Equal(t, conga.SourceConfig, cli.Pairs.Source())
}

// TestConfig_NamespacedMapFlagCascadingMerge pins that a dotted-namespace map
// flag merges key by key across cascading files.
func TestConfig_NamespacedMapFlagCascadingMerge(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "remote.yaml")
	err := os.WriteFile(sysCfg, fixtureNestedNamespaceMap, 0o600)
	assert.NoError(t, err)

	localCfg := writeConfigFile(t, `{"remote": {"pairs": {"c": "3"}}}`)

	cli := new(NamespacedMapCLI)
	app := conga.New(cli).
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(localCfg, sysCfg), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2", "c": "3"}, cli.Pairs.Get())
}

func TestConfigFiles_MultiFlag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "base.json")
	err := os.WriteFile(basePath, []byte(`{"port": 9000, "host": "0.0.0.0", "env": "staging"}`), 0o600)
	assert.NoError(t, err)

	overridePath := filepath.Join(tmpDir, "override.json")
	err = os.WriteFile(overridePath, []byte(`{"port": 9999, "env": "prod"}`), 0o600)
	assert.NoError(t, err)

	cli := new(MultiConfigFileTestCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		Args([]string{"-c", basePath, "-c", overridePath})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 2, len(cli.Configs.Paths()))
	assert.Equal(t, []string{basePath, overridePath}, cli.Configs.Paths())
	assert.Equal(t, 9999, cli.Port.Get())
	assert.Equal(t, "0.0.0.0", cli.Host.Get())
	assert.Equal(t, "prod", cli.Env.Get())
}

func TestSession_LoadedConfigFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "base.json")
	err := os.WriteFile(basePath, []byte(`{"port": 9000, "host": "0.0.0.0", "env": "staging"}`), 0o600)
	assert.NoError(t, err)

	overridePath := filepath.Join(tmpDir, "override.json")
	err = os.WriteFile(overridePath, []byte(`{"port": 9999, "env": "prod"}`), 0o600)
	assert.NoError(t, err)

	cli := new(MultiConfigFileTestCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		Args([]string{"-c", basePath, "-c", overridePath})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{basePath, overridePath}, cli.Loaded)
}

type NamespacedFlagAliasCLI struct {
	ConfigFile conga.ConfigFile        `short:"c"`
	Port       conga.OptionalFlag[int] `aliases:"listen-port" config:"network.port" short:"p"`
}

func (n *NamespacedFlagAliasCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

func TestConfig_NamespacedFlagAlias(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "app.json")
	err := os.WriteFile(cfgPath, []byte(`{"network": {"listen-port": 8888}}`), 0o600)
	assert.NoError(t, err)

	cli := new(NamespacedFlagAliasCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 8888, cli.Port.Get())
}

func TestConfigFiles_CommaSeparated(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "base.json")
	err := os.WriteFile(basePath, []byte(`{"port": 9000, "host": "0.0.0.0"}`), 0o600)
	assert.NoError(t, err)

	overridePath := filepath.Join(tmpDir, "override.json")
	err = os.WriteFile(overridePath, []byte(`{"port": 7777}`), 0o600)
	assert.NoError(t, err)

	cli := new(MultiConfigFileTestCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		Args([]string{"-c", basePath + "," + overridePath})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 2, len(cli.Configs.Paths()))
	assert.Equal(t, []string{basePath, overridePath}, cli.Configs.Paths())
	assert.Equal(t, 7777, cli.Port.Get())
	assert.Equal(t, "0.0.0.0", cli.Host.Get())
}

func TestConfig_DirectorySearchPaths(t *testing.T) {
	t.Parallel()

	localDir := t.TempDir()
	userDir := t.TempDir()

	err := os.WriteFile(filepath.Join(userDir, "server.json"), []byte(`{"port": 8500, "host": "user-host", "timeout": "10s"}`), 0o600)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(localDir, "server.toml"), []byte("port = 9500\n"), 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		Name("server").
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(localDir, userDir), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9500, cli.Port.Get())
	assert.Equal(t, "user-host", cli.Host.Get())
	assert.Equal(t, 10*time.Second, cli.Timeout.Get())
}

func TestConfig_StemSearchPaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	localStem := filepath.Join(tmpDir, "local")
	userStem := filepath.Join(tmpDir, "user")

	err := os.WriteFile(userStem+".json", []byte(`{"port": 8500, "host": "user-host"}`), 0o600)
	assert.NoError(t, err)

	err = os.WriteFile(localStem+".toml", []byte("port = 9500\n"), 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(localStem, userStem), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9500, cli.Port.Get())
	assert.Equal(t, "user-host", cli.Host.Get())
}

func TestConfig_MultiFormatConflict_Directory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "server.toml"), []byte("port = 9500\n"), 0o600)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "server.json"), []byte(`{"port": 8500}`), 0o600)
	assert.NoError(t, err)

	app := conga.New[MultiFormatServerCLI]().
		Name("server").
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(tmpDir), conga.ConfigCascading()).
		Args([]string{})

	err = app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), "multiple conflicting config files found")
	assert.Contains(t, err.Error(), "server.json, server.toml")
}

func TestConfig_MultiFormatConflict_Stem(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgStem := filepath.Join(tmpDir, "config")

	err := os.WriteFile(cfgStem+".toml", []byte("port = 9500\n"), 0o600)
	assert.NoError(t, err)

	err = os.WriteFile(cfgStem+".hcl", []byte("port = 8500\n"), 0o600)
	assert.NoError(t, err)

	app := conga.New[MultiFormatServerCLI]().
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".hcl", hcl.Decode).
		ConfigOptions(conga.ConfigPaths(cfgStem)).
		Args([]string{})

	err = app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), "multiple conflicting config files found")
	assert.Contains(t, err.Error(), "config.hcl, config.toml")
}

type NativeMapCLI struct {
	Env conga.Command[NativeMapEnvCmd] `summary:"Environment config"`
}

type NativeMapEnvCmd struct {
	Vars   conga.OptionalFlag[map[string]string] `help:"Environment variables"`
	Labels conga.OptionalFlag[map[string]string] `help:"Resource labels"`
}

func (n *NativeMapEnvCmd) Run(_ context.Context, _ *conga.Session) error { return nil }

func TestConfig_NativeMap_TOML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(cfgPath, fixtureNativeMapTOML, 0o600)
	assert.NoError(t, err)

	cli := new(NativeMapCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{"env"})

	err = app.Run()

	assert.NoError(t, err)

	vars := cli.Env.Get().Vars.Get()
	assert.Equal(t, "debug", vars["LOG_LEVEL"])
	assert.Equal(t, "production", vars["APP_ENV"])

	labels := cli.Env.Get().Labels.Get()
	assert.Equal(t, "frontend", labels["tier"])
	assert.Equal(t, "platform", labels["team"])
}

func TestConfig_NativeMap_YAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(cfgPath, fixtureNativeMapYAML, 0o600)
	assert.NoError(t, err)

	cli := new(NativeMapCLI)
	app := conga.New(cli).
		RegisterDecoder(".yaml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{"env"})

	err = app.Run()

	assert.NoError(t, err)

	vars := cli.Env.Get().Vars.Get()
	assert.Equal(t, "debug", vars["LOG_LEVEL"])
	assert.Equal(t, "production", vars["APP_ENV"])

	labels := cli.Env.Get().Labels.Get()
	assert.Equal(t, "frontend", labels["tier"])
	assert.Equal(t, "platform", labels["team"])
}

func TestConfig_NativeMap_JSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(cfgPath, fixtureNativeMapJSON, 0o600)
	assert.NoError(t, err)

	cli := new(NativeMapCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{"env"})

	err = app.Run()

	assert.NoError(t, err)

	vars := cli.Env.Get().Vars.Get()
	assert.Equal(t, "debug", vars["LOG_LEVEL"])
	assert.Equal(t, "production", vars["APP_ENV"])

	labels := cli.Env.Get().Labels.Get()
	assert.Equal(t, "frontend", labels["tier"])
	assert.Equal(t, "platform", labels["team"])
}

func TestConfig_NativeMap_HCL(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.hcl")
	err := os.WriteFile(cfgPath, fixtureNativeMapHCL, 0o600)
	assert.NoError(t, err)

	cli := new(NativeMapCLI)
	app := conga.New(cli).
		RegisterDecoder(".hcl", hcl.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{"env"})

	err = app.Run()

	assert.NoError(t, err)

	vars := cli.Env.Get().Vars.Get()
	assert.Equal(t, "debug", vars["LOG_LEVEL"])
	assert.Equal(t, "production", vars["APP_ENV"])

	labels := cli.Env.Get().Labels.Get()
	assert.Equal(t, "frontend", labels["tier"])
	assert.Equal(t, "platform", labels["team"])
}

type NestedNativeMapCLI struct {
	Server conga.Command[NestedNativeMapServerCmd] `summary:"Server config"`
	Env    conga.Command[NestedNativeMapEnvCmd]    `summary:"Environment config"`
}

type NestedNativeMapServerCmd struct {
	Port conga.OptionalFlag[int]    `default:"8080"      help:"Server port"`
	Host conga.OptionalFlag[string] `default:"localhost" help:"Server host"`
}

type NestedNativeMapEnvCmd struct {
	Vars   conga.OptionalFlag[map[string]string] `help:"Environment variables"`
	Labels conga.OptionalFlag[map[string]string] `help:"Resource labels"`
}

func (n *NestedNativeMapEnvCmd) Run(_ context.Context, _ *conga.Session) error { return nil }

func TestConfig_NestedNativeMap_TOML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(cfgPath, fixtureNestedNativeMap, 0o600)
	assert.NoError(t, err)

	cli := new(NestedNativeMapCLI)
	app := conga.New(cli).
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{"env"})

	err = app.Run()

	assert.NoError(t, err)

	vars := cli.Env.Get().Vars.Get()
	assert.Equal(t, "true", vars["DEBUG"])
	assert.Equal(t, "test", vars["MODE"])

	labels := cli.Env.Get().Labels.Get()
	assert.Equal(t, "test", labels["env"])
	assert.Equal(t, "1.0", labels["version"])
}

func TestConfig_FileSizeLimitExceeded(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "large.json")

	f, err := os.Create(cfgPath)
	assert.NoError(t, err)
	err = f.Truncate(16*1024*1024 + 1)
	assert.NoError(t, err)
	err = f.Close()
	assert.NoError(t, err)

	type SimpleConfigCLI struct {
		Host conga.OptionalFlag[string] `default:"localhost" help:"Host"`
	}

	app := conga.New[SimpleConfigCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	err = app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestConfig_MultipleEnvVarsForConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(cfgPath, []byte(`{"host":"remote"}`), 0o600)
	assert.NoError(t, err)

	type MultiEnvConfigCLI struct {
		noopRunner

		Config conga.ConfigFile           `env:"CONGA_TEST_PRIMARY_CFG,CONGA_TEST_FALLBACK_CFG" name:"config"`
		Host   conga.OptionalFlag[string] `name:"host"`
	}

	t.Setenv("CONGA_TEST_PRIMARY_CFG", "")
	t.Setenv("CONGA_TEST_FALLBACK_CFG", cfgPath)

	cli := new(MultiEnvConfigCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		Args([]string{})

	err = app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "remote", cli.Host.Get())
	assert.Equal(t, conga.SourceEnv, cli.Config.Source())
}

func TestConfig_NestedSubcommandPrefixing(t *testing.T) {
	t.Parallel()

	type NodeCLI struct {
		noopRunner

		Zone conga.OptionalFlag[string] `config:"zone" name:"zone"`
	}

	type ClusterCLI struct {
		noopRunner

		Node conga.Command[NodeCLI] `name:"node"`
	}

	type RootCLI struct {
		noopRunner

		Cluster conga.Command[ClusterCLI] `name:"cluster"`
	}

	t.Run("resolves hierarchical config key for deep subcommand", func(t *testing.T) {
		t.Parallel()

		cli := new(RootCLI)
		app := conga.New(cli).
			RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"cluster": {"node": {"zone": "us-east-1"}}}`))).
			Args([]string{"cluster", "node"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)

		clusterCmd := cli.Cluster.Get()
		nodeCmd := clusterCmd.Node.Get()
		assert.Equal(t, "us-east-1", nodeCmd.Zone.Get())
		assert.Equal(t, conga.SourceConfig, nodeCmd.Zone.Source())
	})

	t.Run("subcommand flag with explicit already-namespaced config tag does not double prefix", func(t *testing.T) {
		t.Parallel()

		type ServerCLI struct {
			noopRunner

			Port conga.OptionalFlag[int] `config:"server.port" name:"port"`
		}

		type AppCLI struct {
			noopRunner

			Server conga.Command[ServerCLI] `name:"server"`
		}

		cli := new(AppCLI)
		app := conga.New(cli).
			RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"server": {"port": "9999"}}`))).
			Args([]string{"server"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)

		serverCmd := cli.Server.Get()
		assert.Equal(t, 9999, serverCmd.Port.Get())
		assert.Equal(t, conga.SourceConfig, serverCmd.Port.Source())
	})
}

func TestConfig_DefaultXDGTierLoadsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	appDir := filepath.Join(tmpDir, "xdg-server")
	err := os.MkdirAll(appDir, 0o700)
	assert.NoError(t, err)

	configPath := filepath.Join(appDir, "config.toml")
	err = os.WriteFile(configPath, []byte("port = 4444\n"), 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		Name("xdg-server").
		RegisterDecoder(".toml", toml.Decode).
		Args([]string{})

	err = app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 4444, cli.Port.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
	assert.Equal(t, configPath, cli.ConfigFile.Path())
	assert.Equal(t, conga.SourceConfig, cli.ConfigFile.Source())
}

func TestConfig_DefaultXDGTierAppNamedFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	appDir := filepath.Join(tmpDir, "xdg-server")
	err := os.MkdirAll(appDir, 0o700)
	assert.NoError(t, err)

	namedPath := filepath.Join(appDir, "xdg-server.toml")
	err = os.WriteFile(namedPath, []byte("port = 5555\n"), 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		Name("xdg-server").
		RegisterDecoder(".toml", toml.Decode).
		Args([]string{})

	err = app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 5555, cli.Port.Get())
	assert.Equal(t, namedPath, cli.ConfigFile.Path())
}

func TestConfig_AuthorSearchPathsReplaceDefaultXDGTier(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	appDir := filepath.Join(tmpDir, "xdg-server")
	err := os.MkdirAll(appDir, 0o700)
	assert.NoError(t, err)

	configPath := filepath.Join(appDir, "config.toml")
	err = os.WriteFile(configPath, []byte("port = 4444\n"), 0o600)
	assert.NoError(t, err)

	missingPath := filepath.Join(tmpDir, "missing.toml")

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		Name("xdg-server").
		RegisterDecoder(".toml", toml.Decode).
		ConfigOptions(conga.ConfigPaths(missingPath)).
		Args([]string{})

	err = app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 8080, cli.Port.Get())
	assert.Equal(t, conga.SourceDefault, cli.Port.Source())
	assert.Equal(t, "", cli.ConfigFile.Path())
}

func TestConfig_DefaultXDGTierInactiveWithoutDecoder(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	appDir := filepath.Join(tmpDir, "xdg-server")
	err := os.MkdirAll(appDir, 0o700)
	assert.NoError(t, err)

	configPath := filepath.Join(appDir, "config.toml")
	err = os.WriteFile(configPath, []byte("port = 4444\n"), 0o600)
	assert.NoError(t, err)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		Name("xdg-server").
		Args([]string{})

	err = app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 8080, cli.Port.Get())
	assert.Equal(t, conga.SourceDefault, cli.Port.Source())
	assert.Equal(t, "", cli.ConfigFile.Path())
}

func TestConfig_DefaultXDGTierMissingDirectoryIsSilent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cli := new(MultiFormatServerCLI)
	app := conga.New(cli).
		Name("xdg-server").
		RegisterDecoder(".toml", toml.Decode).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 8080, cli.Port.Get())
	assert.Equal(t, conga.SourceDefault, cli.Port.Source())
	assert.Equal(t, "", cli.ConfigFile.Path())
}

func TestConfig_ConfigExplicit_ConflictWithPathsOrCascading_FailsSchema(t *testing.T) {
	t.Parallel()

	type SimpleCLI struct {
		noopRunner

		Port conga.OptionalFlag[int] `name:"port"`
	}

	t.Run("explicit combined with paths", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleCLI]().
			ConfigOptions(conga.ConfigExplicit("app.json"), conga.ConfigPaths("search.json")).
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Contains(t, err.Error(), "conga.ConfigExplicit() specifies an absolute configuration file and cannot be combined with conga.ConfigPaths() or conga.ConfigCascading()")
	})

	t.Run("explicit combined with cascading", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleCLI]().
			ConfigOptions(conga.ConfigExplicit("app.json"), conga.ConfigCascading()).
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Contains(t, err.Error(), "conga.ConfigExplicit() specifies an absolute configuration file and cannot be combined with conga.ConfigPaths() or conga.ConfigCascading()")
	})
}
