package main

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI is flowctl: it runs named pipelines from one or more pipeline files. The
// files' top-level configuration keys feed the config tier through the
// Bootstrap method; the pipeline definitions themselves stay domain data.
type CLI struct {
	File     conga.OptionalFlag[[]string] `bootstrap:"true" default:"pipeline.yaml" help:"Pipeline files (later files win)" safe:"true" short:"f"`
	Pipeline conga.OptionalFlag[string]   `bootstrap:"true" help:"Pipeline to run"  safe:"true"`

	Parallelism conga.OptionalFlag[int]               `default:"1"                 help:"Parallel steps"        safe:"true"`
	LogLevel    conga.OptionalFlag[string]            `default:"warn"              enum:"debug,info,warn,error" help:"Log level" safe:"true"`
	Env         conga.OptionalFlag[map[string]string] `help:"Pipeline environment"`

	pipelines map[string]mergedPipeline
}

// Run prints the resolved configuration and the selected pipeline's merged
// plan, or lists the available pipelines when none is selected.
func (c *CLI) Run(_ context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	name := c.Pipeline.Get()
	if name == "" {
		for _, pipelineName := range slices.Sorted(maps.Keys(c.pipelines)) {
			fmt.Fprintf(session.Stdout(), "%-10s %s\n", pipelineName, c.pipelines[pipelineName].desc)
		}

		return nil
	}

	pipeline := c.pipelines[name]
	fmt.Fprintf(session.Stdout(), "pipeline %s: needs=%v steps=%v\n", name, pipeline.needs, pipeline.steps)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("flowctl").
		Description("Run named pipelines whose files supply the configuration tier").
		Version("1.0.0").
		EnableWhy().
		EnvPrefix("FLOWCTL")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
