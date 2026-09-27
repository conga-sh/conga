package spec

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	// TagKindBool declares a strict boolean tag value: absence means false,
	// while "", "true", and "false" are the only accepted values. Typos fail
	// at schema build instead of silently inverting the tag's intent.
	TagKindBool TagKind = iota

	// TagKindString declares a raw string tag value: absence leaves the tag
	// undeclared, an empty (all-whitespace) value fails at schema build, and
	// the spec's optional Validate hook (see TagSpec) validates the raw
	// value at schema build.
	TagKindString
)

// TagKind describes how a custom field tag's value is parsed and validated.
type TagKind int

// TagSpec declares a single custom struct tag accepted by a domain type.
// Domain types register their specs via TagSpecProvider; the specs feed the
// strict tag-key vocabulary, schema-time value parsing, and combination
// exclusion checks for the fields that use the type.
type TagSpec struct {
	// Name is the struct tag key (e.g. "existing").
	Name string

	// Kind selects the value grammar parsed at schema build.
	Kind TagKind

	// Validate optionally checks the raw string value of a TagKindString
	// tag at schema build (both at runtime and in the static analyzer,
	// which shares this parse path). The returned error should be
	// self-describing; ParseFieldTags wraps it with ErrInvalidStructTag
	// unless it already wraps ErrSchema, in which case it is propagated
	// unchanged.
	Validate func(value string) error

	// Excludes lists tag keys that cannot be declared together with this
	// tag; declaring both is a schema error.
	Excludes []string
}

// TagSpecProvider is implemented by domain types that accept custom struct
// tags on the fields that use them. Schema build consults the zero value and
// a pointer to the field's element type, so specs may be exposed through
// value or pointer receivers.
type TagSpecProvider interface {
	TagSpecs() []TagSpec
}

// FieldTags holds the custom tag values declared on the field owning a
// resolved value. Only tags registered by the field's domain type appear;
// boolean values are canonicalized to "true" and "false", string values are
// stored verbatim.
type FieldTags map[string]string

// Has reports whether the field declared the tag.
func (t FieldTags) Has(key string) bool {
	_, ok := t[key]

	return ok
}

// Bool reports whether the field declared the tag with a truthy value.
func (t FieldTags) Bool(key string) bool {
	return t[key] == "true"
}

// String returns the raw value of the declared tag, or "" when absent.
func (t FieldTags) String(key string) string {
	return t[key]
}

// ParseFieldTags extracts the declared custom field tag values for tagSpecs:
// each registered key present in the struct tag is parsed per its kind, and
// excluded combinations are rejected as schema errors. Tags not registered by
// the domain type are ignored here; strict tag-key validation governs whether
// they are permitted at all.
func ParseFieldTags(tag reflect.StructTag, tagSpecs []TagSpec) (FieldTags, error) {
	if len(tagSpecs) == 0 {
		return FieldTags{}, nil
	}

	var declaredKeys []string

	values := make(FieldTags, len(tagSpecs))

	for _, tagSpec := range tagSpecs {
		switch tagSpec.Kind {
		case TagKindBool:
			value, present, err := ParseStrictBoolTag(tag, tagSpec.Name)
			if err != nil {
				return nil, err
			}

			if !present {
				continue
			}

			values[tagSpec.Name] = strconv.FormatBool(value)
			declaredKeys = append(declaredKeys, tagSpec.Name)
		case TagKindString:
			raw, present, err := ParseStrictStringTag(tag, tagSpec)
			if err != nil {
				return nil, err
			}

			if !present {
				continue
			}

			values[tagSpec.Name] = raw
			declaredKeys = append(declaredKeys, tagSpec.Name)
		default:
			return nil, fmt.Errorf("%w: tag %q declares an unknown field tag kind", ErrInvalidStructTag, tagSpec.Name)
		}
	}

	for _, key := range declaredKeys {
		for _, excluded := range tagSpecByName(tagSpecs, key).Excludes {
			if _, ok := values[excluded]; ok {
				return nil, fmt.Errorf("%w: tag %q cannot be combined with tag %q", ErrInvalidStructTag, key, excluded)
			}
		}
	}

	if len(declaredKeys) == 0 {
		return FieldTags{}, nil
	}

	return values, nil
}

// FieldTagKeys returns the tag keys of tagSpecs for tag-candidate merging.
func FieldTagKeys(tagSpecs []TagSpec) []string {
	if len(tagSpecs) == 0 {
		return nil
	}

	keys := make([]string, 0, len(tagSpecs))
	for _, tagSpec := range tagSpecs {
		keys = append(keys, tagSpec.Name)
	}

	return keys
}

