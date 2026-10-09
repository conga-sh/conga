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
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/scanx"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/style"
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

//nolint:gochecknoglobals // static HTTP status tables for O(1) lookup
var httpStatusCodes = []httpStatusEntry{
	{Code: 100, Description: "Continue", Style: style.Default},
	{Code: 101, Description: "Switching Protocols", Style: style.Default},
	{Code: 102, Description: "Processing", Style: style.Default},
	{Code: 103, Description: "Early Hints", Style: style.Default},
	{Code: 200, Description: "OK", Style: style.Green},
	{Code: 201, Description: "Created", Style: style.Green},
	{Code: 202, Description: "Accepted", Style: style.Green},
	{Code: 203, Description: "Non-Authoritative Information", Style: style.Green},
	{Code: 204, Description: "No Content", Style: style.Green},
	{Code: 205, Description: "Reset Content", Style: style.Green},
	{Code: 206, Description: "Partial Content", Style: style.Green},
	{Code: 207, Description: "Multi-Status", Style: style.Green},
	{Code: 208, Description: "Already Reported", Style: style.Green},
	{Code: 226, Description: "IM Used", Style: style.Green},
	{Code: 300, Description: "Multiple Choices", Style: style.Yellow},
	{Code: 301, Description: "Moved Permanently", Style: style.Yellow},
	{Code: 302, Description: "Found", Style: style.Yellow},
	{Code: 303, Description: "See Other", Style: style.Yellow},
	{Code: 304, Description: "Not Modified", Style: style.Yellow},
	{Code: 305, Description: "Use Proxy", Style: style.Yellow},
	{Code: 307, Description: "Temporary Redirect", Style: style.Yellow},
	{Code: 308, Description: "Permanent Redirect", Style: style.Yellow},
	{Code: 400, Description: "Bad Request", Style: style.Red},
	{Code: 401, Description: "Unauthorized", Style: style.Red},
	{Code: 402, Description: "Payment Required", Style: style.Red},
	{Code: 403, Description: "Forbidden", Style: style.Red},
	{Code: 404, Description: "Not Found", Style: style.Red},
	{Code: 405, Description: "Method Not Allowed", Style: style.Red},
	{Code: 406, Description: "Not Acceptable", Style: style.Red},
	{Code: 407, Description: "Proxy Authentication Required", Style: style.Red},
	{Code: 408, Description: "Request Timeout", Style: style.Red},
	{Code: 409, Description: "Conflict", Style: style.Red},
	{Code: 410, Description: "Gone", Style: style.Red},
	{Code: 411, Description: "Length Required", Style: style.Red},
	{Code: 412, Description: "Precondition Failed", Style: style.Red},
	{Code: 413, Description: "Request Entity Too Large", Style: style.Red},
	{Code: 414, Description: "Request URI Too Long", Style: style.Red},
	{Code: 415, Description: "Unsupported Media Type", Style: style.Red},
	{Code: 416, Description: "Requested Range Not Satisfiable", Style: style.Red},
	{Code: 417, Description: "Expectation Failed", Style: style.Red},
	{Code: 418, Description: "I'm a teapot", Style: style.Red},
	{Code: 421, Description: "Misdirected Request", Style: style.Red},
	{Code: 422, Description: "Unprocessable Entity", Style: style.Red},
	{Code: 423, Description: "Locked", Style: style.Red},
	{Code: 424, Description: "Failed Dependency", Style: style.Red},
	{Code: 425, Description: "Too Early", Style: style.Red},
	{Code: 426, Description: "Upgrade Required", Style: style.Red},
	{Code: 428, Description: "Precondition Required", Style: style.Red},
	{Code: 429, Description: "Too Many Requests", Style: style.Red},
	{Code: 431, Description: "Request Header Fields Too Large", Style: style.Red},
	{Code: 451, Description: "Unavailable For Legal Reasons", Style: style.Red},
	{Code: 500, Description: "Internal Server Error", Style: style.Red},
	{Code: 501, Description: "Not Implemented", Style: style.Red},
	{Code: 502, Description: "Bad Gateway", Style: style.Red},
	{Code: 503, Description: "Service Unavailable", Style: style.Red},
	{Code: 504, Description: "Gateway Timeout", Style: style.Red},
	{Code: 505, Description: "HTTP Version Not Supported", Style: style.Red},
	{Code: 506, Description: "Variant Also Negotiates", Style: style.Red},
	{Code: 507, Description: "Insufficient Storage", Style: style.Red},
	{Code: 508, Description: "Loop Detected", Style: style.Red},
	{Code: 510, Description: "Not Extended", Style: style.Red},
	{Code: 511, Description: "Network Authentication Required", Style: style.Red},
}

