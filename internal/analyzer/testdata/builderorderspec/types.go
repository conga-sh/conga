// Package builderorderspec exercises the -builder-order-spec override with a
// custom canonical order and with methods left unconstrained.
package builderorderspec

import (
	"context"

	"github.com/conga-sh/conga"
)

// CLI is a leaf command implementing conga.Runner.
type CLI struct{}

// Run executes the CLI.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}
