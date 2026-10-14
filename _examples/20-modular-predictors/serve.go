package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/network"
)

// ServeCmd describes the configuration of a mock HTTP test responder.
type ServeCmd struct {
	Port        conga.OptionalFlag[network.Port]       `default:"8080"             help:"Listen port"                  short:"p"`
	Status      conga.OptionalFlag[network.HTTPStatus] `default:"200"              help:"Default response status code" short:"s"`
	ContentType conga.OptionalFlag[network.MIMEType]   `default:"application/json" help:"Response Content-Type header" short:"c"`
}

func (s *ServeCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), s)

	port := s.Port.Get()
	status := s.Status.Get()
	contentType := s.ContentType.Get()

	fmt.Fprintf(session.Stdout(), "Mock responder would listen on port %d (returning %d %s, %s)\n",
		port.Int(),
		status.Code(),
		status.Text(),
		contentType.String(),
	)
	fmt.Fprintf(session.Stdout(), "port privileged: %t\n", port.IsPrivileged())
	fmt.Fprintf(session.Stdout(), "status class: success=%t client_error=%t server_error=%t\n",
		status.IsSuccess(), status.IsClientError(), status.IsServerError())
	fmt.Fprintf(session.Stdout(), "media: type=%s subtype=%s\n", contentType.Type(), contentType.Subtype())

	return nil
}
