package network

import (
	"cmp"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/scanx"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/style"
	"github.com/conga-sh/conga/predict"
)

func actionHosts(ctx predict.Context) predict.Predictor {
	return predict.Items(scanHosts(ctx)...)
}

func actionHTTPMethods(_ predict.Context) predict.Predictor {
	return predict.Items(scanHTTPMethods()...)
}

func actionHTTPStatusCodes(_ predict.Context) predict.Predictor {
	return predict.Items(scanHTTPStatusCodes()...)
}

func actionMIMETypes(_ predict.Context) predict.Predictor {
	return predict.Items(scanMIMETypes()...)
}

func actionNetInterfaces(_ predict.Context) predict.Predictor {
	return predict.Items(scanNetInterfaces()...)
}

func actionPorts(_ predict.Context) predict.Predictor {
	return predict.Items(scanPorts()...)
}

func parseHostsReader(r io.Reader) []predict.Item {
	var items []predict.Item

	_ = scanx.ScanLines(r, func(line string) bool { //nolint:errcheck // best-effort candidate scan
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
	})

	return items
}

func parseSSHConfigReader(r io.Reader) []predict.Item {
	var items []predict.Item

	_ = scanx.ScanLines(r, func(line string) bool { //nolint:errcheck // best-effort candidate scan
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
	})

	return items
}

func parseSSHKnownHostsReader(r io.Reader) []predict.Item {
	var items []predict.Item

	_ = scanx.ScanLines(r, func(line string) bool { //nolint:errcheck // best-effort candidate scan
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
	})

	return items
}

func scanEtcHosts(ctx predict.Context) []predict.Item {
	hostsPath := "/etc/hosts"

	if runtime.GOOS == "windows" {
		if sysRoot := ctx.Getenv("SystemRoot"); sysRoot != "" {
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

	return parseHostsReader(hostsFile)
}

func scanHosts(ctx predict.Context) []predict.Item {
	items := append(scanEtcHosts(ctx), scanSSHHosts(ctx)...)

	return slicex.UniqueBy(items, func(item predict.Item) string { return item.Value })
}

func scanHTTPMethods() []predict.Item {
	return slicex.Map(httpMethodEntries, func(m httpMethodEntry) predict.Item {
		return predict.Item{
			Value:       m.Method,
			Display:     m.Method,
			Description: m.Description,
			Style:       m.Style,
			Tag:         "http methods",
		}
	})
}

func scanHTTPStatusCodes() []predict.Item {
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

func scanMIMETypes() []predict.Item {
	return slicex.Map(mimeEntries, func(mimeItem mimeEntry) predict.Item {
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
	return slicex.Map(portEntries, func(entry portEntry) predict.Item {
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

func scanSSHHosts(ctx predict.Context) []predict.Item {
	homeDir := cmp.Or(
		ctx.Getenv("HOME"),
		ctx.Getenv("USERPROFILE"),
	)
	if homeDir == "" {
		return nil
	}

	var items []predict.Item

	knownHostsPath := filepath.Clean(filepath.Join(homeDir, ".ssh", "known_hosts"))
	if knownHostsFile, err := os.Open(knownHostsPath); err == nil {
		items = append(items, parseSSHKnownHostsReader(knownHostsFile)...)

		_ = knownHostsFile.Close() //nolint:errcheck // read-only file handle
	}

	sshConfigPath := filepath.Clean(filepath.Join(homeDir, ".ssh", "config"))
	if sshConfigFile, err := os.Open(sshConfigPath); err == nil {
		items = append(items, parseSSHConfigReader(sshConfigFile)...)

		_ = sshConfigFile.Close() //nolint:errcheck // read-only file handle
	}

	return items
}
