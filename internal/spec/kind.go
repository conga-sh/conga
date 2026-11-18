package spec

import (
	"encoding"
	"io"
	"reflect"
)

// DecodeContext carries the ambient input for coercing one raw value into
// its target type: the owning field's declared custom tags and the
// engine-configured standard streams. The zero value selects no tags and
// process defaults (os.Stdin/os.Stdout), so values decoded outside an App
// run behave identically to direct UnmarshalText calls. It is a plain value
// struct: unlike context.Context it carries no cancellation or deadlines.
// internal/coerce and root conga expose it through aliases.
type DecodeContext struct {
	// Tags holds the owning field's declared custom tags (see
	// conga.TagSpecProvider); nil for fields that declare no registered tags.
	Tags FieldTags

	// Stdin is the engine-configured standard input (App.Stdin); nil selects
	// os.Stdin for types implementing "-" conventions.
	Stdin io.Reader

	// Stdout is the engine-configured standard output (App.Stdout); nil
	// selects os.Stdout.
	Stdout io.Writer
}

// ParsesItself reports whether values of targetType decode themselves from
// one text token: its pointer implements encoding.TextUnmarshaler or
// conga.ContextAwareTextUnmarshaler. Such a type is one value whatever its
// kind, so a slice-kind type like net.IP binds as a scalar rather than as a
// list of its elements.
func ParsesItself(targetType reflect.Type) bool {
	if targetType == nil {
		return false
	}

	pointerType := reflect.PointerTo(targetType)

	return pointerType.Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) ||
		pointerType.Implements(reflect.TypeFor[contextAwareTextUnmarshaler]())
}

// IsCollection reports whether targetType binds as a list or a key-value map:
// a slice or map kind that doesn't parse itself (see ParsesItself).
func IsCollection(targetType reflect.Type) bool {
	if targetType == nil {
		return false
	}

	kind := targetType.Kind()
	if kind != reflect.Slice && kind != reflect.Map {
		return false
	}

	return !ParsesItself(targetType)
}

// contextAwareTextUnmarshaler mirrors conga.ContextAwareTextUnmarshaler.
type contextAwareTextUnmarshaler interface {
	UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error
}

// isSliceCollection reports whether targetType binds as a list.
func isSliceCollection(targetType reflect.Type) bool {
	return IsCollection(targetType) && targetType.Kind() == reflect.Slice
}

// isMapCollection reports whether targetType binds as a map.
func isMapCollection(targetType reflect.Type) bool {
	return IsCollection(targetType) && targetType.Kind() == reflect.Map
}
