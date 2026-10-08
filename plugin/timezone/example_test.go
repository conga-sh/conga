package timezone_test

import (
	"context"
	"fmt"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/plugin/timezone"
)

// CLI is an application whose flag accepts an IANA timezone identifier.
type CLI struct {
	Zone conga.OptionalFlag[*time.Location] `help:"Server timezone"`
}

func (c *CLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintln(session.Stdout(), c.Zone.Get())

	return err
}

// Location mounts *time.Location support: the flag accepts an IANA timezone
// identifier and completion offers the host's zones.
func ExampleLocation() {
	app := conga.New[CLI]().
		Use(timezone.Location).
		Args([]string{"--zone", "America/New_York"})

	if err := app.Run(); err != nil {
		panic(err)
	}

	// Output: America/New_York
}