// ParseStrictStringTag parses a raw string struct tag for tagSpec: absence is
// reported via the present flag, an empty (all-whitespace) value is a schema
// error, and the spec's optional Validate hook checks the raw value. A hook
// error that already wraps ErrSchema propagates unchanged; any other hook
// error is wrapped with ErrInvalidStructTag and the tag name.
func ParseStrictStringTag(tag reflect.StructTag, tagSpec TagSpec) (string, bool, error) {
	raw, ok := tag.Lookup(tagSpec.Name)
	if !ok {
		return "", false, nil
	}

	if strings.TrimSpace(raw) == "" {
		return "", false, fmt.Errorf("%w: tag %q cannot be empty", ErrInvalidStructTag, tagSpec.Name)
	}

	if tagSpec.Validate != nil {
		if err := tagSpec.Validate(raw); err != nil {
			if errors.Is(err, ErrSchema) {
				return "", false, err
			}

			return "", false, fmt.Errorf("%w: tag %q: %w", ErrInvalidStructTag, tagSpec.Name, err)
		}
	}

	return raw, true, nil
}

// DefaultTypeHelp returns the default help text contributed by a flag or
// argument's value type. Custom domain types may implement Help(FieldTags) to
// self-describe their accepted value format, tailored to the field's declared
// tags (e.g. File with existing + stdin yields "Path to an existing file or
// '-' for stdin"). Pointer, slice, and map wrappers are unwrapped so the
// repeated value's help is used, and "" is reported when the type contributes
// no help.
func DefaultTypeHelp(targetType reflect.Type, tags FieldTags) string {
	if targetType == nil {
		return ""
	}

	for targetType.Kind() == reflect.Pointer || targetType.Kind() == reflect.Slice || targetType.Kind() == reflect.Map {
		targetType = targetType.Elem()
	}

	if helper, ok := reflect.TypeAssert[helpProviderTagged](reflect.Zero(targetType)); ok {
		return helper.Help(tags)
	}

	if helper, ok := reflect.TypeAssert[helpProviderTagged](reflect.New(targetType)); ok {
		return helper.Help(tags)
	}

	if helper, ok := reflect.TypeAssert[helpProvider](reflect.Zero(targetType)); ok {
		return helper.Help()
	}

	if helper, ok := reflect.TypeAssert[helpProvider](reflect.New(targetType)); ok {
		return helper.Help()
	}

	return ""
}

// ConfigFileFieldTagSpecs returns the tag specs of the config-file flag role
// that the engine owns: multi switches the flag to multi-value arity
// (repeated --config-file, sep-split values). Flag arity is an engine
// concern, so the specs are engine-owned and shared by the config-file
// container adapters.
func ConfigFileFieldTagSpecs() []TagSpec {
	return []TagSpec{
		{Name: "multi", Kind: TagKindBool},
	}
}

// helpProvider is the local mirror of conga.HelpProvider: domain types that
// provide a default help description for their accepted value format.
// helpProviderTagged mirrors conga.TagAwareHelpProvider, the tags-tailored
// variant; the engine prefers the tagged form and falls back to the
// parameterless form.
type helpProvider interface {
	Help() string
}

type helpProviderTagged interface {
	Help(tags FieldTags) string
}

func tagSpecByName(tagSpecs []TagSpec, name string) TagSpec {
	for _, tagSpec := range tagSpecs {
		if tagSpec.Name == name {
			return tagSpec
		}
	}

	return TagSpec{}
}

// fieldTagSpecsForField resolves the field-tag specs of a flag or positional
// argument's element type: types implementing TagSpecProvider contribute their
// registered specs (value receiver first, then pointer receiver, mirroring how
// predictor inference resolves Predictable domain types). Slice and map
// containers resolve to their element type so repeated File values receive
// the same tags.
func fieldTagSpecsForField(containerType reflect.Type) []TagSpec {
	if containerType == nil {
		return nil
	}

	switch {
	case containerType.Kind() == reflect.Slice, containerType.Kind() == reflect.Map:
		containerType = containerType.Elem()
	}

	if provider, ok := reflect.TypeAssert[TagSpecProvider](reflect.Zero(containerType)); ok {
		return provider.TagSpecs()
	}

	if provider, ok := reflect.TypeAssert[TagSpecProvider](reflect.New(containerType)); ok {
		return provider.TagSpecs()
	}

	return nil
}
