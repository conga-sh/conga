package main

import (
	"github.com/conga-sh/conga"
)

// NetworkCmd groups the network subcommands.
type NetworkCmd struct {
	Probe conga.Command[ProbeCmd] `aliases:"p"                                              summary:"Send an HTTP probe request to a target host"`
	Scan  conga.Command[ScanCmd]  `aliases:"s"                                              summary:"Scan open network service ports on a host"`
	Serve conga.Command[ServeCmd] `summary:"Describe the mock HTTP responder configuration"`
}
