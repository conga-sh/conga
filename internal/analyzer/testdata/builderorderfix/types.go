// Package builderorderfix contains auto-fixable builder call order violations;
// the suggested fixes must produce the sibling .golden files byte for byte.
package builderorderfix

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
