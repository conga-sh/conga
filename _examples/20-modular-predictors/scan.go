package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/network"
)

// ScanCmd scans open network ports on a target host.
type ScanCmd struct {
	Ports     conga.OptionalFlag[[]network.Port]       `default:"80,443,22"                         help:"Port list to scan" short:"p"`
	Interface conga.OptionalFlag[network.NetInterface] `help:"Bind to source interface"             short:"i"`
	Host      conga.RequiredArg[network.Host]          `help:"Target server hostname or IP address"`
}

func (s *ScanCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), s)

	return nil
}