//nolint:gochecknoglobals // static HTTP status lookup tables derived from httpStatusCodes
var (
	httpStatusTextByCode = buildHTTPStatusTextMap()
	httpStatusCodeByText = buildHTTPStatusCodeByTextMap()
)

// Host represents a hostname, domain name, or IP address with automatic host completion.
type Host string

// Predict returns host autocompletion candidates from /etc/hosts, SSH config, and known_hosts.
func (h *Host) Predict() predict.Predictor {
	return Hosts()
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
	return predict.Callback(actionHttpStatusCodes)
}

// MIMETypes configures completion of standard IANA MIME / media types.
func MIMETypes() predict.Predictor {
	return predict.Callback(actionMimeTypes)
}

// NetInterfaces configures completion of network interface names discovered on the system.
func NetInterfaces() predict.Predictor {
	return predict.Callback(actionNetInterfaces)
}

// Ports configures completion of well-known TCP/UDP service ports with descriptive annotations.
func Ports() predict.Predictor {
	return predict.Callback(actionPorts)
}

type httpMethodEntry struct {
	Method      string
	Description string
	Style       string
}

type httpStatusEntry struct {
	Code        int
	Description string
	Style       string
}

type mimeEntry struct {
	Type        string
	Description string
}

type portEntry struct {
	Port        int
	Service     string
	Protocol    string
	Description string
	Style       string
}

func actionHosts(ctx predict.Context) predict.Predictor {
	items := scanHosts(ctx)

	return predict.Items(items...)
}

func actionHTTPMethods(_ predict.Context) predict.Predictor {
	items := scanHTTPMethods()

	return predict.Items(items...)
}

func actionHttpStatusCodes(_ predict.Context) predict.Predictor {
	items := scanHttpStatusCodes()

	return predict.Items(items...)
}

func actionMimeTypes(_ predict.Context) predict.Predictor {
	items := scanMimeTypes()

	return predict.Items(items...)
}

func actionNetInterfaces(_ predict.Context) predict.Predictor {
	items := scanNetInterfaces()

	return predict.Items(items...)
}

func actionPorts(_ predict.Context) predict.Predictor {
	items := scanPorts()

	return predict.Items(items...)
}

func httpStatusText(code int) string {
	return httpStatusTextByCode[code]
}

func knownHTTPMethods() []httpMethodEntry {
	return []httpMethodEntry{
		{Method: "GET", Description: "Retrieve resource representation", Style: style.Green},
		{Method: "POST", Description: "Submit data for processing or resource creation", Style: style.Blue},
		{Method: "PUT", Description: "Replace resource state completely", Style: style.Yellow},
		{Method: "PATCH", Description: "Apply partial modifications to resource", Style: style.Magenta},
		{Method: "DELETE", Description: "Remove specified resource", Style: style.Red},
		{Method: "HEAD", Description: "Retrieve response headers without body", Style: style.Default},
		{Method: "OPTIONS", Description: "Describe communication options for target resource", Style: style.Default},
		{Method: "TRACE", Description: "Perform message loop-back test along resource path", Style: style.Default},
		{Method: "CONNECT", Description: "Establish two-way tunnel to destination server", Style: style.Default},
	}
}

func buildHTTPStatusTextMap() map[int]string {
	textByCode := make(map[int]string, len(httpStatusCodes))

	for _, entry := range httpStatusCodes {
		textByCode[entry.Code] = entry.Description
	}

	return textByCode
}

func buildHTTPStatusCodeByTextMap() map[string]int {
	codeByText := make(map[string]int, len(httpStatusCodes))

	for _, entry := range httpStatusCodes {
		codeByText[strings.ToLower(entry.Description)] = entry.Code
	}

	return codeByText
}

