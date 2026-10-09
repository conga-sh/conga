// Package timezone integrates IANA timezones into CONGA applications.
//
// Mount [Location] using [conga.App.Use] to register *time.Location with time.LoadLocation
// and enable IANA timezone shell autocompletion.
//
// Flags parse standard IANA timezone identifiers (e.g. "America/New_York", "UTC",
// "Europe/London"). When shell completion is enabled, pressing Tab completes from
// available system timezones.
//
// See https://docs.conga.sh/reference/plugins/ for the plugin reference.
package timezone

import (
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict/timezone"
)

// Location registers *time.Location with time.LoadLocation and automatic timezone completion.
func Location[T any](app *conga.App[T]) {
	app.RegisterType(time.LoadLocation, conga.WithPredictor(timezone.Timezones()))
}
