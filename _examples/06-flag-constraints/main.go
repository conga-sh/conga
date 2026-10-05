package main

import (
	"context"
	"fmt"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates declarative tag-based constraints (XOR, and, range, len, items, enum)
// and mandatory flags via conga.RequiredFlag.
type CLI struct {
	// Mutually Exclusive Flags (XOR group)
	JSON  conga.OptionalFlag[bool] `help:"Output in JSON format"  xor:"format"`
	YAML  conga.OptionalFlag[bool] `help:"Output in YAML format"  xor:"format"`
	Table conga.OptionalFlag[bool] `help:"Output in Table format" xor:"format"`

	// Mandatory Flag (required)
	APIKey conga.RequiredFlag[string] `help:"API key for authentication (required)" short:"k"`

	// Co-requisite Flags (and group)
	Username conga.OptionalFlag[string] `and:"auth" help:"Authentication username" len:"3..20" short:"u"`
	Password conga.OptionalFlag[string] `and:"auth" help:"Authentication password" len:"8.."`

	// Numeric Range Constraints
	Port    conga.OptionalFlag[int]            `default:"8080"   help:"Port number"     range:"1..65535"  short:"p"`
	Timeout conga.OptionalFlag[time.Duration]  `default:"2s"     help:"Request timeout" range:"100ms..5s"`
	Memory  conga.OptionalFlag[conga.ByteSize] `default:"256MiB" range:"1MiB..1GiB"`

	// Enum Constraint
	Env conga.OptionalFlag[string] `default:"dev" enum:"dev,staging,prod" help:"Deployment environment"`

	// Collection Bounds Constraint
	Zones conga.OptionalFlag[[]string] `help:"Availability zones" items:"2..3" short:"z"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	format := "default"
	if c.JSON.Get() {
		format = "json"
	} else if c.YAML.Get() {
		format = "yaml"
	} else if c.Table.Get() {
		format = "table"
	}

	memoryLimit := c.Memory.Get()

	fmt.Fprintf(session.Stdout(), "Validated Constraints: format=%s env=%s port=%d timeout=%s memory=%s zones=%d api_key_source=%s\n",
		format, c.Env.Get(), c.Port.Get(), c.Timeout.Get(), memoryLimit.String(), len(c.Zones.Get()), c.APIKey.Source())

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("constraints-demo").
		Description("Declarative safety constraints (XOR, And, Range, Len, Items, Enum)").
		Version("1.0.0").
		EnableCompletion().
		UsageOnError(conga.UsageFull)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
