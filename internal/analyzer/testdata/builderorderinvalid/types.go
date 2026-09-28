// Package builderorderinvalid contains construction sites that violate the
// canonical builder call order, one violating site per source file.
package builderorderinvalid

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
