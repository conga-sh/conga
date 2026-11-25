// Package rangenotes declares one unparseable range bound per kind, so the
// analyzer test can assert the note and hint each diagnostic carries.
package rangenotes

import (
	"context"
	"time"

	"github.com/conga-sh/conga"
)

// RangeNotesApp declares one bad bound per field.
type RangeNotesApp struct {
	Syntax   conga.OptionalFlag[int]            `name:"syntax" range:"banana..9" help:"Syntax"`           // want `invalid integer min bound "banana"`
	Float    conga.OptionalFlag[float64]        `name:"float" range:"abc..9" help:"Float"`                // want `invalid float min bound "abc"`
	Duration conga.OptionalFlag[time.Duration]  `name:"duration" range:"1x..1h" help:"Duration"`          // want `invalid duration min bound "1x"`
	Zero     conga.OptionalFlag[int]            `name:"zero" range:"07..9" help:"Zero"`                   // want `invalid integer min bound "07"`
	Huge     conga.OptionalFlag[int]            `name:"huge" range:"99999999999999999999..9" help:"Huge"` // want `invalid integer min bound "99999999999999999999"`
	Narrow   conga.OptionalFlag[float32]        `name:"narrow" range:"0..1e39" help:"Narrow"`             // want `invalid float max bound "1e39"`
	Size     conga.OptionalFlag[conga.ByteSize] `name:"size" range:"5XB..1GiB" help:"Size"`               // want `invalid byte size min bound "5XB"`
}

// Run executes the range notes fixture.
func (r *RangeNotesApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[RangeNotesApp]()