func knownMimeTypes() []mimeEntry {
	return []mimeEntry{
		{Type: "application/json", Description: "JavaScript Object Notation"},
		{Type: "application/ld+json", Description: "JSON-LD Linked Data"},
		{Type: "application/xml", Description: "XML Document"},
		{Type: "application/yaml", Description: "YAML Document"},
		{Type: "application/toml", Description: "TOML Document"},
		{Type: "application/pdf", Description: "Portable Document Format"},
		{Type: "application/zip", Description: "ZIP Archive"},
		{Type: "application/gzip", Description: "Gzip Archive"},
		{Type: "application/tar", Description: "Tar Archive"},
		{Type: "application/zstd", Description: "Zstandard Compressed Archive"},
		{Type: "application/octet-stream", Description: "Arbitrary Binary Data"},
		{Type: "application/x-www-form-urlencoded", Description: "URL-encoded Form Data"},
		{Type: "application/graphql+json", Description: "GraphQL Query Payload"},
		{Type: "application/sql", Description: "SQL Query"},
		{Type: "application/wasm", Description: "WebAssembly Binary"},
		{Type: "application/javascript", Description: "JavaScript Source"},
		{Type: "application/protobuf", Description: "Protocol Buffers Binary"},
		{Type: "text/plain", Description: "Plain Text"},
		{Type: "text/html", Description: "HTML Document"},
		{Type: "text/css", Description: "Cascading Style Sheets"},
		{Type: "text/javascript", Description: "JavaScript Code"},
		{Type: "text/csv", Description: "Comma-Separated Values"},
		{Type: "text/markdown", Description: "Markdown Document"},
		{Type: "text/xml", Description: "XML Text"},
		{Type: "image/png", Description: "Portable Network Graphics"},
		{Type: "image/jpeg", Description: "JPEG Image"},
		{Type: "image/gif", Description: "Graphics Interchange Format"},
		{Type: "image/svg+xml", Description: "Scalable Vector Graphics"},
		{Type: "image/webp", Description: "WebP Image"},
		{Type: "image/avif", Description: "AVIF Image"},
		{Type: "image/x-icon", Description: "Favicon / Icon"},
		{Type: "audio/mpeg", Description: "MP3 Audio"},
		{Type: "audio/ogg", Description: "Ogg Audio"},
		{Type: "audio/wav", Description: "WAV Audio"},
		{Type: "audio/webm", Description: "WebM Audio"},
		{Type: "video/mp4", Description: "MP4 Video"},
		{Type: "video/webm", Description: "WebM Video"},
		{Type: "video/ogg", Description: "Ogg Video"},
		{Type: "video/quicktime", Description: "QuickTime Video"},
		{Type: "multipart/form-data", Description: "Multipart Form Uploads"},
		{Type: "multipart/byteranges", Description: "Partial HTTP Byte Ranges"},
	}
}

