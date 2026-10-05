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
		Description("Opting out of strict struct-tag linting for third-party tags").
		// Strict tag linting is on by default and rejects unknown tag keys.
		// The validate/db tags below belong to other libraries, so the whole
		// schema opts out. Prefer AllowTags(...) or a per-field
		// conga:"allow-unknown-tags" directive when only a few fields carry
		// foreign tags.
		StrictTagValidation(false)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
