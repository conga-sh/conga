// Package sessionroot tests the Session.Root[T]() static verification rule.
package sessionroot

import (
	"context"

	"github.com/conga-sh/conga"
)

type WrongCLI struct{}

func (w *WrongCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type ChildCLI struct{}

func (c *ChildCLI) Run(ctx context.Context, session *conga.Session) error {
	_ = session.Root[WrongCLI]() // want `Session\.Root\[T\] type WrongCLI does not match application root command RootCLI`
	_ = session.Root[*RootCLI]() // want `Session\.Root\[T\] type \*RootCLI does not match application root command RootCLI`
	_ = session.Root[RootCLI]()

	return nil
}

type RootCLI struct {
	Child conga.Command[ChildCLI] `name:"child"`
}

func (r *RootCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[RootCLI]()
