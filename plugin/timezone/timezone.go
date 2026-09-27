// Package timezone provides standard library timezone plugins for the CONGA CLI
// builder. Apply [Location] with App.Use to register *time.Location through
// time.LoadLocation and add IANA timezone completion candidates.
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
