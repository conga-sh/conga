// Package strictflag contains command definitions whose contract violations
// only surface when the -strict analyzer flag forces strict tag validation.
package strictflag

import (
	"github.com/conga-sh/conga"
)

// ForcedStrictApp uses an unknown foreign tag without app.StrictTagValidation.
type ForcedStrictApp struct { // want `does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:"verbose" externaltag:"x" help:"Verbose output"` // want `unknown struct tag "externaltag"`
}

var _ = conga.New[ForcedStrictApp]()