func knownPorts() []portEntry {
	return []portEntry{
		{Port: 20, Service: "ftp-data", Protocol: "tcp", Description: "File Transfer Protocol (Data)", Style: style.Blue},
		{Port: 21, Service: "ftp", Protocol: "tcp", Description: "File Transfer Protocol (Control)", Style: style.Blue},
		{Port: 22, Service: "ssh", Protocol: "tcp", Description: "Secure Shell", Style: style.Blue},
		{Port: 23, Service: "telnet", Protocol: "tcp", Description: "Telnet protocol", Style: style.Blue},
		{Port: 25, Service: "smtp", Protocol: "tcp", Description: "Simple Mail Transfer Protocol", Style: style.Blue},
		{Port: 53, Service: "domain", Protocol: "udp/tcp", Description: "Domain Name System (DNS)", Style: style.Blue},
		{Port: 67, Service: "bootps", Protocol: "udp", Description: "DHCP Server", Style: style.Blue},
		{Port: 68, Service: "bootpc", Protocol: "udp", Description: "DHCP Client", Style: style.Blue},
		{Port: 69, Service: "tftp", Protocol: "udp", Description: "Trivial File Transfer Protocol", Style: style.Blue},
		{Port: 80, Service: "http", Protocol: "tcp", Description: "Hypertext Transfer Protocol", Style: style.Blue},
		{Port: 110, Service: "pop3", Protocol: "tcp", Description: "Post Office Protocol v3", Style: style.Blue},
		{Port: 123, Service: "ntp", Protocol: "udp", Description: "Network Time Protocol", Style: style.Blue},
		{Port: 143, Service: "imap", Protocol: "tcp", Description: "Internet Message Access Protocol", Style: style.Blue},
		{Port: 161, Service: "snmp", Protocol: "udp", Description: "Simple Network Management Protocol", Style: style.Blue},
		{Port: 162, Service: "snmptrap", Protocol: "udp", Description: "SNMP Trap", Style: style.Blue},
		{Port: 389, Service: "ldap", Protocol: "tcp", Description: "Lightweight Directory Access Protocol", Style: style.Blue},
		{Port: 443, Service: "https", Protocol: "tcp", Description: "HTTP over TLS/SSL", Style: style.Blue},
		{Port: 465, Service: "smtps", Protocol: "tcp", Description: "Secure SMTP", Style: style.Blue},
		{Port: 514, Service: "syslog", Protocol: "udp", Description: "System Logging", Style: style.Blue},
		{Port: 636, Service: "ldaps", Protocol: "tcp", Description: "LDAP over TLS/SSL", Style: style.Blue},
		{Port: 873, Service: "rsync", Protocol: "tcp", Description: "Rsync file sync", Style: style.Blue},
		{Port: 993, Service: "imaps", Protocol: "tcp", Description: "IMAP over TLS/SSL", Style: style.Blue},
		{Port: 995, Service: "pop3s", Protocol: "tcp", Description: "POP3 over TLS/SSL", Style: style.Blue},
		{Port: 1080, Service: "socks", Protocol: "tcp", Description: "SOCKS Proxy", Style: style.Green},
		{Port: 1433, Service: "ms-sql-s", Protocol: "tcp", Description: "Microsoft SQL Server", Style: style.Green},
		{Port: 1521, Service: "oracle", Protocol: "tcp", Description: "Oracle Database", Style: style.Green},
		{Port: 2049, Service: "nfs", Protocol: "tcp/udp", Description: "Network File System", Style: style.Green},
		{Port: 2181, Service: "zookeeper", Protocol: "tcp", Description: "Apache ZooKeeper", Style: style.Green},
		{Port: 2375, Service: "docker", Protocol: "tcp", Description: "Docker REST API (plain)", Style: style.Green},
		{Port: 2376, Service: "docker-tls", Protocol: "tcp", Description: "Docker REST API (TLS)", Style: style.Green},
		{Port: 2379, Service: "etcd-client", Protocol: "tcp", Description: "etcd Client API", Style: style.Green},
		{Port: 2380, Service: "etcd-server", Protocol: "tcp", Description: "etcd Peer API", Style: style.Green},
		{Port: 3000, Service: "grafana", Protocol: "tcp", Description: "Grafana / Dev Server", Style: style.Green},
		{Port: 3306, Service: "mysql", Protocol: "tcp", Description: "MySQL Database", Style: style.Green},
		{Port: 4222, Service: "nats", Protocol: "tcp", Description: "NATS Messaging", Style: style.Green},
		{Port: 5000, Service: "docker-registry", Protocol: "tcp", Description: "Docker Registry / Dev Server", Style: style.Green},
		{Port: 5432, Service: "postgresql", Protocol: "tcp", Description: "PostgreSQL Database", Style: style.Green},
		{Port: 5672, Service: "amqp", Protocol: "tcp", Description: "RabbitMQ / AMQP", Style: style.Green},
		{Port: 6379, Service: "redis", Protocol: "tcp", Description: "Redis Data Store", Style: style.Green},
		{Port: 8000, Service: "http-alt", Protocol: "tcp", Description: "HTTP Dev Server", Style: style.Green},
		{Port: 8080, Service: "http-proxy", Protocol: "tcp", Description: "HTTP Alternate / Proxy", Style: style.Green},
		{Port: 8443, Service: "https-alt", Protocol: "tcp", Description: "HTTPS Alternate", Style: style.Green},
		{Port: 8888, Service: "jupyter", Protocol: "tcp", Description: "Jupyter / Web Proxy", Style: style.Green},
		{Port: 9000, Service: "minio", Protocol: "tcp", Description: "MinIO / PHP-FPM", Style: style.Green},
		{Port: 9090, Service: "prometheus", Protocol: "tcp", Description: "Prometheus Monitoring", Style: style.Green},
		{Port: 9092, Service: "kafka", Protocol: "tcp", Description: "Apache Kafka", Style: style.Green},
		{Port: 9200, Service: "elasticsearch", Protocol: "tcp", Description: "Elasticsearch HTTP", Style: style.Green},
		{Port: 9300, Service: "es-transport", Protocol: "tcp", Description: "Elasticsearch Node Transport", Style: style.Green},
		{Port: 27017, Service: "mongodb", Protocol: "tcp", Description: "MongoDB Database", Style: style.Green},
		{Port: 50051, Service: "grpc", Protocol: "tcp", Description: "gRPC Server", Style: style.Green},
	}
}

