package timezone_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
	"github.com/conga-sh/conga/timezone"
)

//go:embed testdata/fixtures/zone1970.tab
var fixtureZone1970Tab []byte

func TestTimezones_Execution(t *testing.T) {
	t.Parallel()

	pred := timezone.Timezones()

	result := pred.Invoke(predict.Context{})

	assert.True(t, len(result.Items) > 0, "expected timezone candidates")

	values := itemValues(result.Items)
	assert.True(t, slices.Contains(values, "UTC"))
	assert.True(t, slices.Contains(values, "America/New_York") || slices.Contains(values, "Europe/London"))

	for _, item := range result.Items {
		assert.Equal(t, "timezones", item.Tag)
		assert.Equal(t, item.Value, item.Display)
	}
}

func TestTimezones_ParseZoneTab(t *testing.T) {
	tmpDir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "zone1970.tab"), fixtureZone1970Tab, 0600))
	t.Setenv("ZONEINFO", tmpDir)

	pred := timezone.Timezones()

	result := pred.Invoke(predict.Context{})

	values := itemValues(result.Items)
	assert.True(t, slices.Contains(values, "America/New_York"))
	assert.True(t, slices.Contains(values, "America/Chicago"))
	assert.True(t, slices.Contains(values, "America/Los_Angeles"))
	assert.True(t, slices.Contains(values, "Europe/London"))
	assert.True(t, slices.Contains(values, "Europe/Paris"))
	assert.True(t, slices.Contains(values, "Asia/Tokyo"))
	assert.True(t, slices.Contains(values, "Australia/Sydney"))
	assert.True(t, slices.Contains(values, "Antarctica/Vostok"))
	assert.True(t, slices.Contains(values, "UTC"))

	for _, item := range result.Items {
		if item.Value == "America/New_York" {
			assert.Equal(t, "Eastern Time", item.Description)
			assert.Equal(t, "timezones", item.Tag)
		}
	}
}

func TestTimezones_DirectoryTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	zoneSubDir := filepath.Join(tmpDir, "CustomRegion")
	assert.NoError(t, os.MkdirAll(zoneSubDir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(zoneSubDir, "CustomCity"), []byte("dummy tzdata"), 0o644))

	etcDir := filepath.Join(tmpDir, "Etc")
	assert.NoError(t, os.MkdirAll(etcDir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(etcDir, "UTC"), []byte("dummy tzdata"), 0o644))

	// Symlinked posix directory should be skipped
	assert.NoError(t, os.Symlink(zoneSubDir, filepath.Join(tmpDir, "posix")))

	t.Setenv("ZONEINFO", tmpDir)

	pred := timezone.Timezones()

	result := pred.Invoke(predict.Context{})

	values := itemValues(result.Items)
	assert.True(t, slices.Contains(values, "CustomRegion/CustomCity"))
	assert.True(t, slices.Contains(values, "Etc/UTC"), "expected Etc/UTC to be included")
	assert.False(t, slices.Contains(values, "posix"), "expected symlinked posix directory to be excluded")
	assert.True(t, slices.Contains(values, "UTC"))
}

func TestTimezones_ExcludesNonZoneFiles(t *testing.T) {
	tmpDir := t.TempDir()
	americaDir := filepath.Join(tmpDir, "America")
	assert.NoError(t, os.MkdirAll(americaDir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(americaDir, "New_York"), []byte("dummy"), 0o644))

	nonZoneFiles := []string{
		"localtime",
		"posixrules",
		"tzdata.zi",
		"leapseconds",
		"+VERSION",
		"factory",
		"iso3166.tab",
		"leap-seconds.list",
	}
	for _, f := range nonZoneFiles {
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, f), []byte("dummy"), 0o644))
	}

	t.Setenv("ZONEINFO", tmpDir)

	pred := timezone.Timezones()

	result := pred.Invoke(predict.Context{})

	values := itemValues(result.Items)
	assert.True(t, slices.Contains(values, "America/New_York"))

	for _, f := range nonZoneFiles {
		assert.False(t, slices.Contains(values, f), "expected non-zone file %q to be excluded", f)
	}
}

func itemValues(items []predict.Item) []string {
	return slicex.Map(items, func(item predict.Item) string { return item.Value })
}
