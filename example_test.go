package conga_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/predict"
	"github.com/conga-sh/conga/timezone"
)

// CLI is a minimal single-command application.
type CLI struct {
	Name conga.OptionalFlag[string] `default:"world" help:"Who to greet"`
}

func (c *CLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "Hello, %s!\n", c.Name.Get())

	return err
}

// RootCLI is an application with one subcommand that reads a root flag.
type RootCLI struct {
	Verbose conga.OptionalFlag[bool]  `help:"Verbose output"`
	Migrate conga.Command[MigrateCmd] `summary:"Run database migrations"`
}

// MigrateCmd runs the database migration subcommand.
type MigrateCmd struct{}

func (c *MigrateCmd) Run(_ context.Context, session *conga.Session) error {
	root := session.Root[RootCLI]()
	_, err := fmt.Fprintf(session.Stdout(), "%s verbose=%t\n", session.Path(), root.Verbose.Get())

	return err
}

// Percent is a whole-number percentage between 0% and 100%, parsed from flag
// values such as "75%" or "75".
type Percent int

// String returns the percentage in its canonical flag form, for example "75%".
func (p Percent) String() string {
	return strconv.Itoa(int(p)) + "%"
}

// PercentCLI is an application whose flag accepts a percentage threshold.
type PercentCLI struct {
	Threshold conga.OptionalFlag[Percent] `default:"50%" help:"Alert threshold"`
}

func (c *PercentCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "threshold=%v\n", c.Threshold.Get())

	return err
}

// NetworkCLI is an application whose flag accepts a CIDR network.
type NetworkCLI struct {
	Network conga.OptionalFlag[*net.IPNet] `help:"Allowed network in CIDR notation" placeholder:"CIDR"`
}

func (c *NetworkCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "network=%s\n", c.Network.Get())

	return err
}

// DeprecatedCLI is an application that warns when it runs.
type DeprecatedCLI struct{}

func (c *DeprecatedCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintln(session.Stdout(), "running")

	return err
}

// ConfigCLI is an application whose flags resolve from a configuration file.
type ConfigCLI struct {
	Config conga.ConfigFile           `help:"Configuration file"`
	Host   conga.OptionalFlag[string] `help:"Bind host"`
	Port   conga.OptionalFlag[int]    `help:"Listen port"`
}

func (c *ConfigCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "port=%d host=%s\n", c.Port.Get(), c.Host.Get())

	return err
}

// BootstrapCLI is an application whose Bootstrap hook supplies a configuration
// document.
type BootstrapCLI struct {
	Host conga.OptionalFlag[string] `default:"localhost"`
	Port conga.OptionalFlag[int]    `default:"8000"`
}

func (c *BootstrapCLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	document := &conga.ConfigDocument{}
	document.Set("host", "document.example.com", "embedded defaults")
	document.Set("port", 9200, "embedded defaults")
	b.SetConfig(document)

	return nil
}

func (c *BootstrapCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "port=%d host=%s\n", c.Port.Get(), c.Host.Get())

	return err
}

// RegionCLI is an application whose flag completes from a registered predictor.
type RegionCLI struct {
	Region conga.OptionalFlag[string] `help:"Deployment region" predictor:"regions"`
}

func (c *RegionCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// Cluster is a flag type that completes itself through Predictable.
type Cluster string

func (c Cluster) Predict() predict.Predictor {
	return predict.Values("alpha", "beta", "gamma")
}

// PredictableCLI is an application whose flag completes from its own type.
type PredictableCLI struct {
	Cluster conga.OptionalFlag[Cluster] `help:"Target cluster"`
}

func (c *PredictableCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// ReplicasCLI is an application whose flag accepts a map of counts.
type ReplicasCLI struct {
	Replicas conga.OptionalFlag[map[string]int] `help:"Replica counts"`
}

func (c *ReplicasCLI) Run(_ context.Context, session *conga.Session) error {
	replicas := c.Replicas.Get()
	_, err := fmt.Fprintf(session.Stdout(), "api=%d web=%d\n", replicas["api"], replicas["web"])

	return err
}

// InspectCLI is an application whose bound fields are read after execution.
type InspectCLI struct {
	Port conga.OptionalFlag[int] `help:"Port"`
}

func (c *InspectCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// CounterCLI is an application with a verbosity counter flag.
type CounterCLI struct {
	Verbose conga.Counter `help:"Increase verbosity" short:"v"`
}

func (c *CounterCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "verbosity=%d\n", c.Verbose.Get())

	return err
}

// FlagStateCLI is an application whose optional flag declares a default.
type FlagStateCLI struct {
	Port conga.OptionalFlag[int] `default:"8080" help:"Listen port"`
}

func (c *FlagStateCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "value=%d set=%t changed=%t source=%s\n",
		c.Port.Get(), c.Port.IsSet(), c.Port.Changed(), c.Port.Source())

	return err
}

// DashCLI is an application that captures trailing arguments after '--'.
type DashCLI struct {
	Target conga.RequiredArg[string] `help:"Target package" name:"target"`
	Flags  conga.DashArgs            `name:"flags"`
}

func (c *DashCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "%s: %s\n", c.Target.Get(), strings.Join(c.Flags.Get(), " "))

	return err
}

