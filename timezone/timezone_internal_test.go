package timezone

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/alecthomas/assert/v2"

	_ "embed"
)

//go:embed testdata/fixtures/zone1970_malformed.tab
var fixtureZone1970Malformed []byte

func TestFallbackTimezones(t *testing.T) {
	t.Parallel()

	items := fallbackTimezones()

	assert.True(t, len(items) > 0)
	assert.Equal(t, "timezones", items[0].Tag)

	foundNY := false

	for _, item := range items {
		if item.Value == "America/New_York" {
			foundNY = true

			break
		}
	}

	assert.True(t, foundNY)
}

func TestParseZoneTabReader_MalformedAndValid(t *testing.T) {
	t.Parallel()

	items := parseZoneTabReader(bytes.NewReader(fixtureZone1970Malformed))

	assert.Equal(t, 2, len(items))
	assert.Equal(t, "America/New_York", items[0].Value)
	assert.Equal(t, "Eastern Time", items[0].Description)
	assert.Equal(t, "Europe/London", items[1].Value)
	assert.Equal(t, "", items[1].Description)
}

func TestParseZoneTabReader_ErrorReturnsNil(t *testing.T) {
	t.Parallel()

	partialInput := "US +4042-07400 America/New_York Eastern Time\n"
	errorReader := iotest.ErrReader(errors.New("simulated stream error"))
	r := io.MultiReader(strings.NewReader(partialInput), errorReader)

	items := parseZoneTabReader(r)

	assert.Zero(t, items)
}
