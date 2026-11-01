package main

import "github.com/conga-sh/conga"

// CLI is the root CLI.
type CLI struct {
	Verbose conga.OptionalFlag[bool] `help:"Enable verbose output" short:"v"`

	Create conga.Command[CreateCmd] `section:"Lifecycle"  summary:"Create a new service"`
	Delete conga.Command[DeleteCmd] `aliases:"rm,del"     section:"Lifecycle"              summary:"Delete an existing service"`
	List   conga.Command[ListCmd]   `section:"Inspection" summary:"List deployed services"`

	// Hidden housekeeping command: invocable by those who know it exists, but
	// omitted from --help and shell completion candidates.
	Doctor conga.Command[DoctorCmd] `hidden:"true" summary:"Run internal consistency checks"`
}