func lookupPortByName(service string) (int, error) {
	for _, entry := range knownPorts() {
		if strings.EqualFold(entry.Service, service) {
			return entry.Port, nil
		}
	}

	return 0, fmt.Errorf("%w: unknown service name %q", errInvalidPort, service)
}

func parseHTTPStatusText(raw string) (int, error) {
	if code, ok := httpStatusCodeByText[strings.ToLower(raw)]; ok {
		return code, nil
	}

	return 0, fmt.Errorf("%w: unknown HTTP status text %q", errInvalidHTTPStatus, raw)
}

func parseHostsReader(r io.Reader) []predict.Item {
	var items []predict.Item

	if err := scanx.ScanLines(r, func(line string) bool {
		if commentIdx := strings.Index(line, "#"); commentIdx >= 0 {
			line = strings.TrimSpace(line[:commentIdx])
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			return true
		}

		ipAddr := fields[0]

		for _, host := range fields[1:] {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}

			items = append(items, predict.Item{
				Value:       host,
				Display:     host,
				Description: ipAddr,
				Style:       style.Blue,
				Tag:         "hosts",
			})
		}

		return true
	}); err != nil {
		return items
	}

	return items
}

func parseSSHConfigReader(r io.Reader) []predict.Item {
	var items []predict.Item

	if err := scanx.ScanLines(r, func(line string) bool {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(fields[0], "Host") {
			for _, pat := range fields[1:] {
				if strings.ContainsAny(pat, "*?!") {
					continue
				}

				items = append(items, predict.Item{
					Value:       pat,
					Display:     pat,
					Description: "SSH config host",
					Style:       style.Default,
					Tag:         "hosts",
				})
			}
		}

		return true
	}); err != nil {
		return items
	}

	return items
}

func parseSSHKnownHostsReader(r io.Reader) []predict.Item {
	var items []predict.Item

	if err := scanx.ScanLines(r, func(line string) bool {
		if strings.HasPrefix(line, "|1|") {
			return true
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			return true
		}

		hostList := fields[0]
		if strings.HasPrefix(hostList, "@") {
			if strings.EqualFold(hostList, "@revoked") {
				return true
			}

			if len(fields) < 2 {
				return true
			}

			hostList = fields[1]
		}

		for _, host := range slicex.SplitAndTrim(hostList, ",") {
			// Strip bracketed port format, e.g. [host]:2222 -> host
			if strings.HasPrefix(host, "[") {
				if endIdx := strings.Index(host, "]"); endIdx > 1 {
					host = host[1:endIdx]
				}
			}

			if host == "" {
				continue
			}

			items = append(items, predict.Item{
				Value:       host,
				Display:     host,
				Description: "SSH known host",
				Style:       style.Default,
				Tag:         "hosts",
			})
		}

		return true
	}); err != nil {
		return items
	}

	return items
}

func scanEtcHosts(seen map[string]bool) []predict.Item {
	hostsPath := "/etc/hosts"

	if runtime.GOOS == "windows" {
		if sysRoot := os.Getenv("SystemRoot"); sysRoot != "" {
			hostsPath = filepath.Join(sysRoot, "System32", "drivers", "etc", "hosts")
		} else {
			hostsPath = `C:\Windows\System32\drivers\etc\hosts`
		}
	}

	hostsFile, err := os.Open(filepath.Clean(hostsPath))
	if err != nil {
		return nil
	}
	defer hostsFile.Close() //nolint:errcheck // read-only file handle

	var items []predict.Item

	for _, item := range parseHostsReader(hostsFile) {
		if !seen[item.Value] {
			seen[item.Value] = true
			items = append(items, item)
		}
	}

	return items
}

