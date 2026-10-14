// Package network provides domain types and autocompletion predictors for
// network interfaces, hosts, ports, HTTP status codes, MIME types, and HTTP
// methods, plus an optional standard library binding for *net.Interface fields.
//
// Every exported domain type implements encoding.TextUnmarshaler and Predict:
// fields declared with these types coerce automatically and complete from
// package tables or live system state. Invalid values classify under
// conga.ErrCoerce with the flag and reason named in the message. Host.LookupIP
// and NetInterface.Interface report a blank receiver as an error naming the
// failed lookup.
//
// Mount [RegisterInterface] to register *net.Interface with net.InterfaceByName
// and enable interface completion on fields typed with the standard library
// type:
//
//	type CLI struct {
//		Iface conga.OptionalFlag[network.NetInterface] `short:"i" help:"Network interface"`
//		Peer  conga.OptionalFlag[*net.Interface]       `short:"p" help:"Peer interface"`
//	}
//
//	app := conga.New[CLI]().Use(network.RegisterInterface)
//
// See https://docs.conga.sh/reference/domain-types/ for the domain type
// reference.
package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

var (
	errEmptyHost      = errors.New("lookup host: empty host name")
	errEmptyInterface = errors.New("network interface: empty interface name")

	errInvalidHTTPMethod = errors.New("unrecognized HTTP method")
	errInvalidHTTPStatus = errors.New("invalid HTTP status")
	errInvalidMIMEType   = errors.New("invalid MIME type")
	errInvalidPort       = errors.New("invalid port")
)

// Host represents a hostname, domain name, or IP address with automatic host completion.
type Host string

// Predict returns host autocompletion candidates from /etc/hosts, SSH config, and known_hosts.
func (h *Host) Predict() predict.Predictor {
	return Hosts()
}

// Help returns the default value syntax description for host flags. The engine
// renders this text in the flag's help column when the field declares no help
// tag; a field with a help tag shows that text instead. Shells also show it as
// the description of the flag's completion candidates.
func (h *Host) Help() string {
	return "Host (hostname, domain, or IP address)"
}

// String returns the hostname or IP string.
func (h *Host) String() string {
	if h == nil {
		return ""
	}

	return string(*h)
}

// LookupIP queries DNS for the IP addresses associated with the host, honoring
// ctx for cancellation and deadlines.
func (h *Host) LookupIP(ctx context.Context) ([]net.IP, error) {
	if h == nil || *h == "" {
		return nil, errEmptyHost
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", string(*h))
	if err != nil {
		return nil, fmt.Errorf("lookup host %q: %w", string(*h), err)
	}

	return ips, nil
}

// UnmarshalText sets the host string, trimming surrounding whitespace; empty
// input clears the value. It performs no validation: a host is checked only
// when it is used (e.g. LookupIP rejects a blank host with an error).
func (h *Host) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		*h = ""

		return nil
	}

	*h = Host(raw)

	return nil
}

// HTTPMethod represents a standard HTTP request verb (GET, POST, PUT, DELETE, etc.).
type HTTPMethod string

// Predict returns HTTP method autocompletion candidates.
func (m *HTTPMethod) Predict() predict.Predictor {
	return HTTPMethods()
}

// Help returns the default value syntax description for HTTP method flags. The
// engine renders this text in the flag's help column when the field declares no
// help tag; a field with a help tag shows that text instead. Shells also show
// it as the description of the flag's completion candidates.
func (m *HTTPMethod) Help() string {
	return "HTTP method (GET, POST, PUT, DELETE, ...)"
}

// String returns the HTTP method in uppercase.
func (m *HTTPMethod) String() string {
	if m == nil {
		return ""
	}

	return string(*m)
}

// UnmarshalText parses and validates a standard HTTP method verb.
func (m *HTTPMethod) UnmarshalText(text []byte) error {
	raw := strings.ToUpper(strings.TrimSpace(string(text)))
	if raw == "" {
		*m = ""

		return nil
	}

	switch raw {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE", "CONNECT":
		*m = HTTPMethod(raw)

		return nil
	default:
		return fmt.Errorf("%w %q", errInvalidHTTPMethod, string(text))
	}
}

// HTTPStatus represents an HTTP response status code (100–599).
type HTTPStatus int

