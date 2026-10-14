package network

import (
	"fmt"
	"strings"

	"github.com/conga-sh/conga/internal/style"
)

//nolint:gochecknoglobals // static HTTP status table
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

//nolint:gochecknoglobals // static HTTP methods table
var httpMethodEntries = []httpMethodEntry{
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

//nolint:gochecknoglobals // static MIME types table
var mimeEntries = []mimeEntry{
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

//nolint:gochecknoglobals // static well-known ports table
var portEntries = []portEntry{
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

func httpStatusText(code int) string {
	for _, entry := range httpStatusCodes {
		if entry.Code == code {
			return entry.Description
		}
	}

	return ""
}

func lookupPortByName(service string) (int, error) {
	for _, entry := range portEntries {
		if strings.EqualFold(entry.Service, service) {
			return entry.Port, nil
		}
	}

	return 0, fmt.Errorf("%w: unknown service name %q", errInvalidPort, service)
}

func parseHTTPStatusText(raw string) (int, error) {
	for _, entry := range httpStatusCodes {
		if strings.EqualFold(entry.Description, raw) {
			return entry.Code, nil
		}
	}

	return 0, fmt.Errorf("%w: unknown HTTP status text %q", errInvalidHTTPStatus, raw)
}
