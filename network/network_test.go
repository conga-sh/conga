package network_test

import (
	"bytes"
	"context"
	"encoding"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/network"
	"github.com/conga-sh/conga/predict"
)

var (
	// System file payloads in the formats the network predictors parse.
	//go:embed testdata/fixtures/hosts
	fixtureHosts []byte

	//go:embed testdata/fixtures/known_hosts
	fixtureKnownHosts []byte

	//go:embed testdata/fixtures/ssh_config
	fixtureSSHConfig []byte
)

func TestNetInterfaces_Execution(t *testing.T) {
	t.Parallel()

	pred := network.NetInterfaces()

	result := pred.Invoke(predict.Context{})

	assert.True(t, len(result.Items) > 0, "expected at least one network interface")

	for _, item := range result.Items {
		assert.Equal(t, "network interfaces", item.Tag)
		assert.True(t, item.Style == "green" || item.Style == "blue" || item.Style == "yellow")
		assert.Equal(t, item.Value, item.Display)
	}
}

func TestHosts_ParseEtcHosts(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	assert.NoError(t, os.MkdirAll(sshDir, 0700))
	assert.NoError(t, os.WriteFile(filepath.Join(sshDir, "known_hosts"), fixtureKnownHosts, 0600))
	assert.NoError(t, os.WriteFile(filepath.Join(sshDir, "config"), fixtureSSHConfig, 0600))

	pred := network.Hosts()

	result := pred.Invoke(predict.Context{
		Env: map[string]string{
			"HOME": tmpDir,
		},
	})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "bastion"), "expected SSH config host bastion")
	assert.True(t, slices.Contains(values, "prod-api"), "expected SSH config host prod-api")
	assert.True(t, slices.Contains(values, "prod-web"), "expected SSH config host prod-web")
	assert.True(t, slices.Contains(values, "github.com"), "expected SSH known_host github.com")
	assert.True(t, slices.Contains(values, "gitlab.com"), "expected SSH known_host gitlab.com")
	assert.True(t, slices.Contains(values, "ssh.dev.local"), "expected unbracketed SSH host ssh.dev.local")
	assert.True(t, slices.Contains(values, "*.example.com"), "expected host from @cert-authority line")
	assert.False(t, slices.Contains(values, "@cert-authority"), "marker token must not be emitted as host")
	assert.False(t, slices.Contains(values, "revoked.example.com"), "@revoked host must not be emitted")
	assert.False(t, slices.Contains(values, "@revoked"), "marker token must not be emitted as host")
	assert.False(t, slices.Contains(values, "*"), "wildcard Host patterns must be excluded")
	assert.False(t, slices.Contains(values, "*.internal"), "wildcard Host patterns must be excluded")
	assert.True(t, len(fixtureHosts) > 0, "hosts fixture must be valid")
	assert.True(t, bytes.Contains(fixtureHosts, []byte("localhost")))
}

func TestPorts_StandardEntries(t *testing.T) {
	t.Parallel()

	pred := network.Ports()

	result := pred.Invoke(predict.Context{})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "80"))
	assert.True(t, slices.Contains(values, "443"))
	assert.True(t, slices.Contains(values, "22"))
	assert.True(t, slices.Contains(values, "5432"))
	assert.True(t, slices.Contains(values, "6379"))
	assert.True(t, slices.Contains(values, "8080"))

	for _, item := range result.Items {
		assert.Equal(t, "ports", item.Tag)

		switch item.Value {
		case "80":
			assert.Equal(t, "http (Hypertext Transfer Protocol)", item.Description)
			assert.Equal(t, "blue", item.Style)
		case "443":
			assert.Equal(t, "https (HTTP over TLS/SSL)", item.Description)
			assert.Equal(t, "blue", item.Style)
		case "5432":
			assert.Equal(t, "postgresql (PostgreSQL Database)", item.Description)
			assert.Equal(t, "green", item.Style)
		}
	}
}

func TestHTTPStatusCodes_Execution(t *testing.T) {
	t.Parallel()

	pred := network.HTTPStatusCodes()

	result := pred.Invoke(predict.Context{})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "200"))
	assert.True(t, slices.Contains(values, "201"))
	assert.True(t, slices.Contains(values, "400"))
	assert.True(t, slices.Contains(values, "404"))
	assert.True(t, slices.Contains(values, "500"))

	for _, item := range result.Items {
		assert.Equal(t, "http status codes", item.Tag)

		switch item.Value {
		case "200":
			assert.Equal(t, "OK", item.Description)
			assert.Equal(t, "green", item.Style)
		case "404":
			assert.Equal(t, "Not Found", item.Description)
			assert.Equal(t, "red", item.Style)
		case "500":
			assert.Equal(t, "Internal Server Error", item.Description)
			assert.Equal(t, "red", item.Style)
		}
	}
}

