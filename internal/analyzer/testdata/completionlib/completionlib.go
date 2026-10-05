// Package completionlib declares a reusable CLI struct for the
// completionforeign fixture: the app that owns it is constructed in another
// package.
package completionlib

import (
	"context"

	"github.com/conga-sh/conga"
)

// Root declares a completion command; the owning app must enable completion.
type Root struct {
	Completion conga.CompletionCommand `summary:"Generate shell completion scripts"`
}

// Run executes the root.
func (r *Root) Run(ctx context.Context, session *conga.Session) error {
	return nil
}