func scanHosts(ctx predict.Context) []predict.Item {
	seen := make(map[string]bool)

	etcHosts := scanEtcHosts(seen)
	sshHosts := scanSSHHosts(ctx, seen)

	items := make([]predict.Item, 0, len(etcHosts)+len(sshHosts))
	items = append(items, etcHosts...)
	items = append(items, sshHosts...)

	return items
}

func scanHTTPMethods() []predict.Item {
	return slicex.Map(knownHTTPMethods(), func(m httpMethodEntry) predict.Item {
		return predict.Item{
			Value:       m.Method,
			Display:     m.Method,
			Description: m.Description,
			Style:       m.Style,
			Tag:         "http methods",
		}
	})
}

func scanHttpStatusCodes() []predict.Item {
	return slicex.Map(httpStatusCodes, func(statusEntry httpStatusEntry) predict.Item {
		codeStr := strconv.Itoa(statusEntry.Code)

		return predict.Item{
			Value:       codeStr,
			Display:     codeStr,
			Description: statusEntry.Description,
			Style:       statusEntry.Style,
			Tag:         "http status codes",
		}
	})
}

func scanMimeTypes() []predict.Item {
	return slicex.Map(knownMimeTypes(), func(mimeItem mimeEntry) predict.Item {
		return predict.Item{
			Value:       mimeItem.Type,
			Display:     mimeItem.Type,
			Description: mimeItem.Description,
			Style:       style.Default,
			Tag:         "media types",
		}
	})
}

func scanNetInterfaces() []predict.Item {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	items := make([]predict.Item, 0, len(ifaces))

	for _, iface := range ifaces {
		name := iface.Name
		if name == "" {
			continue
		}

		var flagParts []string

		itemStyle := style.Yellow

		if iface.Flags&net.FlagUp != 0 {
			flagParts = append(flagParts, "up")
			itemStyle = style.Green
		} else {
			flagParts = append(flagParts, "down")
		}

		if iface.Flags&net.FlagLoopback != 0 {
			flagParts = append(flagParts, "loopback")
			itemStyle = style.Blue
		}

		if iface.Flags&net.FlagBroadcast != 0 {
			flagParts = append(flagParts, "broadcast")
		}

		if iface.Flags&net.FlagMulticast != 0 {
			flagParts = append(flagParts, "multicast")
		}

		desc := strings.Join(flagParts, ", ")
		if len(iface.HardwareAddr) > 0 {
			desc += " [" + iface.HardwareAddr.String() + "]"
		}

		items = append(items, predict.Item{
			Value:       name,
			Display:     name,
			Description: desc,
			Style:       itemStyle,
			Tag:         "network interfaces",
		})
	}

	return items
}

func scanPorts() []predict.Item {
	return slicex.Map(knownPorts(), func(entry portEntry) predict.Item {
		portStr := strconv.Itoa(entry.Port)

		desc := entry.Service
		if entry.Description != "" {
			desc += " (" + entry.Description + ")"
		}

		return predict.Item{
			Value:       portStr,
			Display:     portStr,
			Description: desc,
			Style:       entry.Style,
			Tag:         "ports",
		}
	})
}

func scanSSHHosts(ctx predict.Context, seen map[string]bool) []predict.Item {
	homeDir := ""

	if ctx.Env != nil && ctx.Env["HOME"] != "" {
		homeDir = ctx.Env["HOME"]
	} else if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		homeDir = userProfile
	} else if home := os.Getenv("HOME"); home != "" {
		homeDir = home
	}

	if homeDir == "" {
		return nil
	}

	var items []predict.Item

	knownHostsPath := filepath.Clean(filepath.Join(homeDir, ".ssh", "known_hosts"))
	if knownHostsFile, err := os.Open(knownHostsPath); err == nil {
		for _, item := range parseSSHKnownHostsReader(knownHostsFile) {
			if !seen[item.Value] {
				seen[item.Value] = true
				items = append(items, item)
			}
		}

		_ = knownHostsFile.Close() //nolint:errcheck // read-only file handle
	}

	sshConfigPath := filepath.Clean(filepath.Join(homeDir, ".ssh", "config"))
	if sshConfigFile, err := os.Open(sshConfigPath); err == nil {
		for _, item := range parseSSHConfigReader(sshConfigFile) {
			if !seen[item.Value] {
				seen[item.Value] = true
				items = append(items, item)
			}
		}

		_ = sshConfigFile.Close() //nolint:errcheck // read-only file handle
	}

	return items
}
