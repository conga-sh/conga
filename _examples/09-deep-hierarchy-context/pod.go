package main

import (
	"context"
	"fmt"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

type PodCmd struct {
	Wait conga.OptionalFlag[time.Duration] `default:"5s"     help:"Wait for list consistency"         short:"w"`
	List conga.Command[PodListCmd]         `envprefix:"POD_" summary:"List pods running on this node"`
}

type PodListCmd struct {
	Status conga.OptionalFlag[string] `default:"running" help:"Filter pods by status" short:"s"`
}

func (p *PodListCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	// Ancestor accessors: read the root through session.Root, then walk the
	// selected Command fields down to any enclosing command struct.
	root := session.Root[CLI]()
	cluster := root.Cluster.Get()
	node := cluster.Node.Get()
	podParent := node.Pod.Get()

	stdout := session.Stdout()
	fmt.Fprintf(stdout, "=== Deep Hierarchy Resolution ===\n")
	fmt.Fprintf(stdout, "Full Command Path:    %s\n", session.Path())
	fmt.Fprintf(stdout, "Active Leaf Name:     %s\n", session.Name())
	fmt.Fprintf(stdout, "Root Global Region:   %s\n", root.GlobalRegion.Get())
	fmt.Fprintf(stdout, "Ancestor Cluster:     %s\n", cluster.ClusterName.Get())
	fmt.Fprintf(stdout, "Ancestor Node:        %s\n", node.NodeID.Get())
	fmt.Fprintf(stdout, "Parent Pod Wait:      %s\n", podParent.Wait.Get())
	fmt.Fprintf(stdout, "Leaf Status Filter:   %s\n", p.Status.Get())

	return nil
}
