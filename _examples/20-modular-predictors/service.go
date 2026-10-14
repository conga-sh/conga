package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/system"
)

// ServiceCmd configures and runs a background system daemon.
type ServiceCmd struct {
	Name    conga.RequiredArg[string]         `help:"Service daemon name"`
	User    conga.OptionalFlag[system.User]   `help:"System user to run daemon under"       short:"u"`
	Group   conga.OptionalFlag[system.Group]  `help:"System user group to run daemon under" short:"g"`
	EnvVar  conga.OptionalFlag[system.EnvVar] `help:"Environment variable to forward"       short:"v"`
	Locale  conga.OptionalFlag[system.Locale] `default:"en_US.UTF-8"                        help:"Service execution locale" short:"l"`
	PassEnv conga.OptionalFlag[system.EnvVar] `help:"Pass-through environment variable"`
}

func (s *ServiceCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), s)

	if user := s.User.Get(); user != "" {
		if account, err := user.Lookup(); err != nil {
			fmt.Fprintf(session.Stdout(), "user %s: %v\n", user, err)
		} else {
			fmt.Fprintf(session.Stdout(), "user: uid=%s gid=%s\n", account.Uid, account.Gid)
		}
	}

	if group := s.Group.Get(); group != "" {
		if resolved, err := group.Lookup(); err != nil {
			fmt.Fprintf(session.Stdout(), "group %s: %v\n", group, err)
		} else {
			fmt.Fprintf(session.Stdout(), "group: gid=%s\n", resolved.Gid)
		}
	}

	if envVar := s.EnvVar.Get(); envVar != "" {
		value, ok := envVar.Lookup()

		fmt.Fprintf(session.Stdout(), "env %s: set=%t value=%s\n", envVar, ok, value)
	}

	if passEnv := s.PassEnv.Get(); passEnv != "" {
		fmt.Fprintf(session.Stdout(), "pass-env %s=%s\n", passEnv, passEnv.Value())
	}

	return nil
}