// Predict returns HTTP status code autocompletion candidates.
func (s *HTTPStatus) Predict() predict.Predictor {
	return HTTPStatusCodes()
}

// Help returns the default value syntax description for HTTP status flags. The
// engine renders this text in the flag's help column when the field declares no
// help tag; a field with a help tag shows that text instead. Shells also show
// it as the description of the flag's completion candidates.
func (s *HTTPStatus) Help() string {
	return "HTTP status code (100-599, e.g. 200, 404)"
}

// Code returns the HTTP status code as an integer.
func (s *HTTPStatus) Code() int {
	if s == nil {
		return 0
	}

	return int(*s)
}

// Text returns the standard HTTP status reason text (e.g. OK, Not Found).
func (s *HTTPStatus) Text() string {
	if s == nil {
		return ""
	}

	return httpStatusText(int(*s))
}

// IsSuccess returns true if the status code represents a 2xx success response.
func (s *HTTPStatus) IsSuccess() bool {
	if s == nil {
		return false
	}

	return *s >= 200 && *s < 300
}

// IsClientError returns true if the status code represents a 4xx client error response.
func (s *HTTPStatus) IsClientError() bool {
	if s == nil {
		return false
	}

	return *s >= 400 && *s < 500
}

// IsServerError returns true if the status code represents a 5xx server error response.
func (s *HTTPStatus) IsServerError() bool {
	if s == nil {
		return false
	}

	return *s >= 500 && *s < 600
}

// String returns the HTTP status code formatted as a string.
func (s *HTTPStatus) String() string {
	if s == nil {
		return "0"
	}

	return strconv.Itoa(int(*s))
}

// UnmarshalText parses a numeric HTTP status code (100–599) or standard status text.
func (s *HTTPStatus) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		*s = 0

		return nil
	}

	if code, err := strconv.Atoi(raw); err == nil {
		if code < 100 || code > 599 {
			return fmt.Errorf("%w: HTTP status code %d (must be between 100 and 599)", errInvalidHTTPStatus, code)
		}

		*s = HTTPStatus(code)

		return nil
	}

	code, err := parseHTTPStatusText(raw)
	if err != nil {
		return err
	}

	*s = HTTPStatus(code)

	return nil
}

// MIMEType represents an IANA media type string (e.g. application/json).
type MIMEType string

// Predict returns media type autocompletion candidates.
func (m *MIMEType) Predict() predict.Predictor {
	return MIMETypes()
}

// Help returns the default value syntax description for MIME type flags. The
// engine renders this text in the flag's help column when the field declares no
// help tag; a field with a help tag shows that text instead. Shells also show
// it as the description of the flag's completion candidates.
func (m *MIMEType) Help() string {
	return "MIME type (e.g. application/json)"
}

// Type returns the top-level media type component (e.g. application in application/json).
func (m *MIMEType) Type() string {
	if m == nil {
		return ""
	}

	topType, _, _ := strings.Cut(string(*m), "/")

	return strings.TrimSpace(topType)
}

// Subtype returns the subtype component (e.g. json in application/json; charset=utf-8).
func (m *MIMEType) Subtype() string {
	if m == nil {
		return ""
	}

	_, subtype, found := strings.Cut(string(*m), "/")
	if !found {
		return ""
	}

	subtypePart, _, _ := strings.Cut(subtype, ";")

	return strings.TrimSpace(subtypePart)
}

// String returns the media type string.
func (m *MIMEType) String() string {
	if m == nil {
		return ""
	}

	return string(*m)
}

// UnmarshalText parses and validates that the text is a valid MIME media type with a slash.
func (m *MIMEType) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		*m = ""

		return nil
	}

	topType, subtype, found := strings.Cut(raw, "/")
	if !found || strings.TrimSpace(topType) == "" || strings.TrimSpace(subtype) == "" {
		return fmt.Errorf("%w %q (expected format type/subtype, e.g. application/json)", errInvalidMIMEType, raw)
	}

	*m = MIMEType(raw)

	return nil
}

// NetInterface represents a system network interface name.
type NetInterface string

// Predict returns network interface autocompletion candidates.
func (n *NetInterface) Predict() predict.Predictor {
	return NetInterfaces()
}

