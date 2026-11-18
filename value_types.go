package conga

import (
	"encoding"
	"fmt"
	"reflect"

	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/spec"
)

// valueTypeValidator returns the schema-build check that rejects a declared
// flag or positional argument whose value type no parser decodes: a scalar
// type, a slice's element type, or a map's key or value type. parsers is the
// app's registry, collected once per schema build.
func valueTypeValidator(parsers coerce.Registry) func(reflect.Type) error {
	return func(targetType reflect.Type) error {
		if !spec.IsCollection(targetType) {
			return unsupportedValueTypeError("type", targetType, parsers)
		}

		if targetType.Kind() == reflect.Slice {
			return unsupportedValueTypeError("element type", targetType.Elem(), parsers)
		}

		if err := unsupportedValueTypeError("map key type", targetType.Key(), parsers); err != nil {
			return err
		}

		return unsupportedValueTypeError("map value type", targetType.Elem(), parsers)
	}
}

// unsupportedValueTypeError reports targetType, described by role, when no
// parser decodes it, with the registration that fixes it; nil otherwise.
func unsupportedValueTypeError(role string, targetType reflect.Type, parsers coerce.Registry) error {
	if coerce.Supports(targetType, parsers) {
		return nil
	}

	return fmt.Errorf("%w: unsupported %s %s; %s", ErrSchema, role, targetType, parserHint(targetType))
}

// parserHint names the registration that gives targetType a parser: the
// domain-package binding for the standard library types the kits register,
// otherwise App.RegisterType. A nested slice or map has no registration; it
// binds as one value once a named type implements encoding.TextUnmarshaler.
// The kit types are matched by package path and name because root conga
// can't import os/user or net.
func parserHint(targetType reflect.Type) string {
	if spec.IsCollection(targetType) {
		collection := "slice"
		if targetType.Kind() == reflect.Map {
			collection = "map"
		}

		return "give the " + collection + " a named type that implements encoding.TextUnmarshaler"
	}

	if targetType.Kind() == reflect.Pointer {
		element := targetType.Elem()

		switch element.PkgPath() + "." + element.Name() {
		case "time.Location":
			return "register a parser with app.Use(timezone.RegisterLocation)"
		case "os/user.User":
			return "register a parser with app.Use(system.RegisterUser)"
		case "os/user.Group":
			return "register a parser with app.Use(system.RegisterGroup)"
		case "net.Interface":
			return "register a parser with app.Use(network.RegisterInterface)"
		}
	}

	return "register a parser with app.RegisterType"
}

// rejectCollectionRegistration panics for a RegisterType call on a slice or
// map type: the binder decodes collections element by element and entry by
// entry, so a parser for the collection type would never run.
func rejectCollectionRegistration(targetType reflect.Type) {
	kind := targetType.Kind()
	if kind != reflect.Slice && kind != reflect.Map {
		return
	}

	if spec.ParsesItself(targetType) {
		implemented := "encoding.TextUnmarshaler"
		if !reflect.PointerTo(targetType).Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
			implemented = "conga.ContextAwareTextUnmarshaler"
		}

		panic(fmt.Sprintf("conga: RegisterType does not accept %s; it implements %s, so it binds as one value without registration", targetType, implemented))
	}

	if kind == reflect.Map {
		panic(fmt.Sprintf("conga: RegisterType does not accept map type %s; maps bind entry by entry, so register the key or value type instead", targetType))
	}

	panic(fmt.Sprintf("conga: RegisterType does not accept slice type %s; slices bind element by element, so register the element type instead", targetType))
}