// RawCLI is an application that forwards trailing unparsed arguments.
type RawCLI struct {
	Command conga.RawArgs `name:"command"`
}

func (c *RawCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "exec: %s\n", strings.Join(c.Command.Get(), " "))

	return err
}

// ByteSizeCLI is an application whose flag accepts human-readable data volumes.
type ByteSizeCLI struct {
	Limit conga.OptionalFlag[conga.ByteSize] `default:"512MiB" help:"Memory limit"`
}

func (c *ByteSizeCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "limit=%s bytes=%d\n", c.Limit.Get(), c.Limit.Get().Bytes())

	return err
}

// UseCLI is an application whose flag type is registered through a mounted binding.
type UseCLI struct {
	Zone conga.OptionalFlag[*time.Location] `help:"Server timezone"`
}

func (c *UseCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintln(session.Stdout(), c.Zone.Get())

	return err
}

// BuildExperiments declares feature toggles for ExampleFeatures and ExampleApp_EnableFeatures.
type BuildExperiments struct {
	RemoteCache      bool `help:"Experimental distributed CAS artifact cache"`
	SandboxIsolation int  `enum:"1,2"                                         help:"Experimental build sandbox isolation: 1=chroot, 2=rootless-container"`
}

// BuildCLI is an application with feature toggles and gated flags.
type BuildCLI struct {
	Features       conga.Features[BuildExperiments] `flag:"feature-gates"`
	Target         conga.RequiredArg[string]        `help:"Build target"`
	CacheEndpoint  conga.OptionalFlag[string]       `feature:"remote-cache"      help:"CAS server gRPC endpoint"`
	ContainerImage conga.OptionalFlag[string]       `feature:"sandbox-isolation" help:"OCI build container image"`
}

func (c *BuildCLI) Run(_ context.Context, session *conga.Session) error {
	experiments := c.Features.Get()
	_, err := fmt.Fprintf(session.Stdout(),
		"remote-cache=%t sandbox=%d endpoint=%s image=%s target=%s\n",
		experiments.RemoteCache,
		experiments.SandboxIsolation,
		c.CacheEndpoint.Get(),
		c.ContainerImage.Get(),
		c.Target.Get(),
	)

	return err
}

// MapMergeCLI is an application whose map flag merges defaults with CLI inputs.
type MapMergeCLI struct {
	Settings conga.OptionalFlag[map[string]string] `default:"env=prod,retries=3" help:"Runtime settings"`
}

func (c *MapMergeCLI) Run(_ context.Context, session *conga.Session) error {
	settings := c.Settings.Get()
	_, err := fmt.Fprintf(session.Stdout(), "env=%s retries=%s timeout=%s\n",
		settings["env"], settings["retries"], settings["timeout"])

	return err
}

