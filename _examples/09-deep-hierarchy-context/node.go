package main

import "github.com/conga-sh/conga"

type NodeCmd struct {
	NodeID conga.OptionalFlag[string] `default:"node-01"        help:"Target node ID" short:"n"`
	Pod    conga.Command[PodCmd]      `summary:"Pod operations"`
}
