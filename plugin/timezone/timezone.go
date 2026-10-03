// Package timezone integrates IANA timezones into CONGA applications.
//
// Mount [Location] using App.Use to register *time.Location with time.LoadLocation
// and enable IANA timezone shell autocompletion:
//
//	type CLI struct {
//		Zone conga.OptionalFlag[*time.Location] `short:"z" default:"UTC" help:"Server timezone"`
//	}
//
//	app := conga.New[CLI]().Use(timezone.Location)
//
// Flags parse standard IANA timezone identifiers (e.g. "America/New_York", "UTC",
// "Europe/London"). When shell completion is enabled, pressing Tab completes from
// available system timezones.
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