// Declare flags as typed fields, implement [conga.Runner], and call [conga.App.Run].
func ExampleNew() {
	app := conga.New[CLI]().
		Args([]string{"--name", "gopher"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
	// Output: Hello, gopher!
}

// A subcommand reads global flags declared on the root command through
// [conga.Session.Root].
func ExampleSession_Root() {
	app := conga.New[RootCLI]().
		Name("tool").
		Args([]string{"--verbose", "migrate"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
	// Output: tool migrate verbose=true
}

// Register a parser so flags and arguments of a custom type accept its text form.
func ExampleApp_RegisterType() {
	app := conga.New[PercentCLI]().
		RegisterType(parsePercent).
		Args([]string{"--threshold", "75%"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
	// Output: threshold=75%
}

// Register a parser for a type CONGA cannot parse natively, such as a standard
// library type that does not implement encoding.TextUnmarshaler.
func ExampleApp_RegisterType_external() {
	app := conga.New[NetworkCLI]().
		RegisterType(parseCIDR).
		Args([]string{"--network", "192.168.1.7/24"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
	// Output: network=192.168.1.0/24
}

// A deprecated application warns on standard error, then runs normally.
func ExampleApp_Deprecated() {
	app := conga.New[DeprecatedCLI]().
		Name("oldtool").
		Deprecated("use newtool").
		Stderr(os.Stdout).
		Args([]string{})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
	// Output:
	// WARNING: oldtool is deprecated: use newtool
	// running
}

// Register a configuration decoder so flags resolve from a file named with
// --config-file.
func ExampleApp_RegisterDecoder() {
	app := conga.New[ConfigCLI]().
		RegisterDecoder(".toml", toml.Decode).
		Args([]string{"--config-file", "testdata/fixtures/example_config.toml"})

	if err := app.Run(); err != nil {
		panic(err)
	}

	// Output:
	// port=8080 host=localhost
}

// Supply a configuration document with a [conga.Bootstrapper]: the supplied
// document composes with file resolution, is validated like a decoded config
// file, outranks defaults, and still loses to command-line and environment
// values.
func ExampleBootstrapper() {
	app := conga.New[BootstrapCLI]().
		Args([]string{"--host", "cli.example.com"})

	if err := app.Run(); err != nil {
		panic(err)
	}

	// Output:
	// port=9200 host=cli.example.com
}

// Register a named predictor and reference it from a flag's predictor tag;
// shells complete the flag's values from the registered predictor.
func ExampleApp_Predictor() {
	app := conga.New[RegionCLI]().
		EnableCompletion().
		Predictor("regions", predict.Values("eu-west-1", "us-east-1", "us-west-2")).
		Args([]string{"__complete", "fish", "--region", ""})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output:
	// eu-west-1
	// us-east-1
	// us-west-2
}

// A flag type supplies its own completion candidates by implementing
// Predictable; declaring a flag of that type is all the wiring required.
func ExamplePredictable() {
	app := conga.New[PredictableCLI]().
		EnableCompletion().
		Args([]string{"__complete", "fish", "--cluster", ""})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output:
	// alpha
	// beta
	// gamma
}

// Register a map type so flags and arguments accept key=value pairs.
func ExampleApp_RegisterMap() {
	app := conga.New[ReplicasCLI]().
		RegisterMap[map[string]int]().
		Args([]string{"--replicas", "api=2,web=3"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: api=2 web=3
}

// Pass an existing struct pointer to inspect bound values after execution.
func ExampleNew_inspect() {
	cli := &InspectCLI{}
	app := conga.New(cli).
		Args([]string{"--port", "9000"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	fmt.Println(cli.Port.Get())
	// Output: 9000
}

// Counter counts repeated occurrences of a boolean flag on the command line.
func ExampleCounter() {
	app := conga.New[CounterCLI]().
		Args([]string{"-vvv"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: verbosity=3
}

// An omitted flag falls back to its default tag: set, but not changed.
func ExampleOptionalFlag_default() {
	app := conga.New[FlagStateCLI]().
		Args([]string{})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
	// Output: value=8080 set=true changed=false source=default
}

// A command-line value overrides the default and reports its source.
func ExampleOptionalFlag() {
	app := conga.New[FlagStateCLI]().
		Args([]string{"--port", "9000"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
	// Output: value=9000 set=true changed=true source=cli
}

// DashArgs captures arguments passed after the '--' delimiter without flag parsing.
func ExampleDashArgs() {
	app := conga.New[DashCLI]().
		Args([]string{"testpkg", "--", "-v", "--run", "TestFoo"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: testpkg: -v --run TestFoo
}

// RawArgs forwards trailing unparsed command-line arguments to proxy commands.
func ExampleRawArgs() {
	app := conga.New[RawCLI]().
		Args([]string{"sh", "-c", "echo hello"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: exec: sh -c echo hello
}

// ByteSize parses human-readable byte sizes and provides the raw byte count.
func ExampleByteSize() {
	app := conga.New[ByteSizeCLI]().
		Args([]string{"--limit", "1.5GB"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: limit=1500MB bytes=1500000000
}

// Use mounts timezone.RegisterLocation, which registers a parser and
// completion predictor for *time.Location through App.RegisterType.
func ExampleApp_Use() {
	app := conga.New[UseCLI]().
		Use(timezone.RegisterLocation).
		Args([]string{"--zone", "America/New_York"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: America/New_York
}

// Features declares feature toggles evaluated through field defaults,
// configuration, environment, and CLI flags. Enable with [conga.App.EnableFeatures].
func ExampleFeatures() {
	app := conga.New[BuildCLI]().
		EnableFeatures().
		Args([]string{
			"--feature-gates", "remote-cache=true,sandbox-isolation=2",
			"--cache-endpoint", "cas.internal:9092",
			"--container-image", "alpine:latest",
			"main",
		})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: remote-cache=true sandbox=2 endpoint=cas.internal:9092 image=alpine:latest target=main
}

// EnableFeatures activates feature toggles and gated flags.
func ExampleApp_EnableFeatures() {
	app := conga.New[BuildCLI]().
		EnableFeatures().
		Args([]string{"main"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: remote-cache=false sandbox=0 endpoint= image= target=main
}

// Map flags merge key by key across tiers: the command-line flag overrides
// retries and adds timeout, while unmentioned keys like env keep their default.
func ExampleOptionalFlag_mapMerge() {
	app := conga.New[MapMergeCLI]().
		Args([]string{"--settings", "retries=5,timeout=10s"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: env=prod retries=5 timeout=10s
}

// parseCIDR adapts net.ParseCIDR to the App.RegisterType parser signature.
// net.ParseCIDR itself returns three values (net.IP, *net.IPNet, error), so it
// cannot be passed to RegisterType, which takes func(string) (U, error).
func parseCIDR(raw string) (*net.IPNet, error) {
	_, network, err := net.ParseCIDR(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q: %w", raw, err)
	}

	return network, nil
}

// parsePercent parses a "0%".."100%" flag value, with or without the trailing
// percent sign, for App.RegisterType.
func parsePercent(raw string) (Percent, error) {
	digits := strings.TrimSuffix(raw, "%")

	value, err := strconv.ParseUint(digits, 10, 8)
	if err != nil || value > 100 {
		return 0, fmt.Errorf("invalid percentage %q: want 0%%-100%%", raw)
	}

	return Percent(value), nil
}
