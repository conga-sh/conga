package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/predict/network"
)

// ProbeCmd sends an HTTP request probe to a target host or URL.
type ProbeCmd struct {
	Method      conga.OptionalFlag[network.HTTPMethod]   `default:"GET"                               help:"HTTP request method"                short:"X"`
	ContentType conga.OptionalFlag[network.MIMEType]     `default:"application/json"                  help:"Request Content-Type header"        short:"c"`
	Expect      conga.OptionalFlag[network.HTTPStatus]   `default:"200"                               help:"Expected HTTP response status code" short:"e"`
	Interface   conga.OptionalFlag[network.NetInterface] `help:"Bind to local network interface"      short:"i"`
	Resolve     conga.OptionalFlag[bool]                 `help:"Resolve the target host via DNS"      short:"r"`
	Host        conga.RequiredArg[network.Host]          `help:"Target server hostname or IP address"`
}

func (p *ProbeCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	expect := p.Expect.Get()

	fmt.Fprintf(session.Stdout(), "Probing %s via %s (expecting %s %s)\n",
		p.Host.Get(),
		p.Method.Get(),
		expect.String(),
		expect.Text(),
	)

	if p.Resolve.Get() {
		p.reportHostResolution(ctx, session)
	}

	if source := p.Interface.Get(); source != "" {
		if iface, err := source.Interface(); err != nil {
			fmt.Fprintf(session.Stdout(), "interface %s: %v\n", source, err)
		} else {
			fmt.Fprintf(session.Stdout(), "interface %s: mtu=%d\n", source, iface.MTU)
		}

		if addrs, err := source.Addrs(); err != nil {
			fmt.Fprintf(session.Stdout(), "interface %s addrs: %v\n", source, err)
		} else {
			fmt.Fprintf(session.Stdout(), "interface %s addrs: %d\n", source, len(addrs))
		}
	}

	return nil
}

// reportHostResolution resolves the target host and prints the resulting
// addresses.
func (p *ProbeCmd) reportHostResolution(ctx context.Context, session *conga.Session) {
	host := p.Host.Get()

	if ips, err := host.LookupIP(ctx); err != nil {
		fmt.Fprintf(session.Stdout(), "resolve %s: %v\n", host, err)
	} else {
		fmt.Fprintf(session.Stdout(), "resolve %s: %d addresses\n", host, len(ips))
	}
}
