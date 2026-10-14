package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// ExecCmd uses the registry completion style: plain built-in types wired to
// named predictors via predictor:"..." struct tags, with the predictors
// themselves registered in main.go.
type ExecCmd struct {
	Namespace conga.OptionalFlag[string] `default:"backend"                                            enum:"frontend,backend,database"                help:"Target namespace (subcommand flag)" short:"n"`
	Container conga.OptionalFlag[string] `help:"Non-sidecar container within the pod"                  predictor:"containers"                          short:"c"`
	Target    conga.RequiredArg[string]  `help:"Target pod name (contextual: --profile + --namespace)" predictor:"pods"`
	Command   conga.OptionalArg[string]  `default:"/bin/sh"                                            help:"Command to execute (per-pod suggestions)" predictor:"commands"`
}

func (e *ExecCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), e)

	return nil
}
