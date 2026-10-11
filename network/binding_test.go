package network_test

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/network"
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
		Use(network.RegisterInterface)

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, targetName, cli.Iface.Get().Name)
}

func TestInterface_UnknownName(t *testing.T) {
	t.Parallel()

	app := conga.New[netTestCLI]().
		Args([]string{"-i", "no-such-interface-conga-test"}).
		Use(network.RegisterInterface)

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
		Stdout(&buffer).
		Use(network.RegisterInterface).
		Args([]string{"__complete", "export", "--iface", ""})

	err = app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `"value":"`+ifaces[0].Name+`"`)
}

func TestDefaultTypeHelp(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[netHelpCLI]().
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()
	assert.NoError(t, err)

	output := buffer.String()

	for _, testCase := range []struct {
		name string
		want string
	}{
		{"Host", "Host (hostname, domain, or IP address)"},
		{"HTTPMethod", "HTTP method (GET, POST, PUT, DELETE, ...)"},
		{"HTTPStatus", "HTTP status code (100-599, e.g. 200, 404)"},
		{"MIMEType", "MIME type (e.g. application/json)"},
		{"NetInterface", "Network interface (e.g. eth0, lo)"},
		{"Port", `Port (1-65535, or a service name like "http")`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, output, testCase.want)
		})
	}
}

type netHelpCLI struct {
	Host      conga.OptionalFlag[network.Host]
	Method    conga.OptionalFlag[network.HTTPMethod]
	Status    conga.OptionalFlag[network.HTTPStatus]
	MIME      conga.OptionalFlag[network.MIMEType]
	Interface conga.OptionalFlag[network.NetInterface]
	Port      conga.OptionalFlag[network.Port]
}

func (c *netHelpCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type netTestCLI struct {
	Iface conga.OptionalFlag[*net.Interface] `short:"i"`
}

func (c *netTestCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}
