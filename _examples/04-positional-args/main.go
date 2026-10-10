package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates required positionals, optional positionals, and trailing positional slices.
type CLI struct {
	Urgent conga.OptionalFlag[bool] `help:"Mark notification as high priority" short:"u"`

	Channel    conga.RequiredArg[string]   `enum:"slack,email,sms,webhook" help:"Target channel"`
	Message    conga.OptionalArg[string]   `help:"Broadcast message payload"            name:"[message]"`
	Recipients conga.OptionalArg[[]string] `help:"Additional recipient tags or handles" name:"[recipients...]"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	message := c.Message.GetOr("Ping health check")
	fmt.Fprintf(session.Stdout(), "Broadcast: channel=%s message=%q urgent=%t extra_recipients=%d\n",
		c.Channel.Get(), message, c.Urgent.Get(), len(c.Recipients.Get()))

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("notify").
		Description("Positional argument workflows (notification dispatcher)").
		Version("1.0.0")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
