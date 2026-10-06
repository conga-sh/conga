// Package builderordervalid contains construction sites whose builder calls
// already follow the canonical order; the builder-order check must stay quiet.
package builderordervalid

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

// CLI is a leaf command implementing conga.Runner.
type CLI struct{}

// Run executes the CLI.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func orderedChain() {
	app := conga.New[CLI]().
		Name("ordered").
		Description("Every call sits in its canonical position").
		Version("1.0.0").
		Examples("ordered --help").
		HelpFooter("ordered docs").
		Deprecated("").
		DisableHelpFlag().
		DisableVersionFlag().
		EnableCompletion().
		EnableWhy().
		EnvPrefix("ORDERED").
		Dotenv().
		RegisterDecoder(".toml", nil).
		ConfigOptions().
		StrictTagValidation(false).
		AllowTags("custom").
		Predictor("names", predict.Values("alpha", "beta")).
		RegisterType(func(raw string) (int, error) {
			return 0, nil
		}).
		RegisterMap(map[string]int{}).
		Use().
		UsageOnError(conga.UsageHint).
		Args([]string{}).
		Stdin(nil).
		Stdout(nil).
		Stderr(nil).
		ExitFunc(nil)

	_ = app
}

func orderedStatements() {
	app := conga.New[CLI]()
	app.Name("ordered-statements")
	app.Version("1.0.0")
	app.EnableCompletion()
	app.Args([]string{})

	_ = app
}

func orderedInterleaved() {
	app := conga.New[CLI]()

	name := "ordered-interleaved"
	app.Name(name)
	app.Version("1.0.0")

	_ = app
}

func orderedInferredRoot() {
	cli := &CLI{}
	app := conga.New(cli).Name("ordered-inferred").Version("1.0.0")

	_ = app
	_ = cli
}

func orderedExecution() {
	app := conga.New[CLI]().Name("ordered-execution")

	app.Run()
	app.Version("1.0.0")
}
