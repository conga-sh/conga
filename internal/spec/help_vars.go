package spec

import (
	"fmt"
	"reflect"
	"strings"
)

// helpVars names a set of help-text variables: the ones a help tag referenced,
// or the ones a field declares. The fields are exported because the help
// renderer reads them across the package boundary; the type stays unexported so
// the variable vocabulary cannot leak into any API.
type helpVars struct {
	Range, Len, Items, Enum, Default, Env bool
}

// helpVarValues carries the substitution text for each help-text variable,
// named after the tag it mirrors. An empty field means the field declares no
// value for that variable.
type helpVarValues struct {
	rangeText, lenText, itemsText, enumText, defaultText, envText string
}

// helpVarValuesFor assembles the substitution text for a field's declared
// constraints, matching the text the automatic hints carry (bracket-less range
// text, ", "-joined enum/env lists, the raw default tag value).
func helpVarValuesFor(valueRange, lengthRange, itemsRange *Range, enumValues []string, defaultValue string, envVars []string) helpVarValues {
	return helpVarValues{
		rangeText:   valueRange.HintText(),
		lenText:     lengthRange.HintText(),
		itemsText:   itemsRange.HintText(),
		enumText:    strings.Join(enumValues, ", "),
		defaultText: defaultValue,
		envText:     strings.Join(envVars, ", "),
	}
}

// interpolateHelp expands {{name}} references in a help tag value and reports
// which variables the text referenced. Unknown names are a schema error. The
// opening braces are escaped by doubling them ({{{{ renders a literal {{),
// which needs no backslash: a backslash is not representable in a struct tag
// value without its own escaping. Every other character, including a bare '{'
// or a shell-style ${...} sequence, is copied verbatim.
func interpolateHelp(helpText string, values helpVarValues) (string, helpVars, error) {
	if !strings.Contains(helpText, "{{") {
		return helpText, helpVars{}, nil
	}

	var (
		builder    strings.Builder
		referenced helpVars
	)

	for i := 0; i < len(helpText); {
		if strings.HasPrefix(helpText[i:], "{{{{") {
			builder.WriteString("{{")

			i += 4

			continue
		}

		if strings.HasPrefix(helpText[i:], "{{") {
			name, next, ok := scanHelpVarName(helpText, i)
			if !ok {
				builder.WriteByte(helpText[i])

				i++

				continue
			}

			text, target, known := helpVarSlot(name, &values, &referenced)
			if !known {
				return "", helpVars{}, unknownHelpVarError(name)
			}

			builder.WriteString(text)

			*target = true
			i = next

			continue
		}

		builder.WriteByte(helpText[i])
		i++
	}

	return builder.String(), referenced, nil
}

// helpVarAvailability reports which help-text variables a field declares. Each
// constraint is available only when it carries a value the interpolator would
// substitute; an empty tag contributes nothing, so referencing it is an error.
// {{env}} is available when the field binds environment variables at all. An
// unresolved env prefix leaves that undecided in the field's favor, so the
// analyzer never invents a diagnostic the runtime would not report. Config-file
// flags only ever substitute {{env}}, so the other variables stay unavailable.
// The features container substitutes {{enum}} (the declared feature names) and
// {{env}} — the latter only for its combined-map surface, so a container whose
// sole environment surface is a per-feature envprefix or field env tags lacks
// it. Container-level default, range, len, and items are not part of the
// model, so their variables stay unavailable.
func helpVarAvailability(tag reflect.StructTag, role string, envPrefix string, envPrefixKnown bool) helpVars {
	envAvailable := envVarsDeclared(tag, role, envPrefix, envPrefixKnown)
	if role == "config file flag" {
		return helpVars{Env: envAvailable}
	}

	if role == "features" {
		return helpVars{Enum: true, Env: envAvailable}
	}

	return helpVars{
		Range:   tag.Get("range") != "",
		Len:     tag.Get("len") != "",
		Items:   tag.Get("items") != "",
		Enum:    len(SplitTagList(tag, "enum")) > 0,
		Default: hasTag(tag, "default"),
		Env:     envAvailable,
	}
}