// Help returns the default value syntax description for network interface
// flags. The engine renders this text in the flag's help column when the field
// declares no help tag; a field with a help tag shows that text instead. Shells
// also show it as the description of the flag's completion candidates.
func (n *NetInterface) Help() string {
	return "Network interface (e.g. eth0, lo)"
}

// String returns the network interface name.
func (n *NetInterface) String() string {
	if n == nil {
		return ""
	}

	return string(*n)
}

// Interface queries the system for the underlying net.Interface.
func (n *NetInterface) Interface() (*net.Interface, error) {
	if n == nil || *n == "" {
		return nil, errEmptyInterface
	}

	iface, err := net.InterfaceByName(string(*n))
	if err != nil {
		return nil, fmt.Errorf("network interface %q: %w", string(*n), err)
	}

	return iface, nil
}

// Addrs returns the unicast interface addresses for the network interface.
func (n *NetInterface) Addrs() ([]net.Addr, error) {
	iface, err := n.Interface()
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("interface %q addrs: %w", string(*n), err)
	}

	return addrs, nil
}

// UnmarshalText sets the network interface name.
func (n *NetInterface) UnmarshalText(text []byte) error {
	*n = NetInterface(strings.TrimSpace(string(text)))

	return nil
}

// Port represents a network service port (1–65535).
type Port uint16

// Predict returns service port autocompletion candidates.
func (p *Port) Predict() predict.Predictor {
	return Ports()
}

// Help returns the default value syntax description for port flags. The engine
// renders this text in the flag's help column when the field declares no help
// tag; a field with a help tag shows that text instead. Shells also show it as
// the description of the flag's completion candidates.
func (p *Port) Help() string {
	return `Port (1-65535, or a service name like "http")`
}

// Int returns the port number as an integer.
func (p *Port) Int() int {
	if p == nil {
		return 0
	}

	return int(*p)
}

// IsPrivileged returns true if the port is a system/privileged port (< 1024).
func (p *Port) IsPrivileged() bool {
	if p == nil {
		return false
	}

	return *p < 1024
}

// String returns the port number formatted as a string.
func (p *Port) String() string {
	if p == nil {
		return "0"
	}

	return strconv.Itoa(int(*p))
}

// UnmarshalText parses a numeric port (1–65535) or a well-known service name.
func (p *Port) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		*p = 0

		return nil
	}

	if value, err := strconv.Atoi(raw); err == nil {
		if value < 1 || value > 65535 {
			return fmt.Errorf("%w %d (must be between 1 and 65535)", errInvalidPort, value)
		}

		*p = Port(uint16(value))

		return nil
	}

	resolvedPort, err := lookupPortByName(raw)
	if err != nil {
		return err
	}

	if resolvedPort < 1 || resolvedPort > 65535 {
		return fmt.Errorf("%w: resolved %d", errInvalidPort, resolvedPort)
	}

	*p = Port(uint16(resolvedPort))

	return nil
}

// RegisterInterface mounts *net.Interface support onto the app builder: the
// parser resolves interface names (e.g. lo, eth0) with net.InterfaceByName and
// completion offers the host's interfaces. Apply with app.Use.
func RegisterInterface[T any](app *conga.App[T]) {
	app.RegisterType(net.InterfaceByName, conga.WithPredictor(NetInterfaces()))
}

// Hosts configures completion of hostnames from /etc/hosts, SSH known_hosts, and SSH config.
func Hosts() predict.Predictor {
	return predict.Callback(actionHosts)
}

// HTTPMethods configures completion of standard HTTP request methods (GET, POST, PUT, DELETE, etc.).
func HTTPMethods() predict.Predictor {
	return predict.Callback(actionHTTPMethods)
}

// HTTPStatusCodes configures completion of standard HTTP status codes with human-readable descriptions.
func HTTPStatusCodes() predict.Predictor {
	return predict.Callback(actionHTTPStatusCodes)
}

// MIMETypes configures completion of standard IANA MIME / media types.
func MIMETypes() predict.Predictor {
	return predict.Callback(actionMIMETypes)
}

// NetInterfaces configures completion of network interface names discovered on the system.
func NetInterfaces() predict.Predictor {
	return predict.Callback(actionNetInterfaces)
}

// Ports configures completion of well-known TCP/UDP service ports with descriptive annotations.
func Ports() predict.Predictor {
	return predict.Callback(actionPorts)
}
