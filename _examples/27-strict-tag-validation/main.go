package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI mixes CONGA tags with third-party struct tags owned by the
// application's validation and persistence layers.
type CLI struct {
	Email   conga.OptionalFlag[string] `db:"email"   help:"Contact email" validate:"required,email"`
	Retries conga.OptionalFlag[int]    `db:"retries" default:"3"          help:"Retry attempts"     validate:"gte=0"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("strict-tags").
		Description("Registering third-party struct tags with app.AllowTags").
		// Tag validation is always on and rejects unknown tag keys with an
		// ErrSchema error. The db and validate keys below belong to other
		// libraries, so the app registers them explicitly. Registered keys
		// are matched exactly, so a misspelling still fails schema build.
		AllowTags("db", "validate")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