// envVarsDeclared mirrors ResolveFlagEnvVars and extractConfigFileEnvVars: a
// field binds environment variables when its env tag names them, or — for
// flags and config-file flags — when the tag is absent and an env prefix
// supplies the name; env:"-" disables binding outright. Positional arguments
// never bind them. The features container substitutes {{env}} only for its
// combined-map surface, so its explicit env names are the sole source: neither
// a per-feature envprefix nor the application-level EnvPrefix reaches it.
func envVarsDeclared(tag reflect.StructTag, role string, envPrefix string, envPrefixKnown bool) bool {
	if role != "flag" && role != "config file flag" && role != "features" {
		return false
	}

	if envTag, ok := tag.Lookup("env"); ok {
		if strings.TrimSpace(envTag) == "-" {
			return false
		}

		if len(SplitTagList(tag, "env")) > 0 {
			return true
		}
	}

	if role == "features" {
		return false
	}

	if !envPrefixKnown {
		return true
	}

	return envPrefix != ""
}

// validateHelpVariables reports a {{name}} reference the field does not
// declare. Unknown variable names are reported by interpolateHelp; when
// several references are undeclared, the fixed report order of
// firstUnavailableHelpVar decides which one is named.
func validateHelpVariables(helpText string, declared helpVars) error {
	_, referenced, err := interpolateHelp(helpText, helpVarValues{})
	if err != nil {
		return err
	}

	if name := firstUnavailableHelpVar(referenced, declared); name != "" {
		return unavailableHelpVarError(name)
	}

	return nil
}

// firstUnavailableHelpVar returns the undeclared variable that wins the fixed
// report order (range, len, items, enum, default, env), or "" when every
// reference is satisfied. The order is not the textual one, so the reported
// variable is deterministic regardless of how the help text is written.
func firstUnavailableHelpVar(referenced, declared helpVars) string {
	switch {
	case referenced.Range && !declared.Range:
		return "range"
	case referenced.Len && !declared.Len:
		return "len"
	case referenced.Items && !declared.Items:
		return "items"
	case referenced.Enum && !declared.Enum:
		return "enum"
	case referenced.Default && !declared.Default:
		return "default"
	case referenced.Env && !declared.Env:
		return "env"
	default:
		return ""
	}
}

// unknownHelpVarError reports an unknown {{name}} reference. Both the runtime
// schema build and conga-lint report this message verbatim.
func unknownHelpVarError(name string) error {
	return fmt.Errorf(`%w: tag "help" references unknown variable {{%s}}; supported variables are {{range}}, {{len}}, {{items}}, {{enum}}, {{default}}, {{env}}`, ErrInvalidStructTag, name)
}

// unavailableHelpVarError reports a reference to a variable the field does not
// declare. Both the runtime schema build and conga-lint report this message
// verbatim.
func unavailableHelpVarError(name string) error {
	return fmt.Errorf(`%w: tag "help" references {{%s}}, but the field provides no value for that variable`, ErrInvalidStructTag, name)
}

// scanHelpVarName parses a {{name}} reference starting at the opening brace at
// start. It reports the trimmed name and the index just past the closing
// braces, or ok=false when no closing braces follow (an unterminated '{{'),
// which the caller copies verbatim.
func scanHelpVarName(text string, start int) (string, int, bool) {
	end := strings.Index(text[start+2:], "}}")
	if end < 0 {
		return "", start, false
	}

	return strings.TrimSpace(text[start+2 : start+2+end]), start + 2 + end + 2, true
}

// helpVarSlot resolves a variable name to its substitution text and the field
// of referenced that records the reference. It reports false for an unknown
// name.
func helpVarSlot(name string, values *helpVarValues, referenced *helpVars) (string, *bool, bool) {
	switch name {
	case "range":
		return values.rangeText, &referenced.Range, true
	case "len":
		return values.lenText, &referenced.Len, true
	case "items":
		return values.itemsText, &referenced.Items, true
	case "enum":
		return values.enumText, &referenced.Enum, true
	case "default":
		return values.defaultText, &referenced.Default, true
	case "env":
		return values.envText, &referenced.Env, true
	default:
		return "", nil, false
	}
}
