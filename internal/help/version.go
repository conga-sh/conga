package help

import (
	"cmp"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/conga-sh/conga/internal/jsonwire"
)

// commitHashLength is the commit prefix length rendered in version summaries.
// Twelve characters matches the hash embedded in Go's pseudo-versions
// (v0.0.0-<timestamp>-<12-char-commit>).
const commitHashLength = 12

// BuildInfoData holds build-time metadata extracted from Go runtime and debug information.
type BuildInfoData struct {
	Version    string
	Commit     string
	CommitDate string
	Dirty      bool
	GoVersion  string
	OS         string
	Arch       string
}

// GetBuildInfo extracts build metadata from runtime/debug.ReadBuildInfo().
func GetBuildInfo(userVersion string) BuildInfoData {
	buildInfo := BuildInfoData{
		Version:   userVersion,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if buildInfo.Version == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			buildInfo.Version = info.Main.Version
		}

		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				buildInfo.Commit = setting.Value
			case "vcs.time":
				buildInfo.CommitDate = setting.Value
			case "vcs.modified":
				if setting.Value == "true" {
					buildInfo.Dirty = true
				}
			case "GOOS":
				if setting.Value != "" {
					buildInfo.OS = setting.Value
				}
			case "GOARCH":
				if setting.Value != "" {
					buildInfo.Arch = setting.Value
				}
			}
		}
	}

	buildInfo.Version = cmp.Or(buildInfo.Version, "dev")

	return buildInfo
}

// VersionSummary formats the single-line version string (for --version):
// "<app> has version <version> built with <go>" plus " from <commit>" and
// " on <commit time>" when the version string does not already embed them.
func VersionSummary(appName string, userVersion string) string {
	return formatVersionSummary(appName, GetBuildInfo(userVersion))
}

// VersionDetailed formats the docker-style multi-line version block (for the
// `version` subcommand), with the full commit hash and commit timestamp.
func VersionDetailed(appName string, userVersion string) string {
	return formatVersionDetailed(appName, GetBuildInfo(userVersion))
}

// VersionJSON formats the flat machine-readable version object (for
// `version --json`). Every key is always present; unknown values are empty
// strings.
func VersionJSON(userVersion string) string {
	return formatVersionJSON(GetBuildInfo(userVersion))
}

func formatVersionSummary(appName string, buildInfo BuildInfoData) string {
	summary := fmt.Sprintf("%s has version %s built with %s", appName, buildInfo.Version, buildInfo.GoVersion)

	embedsCommit := versionEmbedsCommit(buildInfo.Version, buildInfo.Commit)

	if commit := shortCommit(buildInfo); commit != "" && !embedsCommit {
		summary += " from " + commit
	}

	if buildInfo.CommitDate != "" && !versionEmbedsTimestamp(buildInfo.Version, buildInfo.CommitDate) {
		summary += " on " + buildInfo.CommitDate
	}

	return summary
}

func formatVersionDetailed(appName string, buildInfo BuildInfoData) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "%s:\n", appName)
	fmt.Fprintf(&builder, " %-11s %s\n", "Version:", buildInfo.Version)

	if buildInfo.Commit != "" {
		commit := buildInfo.Commit
		if buildInfo.Dirty {
			commit += " (dirty)"
		}

		fmt.Fprintf(&builder, " %-11s %s\n", "Git commit:", commit)
	} else if buildInfo.Dirty {
		fmt.Fprintf(&builder, " %-11s %s\n", "Status:", "dirty")
	}

	if buildInfo.CommitDate != "" {
		fmt.Fprintf(&builder, " %-11s %s\n", "Built:", buildInfo.CommitDate)
	}

	fmt.Fprintf(&builder, " %-11s %s\n", "Go version:", buildInfo.GoVersion)
	fmt.Fprintf(&builder, " %-11s %s", "OS/Arch:", buildInfo.OS+"/"+buildInfo.Arch)

	return builder.String()
}

func formatVersionJSON(buildInfo BuildInfoData) string {
	buffer := make([]byte, 0, 256)

	buffer = append(buffer, `{"version":`...)
	buffer = jsonwire.AppendString(buffer, buildInfo.Version)
	buffer = append(buffer, `,"gitCommit":`...)
	buffer = jsonwire.AppendString(buffer, buildInfo.Commit)
	buffer = append(buffer, `,"built":`...)
	buffer = jsonwire.AppendString(buffer, buildInfo.CommitDate)
	buffer = append(buffer, `,"goVersion":`...)
	buffer = jsonwire.AppendString(buffer, buildInfo.GoVersion)
	buffer = append(buffer, `,"os":`...)
	buffer = jsonwire.AppendString(buffer, buildInfo.OS)
	buffer = append(buffer, `,"arch":`...)
	buffer = jsonwire.AppendString(buffer, buildInfo.Arch)
	buffer = append(buffer, `,"modified":`...)

	if buildInfo.Dirty {
		buffer = append(buffer, "true}"...)
	} else {
		buffer = append(buffer, "false}"...)
	}

	return string(buffer)
}

// shortCommit renders the twelve-character commit prefix with a -dirty suffix
// when the working tree was modified. Empty when no commit was embedded.
func shortCommit(buildInfo BuildInfoData) string {
	if buildInfo.Commit == "" {
		return ""
	}

	commit := buildInfo.Commit
	if len(commit) > commitHashLength {
		commit = commit[:commitHashLength]
	}

	if buildInfo.Dirty {
		commit += "-dirty"
	}

	return commit
}

// versionEmbedsCommit reports whether the version string already carries the
// commit as its trailing pseudo-version hash segment. Go's build metadata
// suffix (+dirty, +incompatible) is ignored.
func versionEmbedsCommit(version, commit string) bool {
	if len(commit) < commitHashLength {
		return false
	}

	version, _, _ = strings.Cut(version, "+")

	index := strings.LastIndexByte(version, '-')
	if index < 0 {
		return false
	}

	return version[index+1:] == commit[:commitHashLength]
}

// versionEmbedsTimestamp reports whether the version string already carries
// the commit time as the fourteen-digit UTC stamp inside a Go pseudo-version.
func versionEmbedsTimestamp(version, commitDate string) bool {
	stamp, err := time.Parse(time.RFC3339, commitDate)
	if err != nil {
		return false
	}

	return strings.Contains(version, "-"+stamp.UTC().Format("20060102150405")+"-")
}
