// Package completionmulti pins the conservative degradation for packages with
// more than one conga.New anchor: per-anchor attribution is not attempted, so
// the completion guard never fires even though the second app never enables
// completion.
package completionmulti

import (
	"context"

	"github.com/conga-sh/conga"
)

// Alpha declares a completion command and enables completion.
type Alpha struct {
	Completion conga.CompletionCommand `summary:"Generate shell completion scripts"`
}

// Run executes Alpha.
func (a *Alpha) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// Beta declares a completion command without enabling completion.
type Beta struct {
	Completion conga.CompletionCommand `summary:"Generate shell completion scripts"`
}

// Run executes Beta.
func (b *Beta) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func runAlpha() {
	app := conga.New[Alpha]().EnableCompletion()
	app.Run()
}

func runBeta() {
	app := conga.New[Beta]()
	app.Run()
}
