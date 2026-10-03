package conga_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/conga-sh/conga"
)

type exampleGreetCLI struct {
	Name conga.OptionalFlag[string] `default:"world" help:"Who to greet"`
}

func (c *exampleGreetCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "Hello, %s!\n", c.Name.Get())

	return err
}

// Declare flags as typed fields, implement [conga.Runner], and call [conga.App.Run].
func Example() {
	app := conga.New[exampleGreetCLI]().
		Name("greet").
		Args([]string{"--name", "gopher"})

	if err := app.Run(); err != nil {
		fmt.Println("error:", err)
	}
	// Output: Hello, gopher!
}

type exampleRootCLI struct {
	Verbose conga.OptionalFlag[bool]         `help:"Verbose output"`
	Migrate conga.Command[exampleMigrateCLI] `summary:"Run database migrations"`
}

type exampleMigrateCLI struct{}

func (c *exampleMigrateCLI) Run(_ context.Context, session *conga.Session) error {
	root := session.Root[exampleRootCLI]()
	_, err := fmt.Fprintf(session.Stdout(), "%s verbose=%t\n", session.Path(), root.Verbose.Get())

	return err
}

// A subcommand reads global flags declared on the root command through
// [conga.Session.Root].
func ExampleSession_Root() {
	app := conga.New[exampleRootCLI]().
		Name("tool").
		Args([]string{"--verbose", "migrate"})

	if err := app.Run(); err != nil {
		fmt.Println("error:", err)
	}
	// Output: tool migrate verbose=true
}

// examplePercent is a custom flag type parsed from values such as "75%".
type examplePercent int

type examplePercentCLI struct {
	Threshold conga.OptionalFlag[examplePercent] `default:"50%" help:"Alert threshold"`
}

func (c *examplePercentCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintf(session.Stdout(), "threshold=%d\n", c.Threshold.Get())

	return err
}

func exampleParsePercent(raw string) (examplePercent, error) {
	value, err := strconv.Atoi(strings.TrimSuffix(raw, "%"))
	if err != nil {
		return 0, fmt.Errorf("invalid percentage %q: %w", raw, err)
	}

	return examplePercent(value), nil
}

// Register a parser so flags and arguments of a custom type accept its text form.
func ExampleApp_RegisterType() {
	app := conga.New[examplePercentCLI]().
		Name("alerts").
		RegisterType(exampleParsePercent).
		Args([]string{"--threshold", "75%"})

	if err := app.Run(); err != nil {
		fmt.Println("error:", err)
	}
	// Output: threshold=75
}

type exampleDeprecatedCLI struct{}

func (c *exampleDeprecatedCLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintln(session.Stdout(), "running")

	return err
}

// A deprecated application warns on standard error, then runs normally.
func ExampleApp_Deprecated() {
	app := conga.New[exampleDeprecatedCLI]().
		Name("oldtool").
		Deprecated("use newtool").
		Stderr(os.Stdout).
		Args([]string{})

	if err := app.Run(); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// WARNING: oldtool is deprecated: use newtool
	// running
}
