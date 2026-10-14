// Package timezone provides an autocompletion predictor for IANA timezone
// identifiers and an optional standard library binding for *time.Location
// fields.
//
// [Timezones] reads system zoneinfo tables (honoring $ZONEINFO when it names a
// directory) and falls back to a built-in identifier list; UTC is always
// included.
//
// Mount [RegisterLocation] to register *time.Location with time.LoadLocation
// and enable IANA timezone completion on fields typed with the standard library
// type:
//
//	type CLI struct {
//		Zone conga.OptionalFlag[*time.Location] `help:"Server timezone"`
//	}
//
//	app := conga.New[CLI]().Use(timezone.RegisterLocation)
//
// See https://docs.conga.sh/reference/domain-types/ for the domain type
// reference.
package timezone

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/scanx"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/style"
	"github.com/conga-sh/conga/predict"
)

//nolint:gochecknoglobals // static fallback timezone names
var defaultTimezoneNames = []string{
	"Africa/Cairo", "Africa/Johannesburg", "Africa/Lagos", "Africa/Nairobi",
	"America/Anchorage", "America/Argentina/Buenos_Aires", "America/Bogota",
	"America/Chicago", "America/Denver", "America/Los_Angeles", "America/Mexico_City",
	"America/New_York", "America/Phoenix", "America/Santiago", "America/Sao_Paulo",
	"America/Toronto", "America/Vancouver",
	"Asia/Bangkok", "Asia/Dubai", "Asia/Hong_Kong", "Asia/Jakarta", "Asia/Jerusalem",
	"Asia/Kolkata", "Asia/Manila", "Asia/Seoul", "Asia/Shanghai", "Asia/Singapore",
	"Asia/Taipei", "Asia/Tokyo",
	"Atlantic/Reykjavik",
	"Australia/Adelaide", "Australia/Brisbane", "Australia/Melbourne",
	"Australia/Perth", "Australia/Sydney",
	"Europe/Amsterdam", "Europe/Athens", "Europe/Berlin", "Europe/Brussels",
	"Europe/Dublin", "Europe/Helsinki", "Europe/Lisbon", "Europe/London",
	"Europe/Madrid", "Europe/Oslo", "Europe/Paris", "Europe/Prague",
	"Europe/Rome", "Europe/Stockholm", "Europe/Vienna", "Europe/Warsaw",
	"Europe/Zurich",
	"Pacific/Auckland", "Pacific/Fiji", "Pacific/Guam", "Pacific/Honolulu",
}

// RegisterLocation mounts *time.Location support onto the app builder: the
// parser resolves IANA timezone identifiers with time.LoadLocation and
// completion offers the host's zones. Apply with app.Use.
func RegisterLocation[T any](app *conga.App[T]) {
	app.RegisterType(time.LoadLocation, conga.WithPredictor(Timezones()))
}

// Timezones configures completion of IANA timezone identifiers (e.g. UTC, America/New_York, Europe/London).
func Timezones() predict.Predictor {
	return predict.Callback(actionTimezones)
}

func actionTimezones(_ predict.Context) predict.Predictor {
	items := scanTimezones()

	return predict.Items(items...)
}

func fallbackTimezones() []predict.Item {
	return slicex.Map(defaultTimezoneNames, func(z string) predict.Item {
		return predict.Item{
			Value:   z,
			Display: z,
			Style:   style.Default,
			Tag:     "timezones",
		}
	})
}

func parseZoneTabReader(r io.Reader) []predict.Item {
	var items []predict.Item

	if err := scanx.ScanLines(r, func(line string) bool {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return true
		}

		zoneName := fields[2]

		desc := ""
		if len(fields) >= 4 {
			desc = strings.Join(fields[3:], " ")
		}

		items = append(items, predict.Item{
			Value:       zoneName,
			Display:     zoneName,
			Description: desc,
			Style:       style.Default,
			Tag:         "timezones",
		})

		return true
	}); err != nil {
		return nil
	}

	return items
}

func scanTimezones() []predict.Item {
	items := scanZoneTabFiles()
	if len(items) == 0 {
		items = scanZoneinfoDirs()
	}

	if len(items) == 0 {
		items = fallbackTimezones()
	}

	deduped := slicex.UniqueBy(items, func(item predict.Item) string { return item.Value })

	if !slices.ContainsFunc(deduped, func(item predict.Item) bool { return item.Value == "UTC" }) {
		deduped = append([]predict.Item{
			{Value: "UTC", Display: "UTC", Description: "Coordinated Universal Time", Style: style.Default, Tag: "timezones"},
		}, deduped...)
	}

	return deduped
}

func scanZoneTabFiles() []predict.Item {
	searchPaths := zoneinfoSearchPaths()

	for _, baseDir := range searchPaths {
		for _, tabName := range []string{"zone1970.tab", "zone.tab"} {
			filePath := filepath.Join(baseDir, tabName)

			file, err := os.Open(filepath.Clean(filePath))
			if err != nil {
				continue
			}

			items := parseZoneTabReader(file)
			_ = file.Close() //nolint:errcheck // read-only file handle

			if len(items) > 0 {
				return items
			}
		}
	}

	return nil
}

func scanZoneinfoDirs() []predict.Item {
	searchPaths := zoneinfoSearchPaths()

	for _, baseDir := range searchPaths {
		if items := scanZoneinfoDir(baseDir); len(items) > 0 {
			return items
		}
	}

	return nil
}

func scanZoneinfoDir(baseDir string) []predict.Item {
	if fileInfo, err := os.Stat(baseDir); err != nil || !fileInfo.IsDir() {
		return nil
	}

	var items []predict.Item

	err := filepath.WalkDir(baseDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			if shouldSkipZoneDir(entry.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		if entry.Type()&os.ModeSymlink != 0 && isSymlinkToDir(path) {
			return nil
		}

		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}

		rel = filepath.ToSlash(rel)
		if isNonZoneFile(entry.Name(), rel) {
			return nil
		}

		items = append(items, predict.Item{
			Value:   rel,
			Display: rel,
			Style:   style.Default,
			Tag:     "timezones",
		})

		return nil
	})
	if err != nil || len(items) == 0 {
		return nil
	}

	return items
}

func isSymlinkToDir(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	return fileInfo.IsDir()
}

func zoneinfoSearchPaths() []string {
	if envPath := os.Getenv("ZONEINFO"); envPath != "" {
		return []string{envPath}
	}

	return []string{
		"/usr/share/zoneinfo",
		"/usr/share/lib/zoneinfo",
		"/usr/lib/zoneinfo",
		"/etc/zoneinfo",
		"/var/db/timezone/zoneinfo",
	}
}

func isNonZoneFile(name, rel string) bool {
	switch name {
	case "localtime", "posixrules", "tzdata.zi", "leapseconds", "+VERSION", "factory", "iso3166.tab", "zone.tab", "zone1970.tab", "leap-seconds.list", "posix", "right":
		return true
	}

	return strings.HasSuffix(rel, ".tab") || strings.HasSuffix(rel, ".list") ||
		strings.HasSuffix(rel, ".zi")
}

func shouldSkipZoneDir(name string) bool {
	return name == "posix" || name == "right" || strings.HasPrefix(name, ".")
}
