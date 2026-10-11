package main

import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

// Profile demonstrates self-completing global cloud profiles via predict.Predictable.
type Profile string

func (p Profile) Predict() predict.Predictor {
	return predict.Items(
		predict.NewItem("production", "Production workloads"),
		predict.NewItem("staging", "Pre-production staging cluster"),
		predict.NewItem("development", "Local sandbox environment"),
	)
}

// CLI represents a cloud management CLI. The deploy subcommand uses the
// recommended self-completing domain types (Predictable, FileFilterable,
// conga.Dir, conga.File); the exec and logs subcommands use the predictor
// registry (app.Predictor + predictor:"..." struct tags).
type CLI struct {
	Profile    conga.OptionalFlag[Profile]    `default:"staging" help:"Global cloud profile" short:"p"`
	Kubeconfig conga.OptionalFlag[conga.File] `short:"k"`

	Deploy     conga.Command[DeployCmd] `aliases:"d"                                  summary:"Deploy service to a cluster"`
	Logs       conga.Command[LogsCmd]   `summary:"Stream and export service logs"`
	Exec       conga.Command[ExecCmd]   `summary:"Execute interactive command in pod"`
	Completion conga.CompletionCommand
	Help       conga.HelpCommand `summary:"Show help for any command"`
}
