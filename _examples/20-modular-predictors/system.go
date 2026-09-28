package main

import (
	"github.com/conga-sh/conga"
)

// SystemCmd groups the operating-system subcommands.
type SystemCmd struct {
	Kill    conga.Command[KillCmd]    `aliases:"k" summary:"Send a signal to a process or PID"`
	Service conga.Command[ServiceCmd] `aliases:"s" summary:"Manage system service daemon configurations"`
	Inspect conga.Command[InspectCmd] `aliases:"i" summary:"Inspect running process information"`
}
