package conga_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/predict"
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

// Declare flags as typed fields, implement [conga.Runner], and call [conga.App.Run].
func Example() {
	app := conga.New[CLI]().
		Name("greet").
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
		Name("alerts").
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
		Name("nettool").
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