func TestMIMETypes_Execution(t *testing.T) {
	t.Parallel()

	pred := network.MIMETypes()

	result := pred.Invoke(predict.Context{})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "application/json"))
	assert.True(t, slices.Contains(values, "text/html"))
	assert.True(t, slices.Contains(values, "image/png"))
	assert.True(t, slices.Contains(values, "multipart/form-data"))

	for _, item := range result.Items {
		assert.Equal(t, "media types", item.Tag)

		if item.Value == "application/json" {
			assert.Equal(t, "JavaScript Object Notation", item.Description)
		}
	}
}

func TestHTTPMethods_Execution(t *testing.T) {
	t.Parallel()

	pred := network.HTTPMethods()

	result := pred.Invoke(predict.Context{})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "GET"))
	assert.True(t, slices.Contains(values, "POST"))
	assert.True(t, slices.Contains(values, "PUT"))
	assert.True(t, slices.Contains(values, "PATCH"))
	assert.True(t, slices.Contains(values, "DELETE"))
	assert.True(t, slices.Contains(values, "HEAD"))
	assert.True(t, slices.Contains(values, "OPTIONS"))

	for _, item := range result.Items {
		assert.Equal(t, "http methods", item.Tag)

		switch item.Value {
		case "GET":
			assert.Equal(t, "green", item.Style)
		case "POST":
			assert.Equal(t, "blue", item.Style)
		case "PUT":
			assert.Equal(t, "yellow", item.Style)
		case "PATCH":
			assert.Equal(t, "magenta", item.Style)
		case "DELETE":
			assert.Equal(t, "red", item.Style)
		}
	}
}

func TestDomainTypes(t *testing.T) {
	t.Parallel()

	t.Run("NetInterface domain type", func(t *testing.T) {
		t.Parallel()

		var netif network.NetInterface

		err := netif.UnmarshalText([]byte("lo"))

		assert.NoError(t, err)
		assert.Equal(t, "lo", netif.String())
		assert.NotZero(t, netif.Predict())

		var emptyNetif network.NetInterface

		_, err = emptyNetif.Interface()

		assert.Error(t, err)
		assert.Equal(t, "network interface: empty interface name", err.Error())

		var badNetif network.NetInterface = "nonexistent_iface_999"

		_, err = badNetif.Interface()

		assert.Error(t, err)

		_, err = badNetif.Addrs()

		assert.Error(t, err)

		ifaces, err := net.Interfaces()
		if err == nil && len(ifaces) > 0 {
			realNetif := network.NetInterface(ifaces[0].Name)
			realIface, err := realNetif.Interface()

			assert.NoError(t, err)
			assert.Equal(t, ifaces[0].Name, realIface.Name)

			addrs, err := realNetif.Addrs()

			assert.NoError(t, err)
			assert.NotZero(t, addrs)
		}
	})

	t.Run("Host domain type", func(t *testing.T) {
		t.Parallel()

		var host network.Host

		err := host.UnmarshalText([]byte("localhost"))

		assert.NoError(t, err)
		assert.Equal(t, "localhost", host.String())
		assert.NotZero(t, host.Predict())

		var emptyHost network.Host

		_, err = emptyHost.LookupIP(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "lookup host: empty host name", err.Error())

		var ipHost network.Host = "127.0.0.1"

		ips, err := ipHost.LookupIP(t.Context())

		assert.NoError(t, err)
		assert.True(t, len(ips) > 0)
	})

	t.Run("Port domain type", func(t *testing.T) {
		t.Parallel()

		var port network.Port

		err := port.UnmarshalText([]byte("80"))

		assert.NoError(t, err)
		assert.Equal(t, 80, port.Int())
		assert.True(t, port.IsPrivileged())
		assert.Equal(t, "80", port.String())
		assert.NotZero(t, port.Predict())

		var httpPort network.Port

		err = httpPort.UnmarshalText([]byte("http"))

		assert.NoError(t, err)
		assert.Equal(t, 80, httpPort.Int())

		var unprivPort network.Port

		err = unprivPort.UnmarshalText([]byte("8080"))

		assert.NoError(t, err)
		assert.False(t, unprivPort.IsPrivileged())

		var invalidPort network.Port

		err = invalidPort.UnmarshalText([]byte("99999"))

		assert.Error(t, err)
	})

	t.Run("HTTPStatus domain type", func(t *testing.T) {
		t.Parallel()

		var status network.HTTPStatus

		err := status.UnmarshalText([]byte("200"))

		assert.NoError(t, err)
		assert.Equal(t, 200, status.Code())
		assert.Equal(t, "OK", status.Text())
		assert.True(t, status.IsSuccess())
		assert.False(t, status.IsClientError())
		assert.False(t, status.IsServerError())
		assert.Equal(t, "200", status.String())
		assert.NotZero(t, status.Predict())

		var clientStatus network.HTTPStatus

		err = clientStatus.UnmarshalText([]byte("404"))

		assert.NoError(t, err)
		assert.True(t, clientStatus.IsClientError())
		assert.Equal(t, "Not Found", clientStatus.Text())

		var serverStatus network.HTTPStatus

		err = serverStatus.UnmarshalText([]byte("500"))

		assert.NoError(t, err)
		assert.True(t, serverStatus.IsServerError())
		assert.Equal(t, "Internal Server Error", serverStatus.Text())

		var namedStatus network.HTTPStatus

		err = namedStatus.UnmarshalText([]byte("Created"))

		assert.NoError(t, err)
		assert.Equal(t, 201, namedStatus.Code())

		var invalidStatus network.HTTPStatus

		err = invalidStatus.UnmarshalText([]byte("999"))

		assert.Error(t, err)
	})

	t.Run("MIMEType domain type", func(t *testing.T) {
		t.Parallel()

		var mime network.MIMEType

		err := mime.UnmarshalText([]byte("application/json; charset=utf-8"))

		assert.NoError(t, err)
		assert.Equal(t, "application", mime.Type())
		assert.Equal(t, "json", mime.Subtype())
		assert.Equal(t, "application/json; charset=utf-8", mime.String())
		assert.NotZero(t, mime.Predict())

		var invalidMime network.MIMEType

		err = invalidMime.UnmarshalText([]byte("not-a-mime"))

		assert.Error(t, err)
	})

	t.Run("HTTPMethod domain type", func(t *testing.T) {
		t.Parallel()

		var method network.HTTPMethod

		err := method.UnmarshalText([]byte("post"))

		assert.NoError(t, err)
		assert.Equal(t, "POST", method.String())
		assert.NotZero(t, method.Predict())

		var invalidMethod network.HTTPMethod

		err = invalidMethod.UnmarshalText([]byte("INVALID_METHOD"))

		assert.Error(t, err)
	})
}

