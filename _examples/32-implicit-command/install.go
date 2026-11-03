package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

// InstallCmd demonstrates an explicit sibling command.
type InstallCmd struct {
	Frozen conga.OptionalFlag[bool] `help:"Skip network access"`
}

func (c *InstallCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "install frozen=%t\n", c.Frozen.Get())

	return nil
}
