package spec

import (
	"errors"
	"fmt"
)

// Schema error sentinels.
var (
	// ErrSchema indicates the command schema could not be built from the target struct.
	ErrSchema = errors.New("schema error")

	// ErrInvalidTarget indicates the schema target is not a pointer to a struct.
	ErrInvalidTarget = fmt.Errorf("%w: target must be a pointer to a struct", ErrSchema)

	// ErrDuplicateFlag indicates a duplicate flag name or short flag rune was defined.
	ErrDuplicateFlag = fmt.Errorf("%w: duplicate flag", ErrSchema)

	// ErrInvalidArgOrder indicates an invalid positional argument order.
	ErrInvalidArgOrder = fmt.Errorf("%w: invalid positional argument order", ErrSchema)

	// ErrInvalidRangeSyntax indicates an invalid syntax was provided for a range, len, or items tag.
	ErrInvalidRangeSyntax = fmt.Errorf("%w: invalid range syntax", ErrSchema)

	// ErrInvalidConfigFilePlacement indicates a ConfigFile field was declared on a subcommand struct.
	ErrInvalidConfigFilePlacement = fmt.Errorf("%w: ConfigFile can only be declared on the root command", ErrSchema)

	// ErrInvalidStructTag indicates a malformed struct tag was encountered.
	ErrInvalidStructTag = fmt.Errorf("%w: invalid struct tag", ErrSchema)

	errNonStringMapKey = fmt.Errorf("%w: map keys must be of type string", ErrSchema)
)
