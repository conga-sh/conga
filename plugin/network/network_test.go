package network_test

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/plugin/network"
)

func TestInterface(t *testing.T) {
	t.Parallel()

	ifaces, err := net.Interfaces()
	assert.NoError(t, err)

	if len(ifaces) == 0 {
		t.Skip("no network interfaces available")
	}

	targetName := ifaces[0].Name

	cli := new(netTestCLI)
	app := conga.New(cli).
		Args([]string{"-i", targetName}).
		Use(network.Interface)

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, targetName, cli.Iface.Get().Name)
}

func TestInterface_UnknownName(t *testing.T) {
	t.Parallel()

	app := conga.New[netTestCLI]().
		Args([]string{"-i", "no-such-interface-conga-test"}).
		Use(network.Interface)

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), "for flag --iface")
	assert.Contains(t, err.Error(), "no such network interface")
}

func TestInterface_Completion(t *testing.T) {
	t.Parallel()

	ifaces, err := net.Interfaces()
	assert.NoError(t, err)

	if len(ifaces) == 0 {
		t.Skip("no network interfaces available")
	}

	var buffer bytes.Buffer

	app := conga.New[netTestCLI]().
		EnableCompletion().
		Name("app").
		Stdout(&buffer).
		Use(network.Interface).
		Args([]string{"__complete", "export", "--iface", ""})

	err = app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `"value":"`+ifaces[0].Name+`"`)
}

type netTestCLI struct {
	Iface conga.OptionalFlag[*net.Interface] `short:"i"`
}

func (c *netTestCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}
