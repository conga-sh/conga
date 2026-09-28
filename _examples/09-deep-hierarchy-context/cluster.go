package main

import "github.com/conga-sh/conga"

type ClusterCmd struct {
	ClusterName conga.OptionalFlag[string] `default:"default-cluster" help:"Target cluster name" short:"c"`
	Node        conga.Command[NodeCmd]     `summary:"Node operations"`
}