func TestDomainTypes_MarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    encoding.TextMarshaler
		expected string
	}{
		{"Host", network.Host("example.com"), "example.com"},
		{"HTTPMethod", network.HTTPMethod("POST"), "POST"},
		{"HTTPStatus", network.HTTPStatus(200), "200"},
		{"MIMEType", network.MIMEType("application/json"), "application/json"},
		{"NetInterface", network.NetInterface("lo"), "lo"},
		{"Port", network.Port(8080), "8080"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := testCase.input.MarshalText()

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, string(encoded))
		})
	}
}

type chainingCLI struct {
	Port   conga.OptionalFlag[network.Port]       `short:"p"`
	Status conga.OptionalFlag[network.HTTPStatus] `short:"s"`
	Method conga.OptionalFlag[network.HTTPMethod] `short:"m"`
}

func (c *chainingCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestDomainTypes_ValueReceiverChaining(t *testing.T) {
	t.Parallel()

	cli := &chainingCLI{}
	app := conga.New(cli).Args([]string{"--port", "80", "--status", "204", "--method", "get"})

	err := app.Run()

	assert.NoError(t, err)

	assert.True(t, cli.Port.Get().IsPrivileged())
	assert.Equal(t, 80, cli.Port.Get().Int())
	assert.Equal(t, "80", cli.Port.Get().String())

	assert.True(t, cli.Status.Get().IsSuccess())
	assert.False(t, cli.Status.Get().IsClientError())
	assert.False(t, cli.Status.Get().IsServerError())
	assert.Equal(t, 204, cli.Status.Get().Code())
	assert.Equal(t, "No Content", cli.Status.Get().Text())

	assert.Equal(t, "GET", cli.Method.Get().String())
}

func TestRegisterNetInterface_CoercesInterface(t *testing.T) {
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
		Use(network.RegisterNetInterface)

	err = app.Run()

	assert.NoError(t, err)

	iface := cli.Iface.Get()
	assert.NotZero(t, iface)
	assert.Equal(t, targetName, iface.Name)
}

func itemValues(items []predict.Item) []string {
	return slicex.Map(items, func(item predict.Item) string { return item.Value })
}
