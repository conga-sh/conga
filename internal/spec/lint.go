package spec

import (
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/levenshtein"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
)

//nolint:gochecknoglobals // Static candidate definitions for schema tag validation.
var (
	flagTagCandidates           = []string{"name", "short", "help", "default", "env", "aliases", "group", "hidden", "safe", "xor", "and", "enum", "enumhint", "range", "len", "items", "sep", "negatable", "placeholder", "config", "deprecated", "predictor", "feature", "bootstrap", "conga"}
	argTagCandidates            = []string{"name", "help", "default", "enum", "enumhint", "range", "len", "items", "sep", "predictor", "bootstrap", "conga"}
	builtinCommandTagCandidates = []string{"name", "summary", "aliases", "group", "hidden", "conga"}
	userCommandTagCandidates    = []string{"name", "summary", "aliases", "group", "hidden", "envprefix", "deprecated", "feature", "conga"}
	configFileTagCandidates     = []string{"name", "short", "help", "env", "aliases", "placeholder", "group", "hidden", "multi", "sep", "config", "deprecated", "feature", "conga"}
	featuresFieldTagCandidates  = []string{"flag", "env", "envprefix", "config", "sep", "help", "enumhint", "conga"}
	featureFieldTagCandidates   = []string{"name", "help", "env", "default", "enum", "stage", "conga"}
	dashRawTagCandidates        = []string{"name", "bootstrap", "conga"}
	sentinelTagCandidates       = []string{"envprefix", "conga"}
	allCongaTagCandidates       = []string{
		"name", "short", "help", "default", "env", "aliases", "group", "hidden", "safe",
		"xor", "and", "enum", "enumhint", "range", "len", "items", "sep", "negatable", "placeholder",
		"config", "deprecated", "predictor", "feature", "conga", "summary", "envprefix",
		"multi", "flag", "stage", "bootstrap",
	}
)

// IsReservedPredictorName reports whether name is a reserved built-in predictor name or alias.
func IsReservedPredictorName(name string) bool {
	switch strings.ToLower(name) {
	case "file", "files",
		"dir", "dirs", "directory", "directories",
		"exec", "executable", "executables",
		"symlink", "symlinks",
		"none", "off":
		return true
	default:
		return false
	}
}

// predictable is the local mirror of conga.Predictable: domain types that
// define their own autocompletion candidates. predictableTagged mirrors
// conga.TagAwarePredictable, the tags-tailored variant; the engine prefers
// the tagged form and falls back to the parameterless form.
type predictable interface {
	Predict() predict.Predictor
}

type predictableTagged interface {
	Predict(tags FieldTags) predict.Predictor
}

func isAllowedForeignTag(tag string) bool {
	switch tag {
	case "json", "yaml", "yml", "toml", "hcl", "xml", "mapstructure", "envconfig",
		"validate", "binding", "gorm", "db", "doc", "example", "description":
		return true
	default:
		return false
	}
}

func isAnyCongaTag(tag string) bool {
	return slices.Contains(allCongaTagCandidates, tag)
}

func validContextDescription(tag string) string {
	switch tag {
	case "short", "xor", "and", "negatable", "config", "placeholder", "safe":
		return "flags"
	case "envprefix":
		return "subcommands and sentinel fields"
	case "summary":
		return "commands"
	case "deprecated":
		return "subcommands, flags, and config file flags"
	case "predictor", "default":
		return "flags and positional arguments"
	case "bootstrap":
		return "flags, positional arguments, and trailing-argument collectors"
	default:
		return "other CONGA constructs"
	}
}

func findClosestTagSuggestion(key string, candidates []string) string {
	if match := levenshtein.Suggest(key, candidates, 4); match != "" {
		return match
	}

	return levenshtein.Suggest(key, allCongaTagCandidates, 4)
}

func parsePredictorSpec(structTag reflect.StructTag, targetType reflect.Type, isBool bool, tags FieldTags, opts *Options) (*PredictorSpec, error) {
	tagValue, hasPred := structTag.Lookup("predictor")
	if !hasPred {
		return inferPredictorSpec(targetType, tags, opts), nil
	}

	return validateExplicitPredictor(strings.TrimSpace(tagValue), isBool, slices.Sorted(maps.Keys(opts.RegisteredPredictors)), true)
}

// inferPredictorSpec resolves the predictor for a field without an explicit
// predictor:"..." tag: register-time type predictors first, then the optional
// completion-supplied inference hook. It returns nil when inference is
// disabled.
func inferPredictorSpec(targetType reflect.Type, tags FieldTags, opts *Options) *PredictorSpec {
	if spec := inferPredictorFromOptions(targetType, opts.TypePredictors); spec != nil {
		return spec
	}

	if opts.InferPredictor != nil {
		return opts.InferPredictor(targetType, tags)
	}

	return nil
}

// validateExplicitPredictor validates the trimmed value of an explicit
// predictor:"..." struct tag: built-in predictor vocabulary first, then
// registered custom predictor names. A false checkCustomNames skips the
// registration lookup while keeping every other check.
func validateExplicitPredictor(trimmedValue string, isBool bool, registeredNames []string, checkCustomNames bool) (*PredictorSpec, error) {
	if trimmedValue == "" {
		return nil, fmt.Errorf("%w: predictor tag cannot be empty", ErrInvalidStructTag)
	}

	if isBool {
		return nil, fmt.Errorf("%w: tag \"predictor\" is invalid on boolean flags", ErrInvalidStructTag)
	}

	spec, handled, err := parseBuiltinPredictor(trimmedValue)
	if err != nil {
		return nil, err
	}

	if handled {
		return spec, nil
	}

	if !checkCustomNames {
		return &PredictorSpec{Name: trimmedValue, Kind: PredictorKindCustom}, nil
	}

	return validateCustomPredictor(trimmedValue, registeredNames)
}

func parseBuiltinPredictor(value string) (*PredictorSpec, bool, error) {
	if spec, ok, err := parseBuiltinFilesystemPredictor(value); ok || err != nil {
		return spec, ok, err
	}

	if spec, ok, err := parseBuiltinEnginePredictor(value); ok || err != nil {
		return spec, ok, err
	}

	if prefix, _, found := strings.Cut(value, ":"); found {
		return nil, false, validateColonPrefixTypo(value, prefix)
	}

	if err := validateBuiltinNameTypo(value); err != nil {
		return nil, false, err
	}

	return nil, false, nil
}

func parseBuiltinFilesystemPredictor(value string) (*PredictorSpec, bool, error) {
	switch {
	case value == "file":
		return &PredictorSpec{Kind: PredictorKindFile, Name: "file"}, true, nil
	case strings.HasPrefix(value, "file:"):
		spec, err := parseFileFilterPredictor(value)

		return spec, true, err
	case value == "dir":
		return &PredictorSpec{Kind: PredictorKindDir, Name: "dir"}, true, nil
	case strings.HasPrefix(value, "dir:"):
		return nil, false, fmt.Errorf("%w: invalid predictor %q (directories predictor does not accept filter patterns, use predictor:\"dir\")", ErrInvalidStructTag, value)
	case value == "symlink" || value == "symlinks":
		return &PredictorSpec{Kind: PredictorKindSymlink, Name: "symlink"}, true, nil
	case strings.HasPrefix(value, "symlink:") || strings.HasPrefix(value, "symlinks:"):
		return nil, false, fmt.Errorf("%w: invalid predictor %q (symlink predictor does not accept parameters, use predictor:\"symlink\")", ErrInvalidStructTag, value)
	}

	return nil, false, nil
}

func parseBuiltinEnginePredictor(value string) (*PredictorSpec, bool, error) {
	switch value {
	case "exec", "executable":
		return &PredictorSpec{Kind: PredictorKindExec, Name: "exec"}, true, nil
	case "none", "off":
		return &PredictorSpec{Kind: PredictorKindNone, Name: "none"}, true, nil
	}

	if strings.HasPrefix(value, "exec:") || strings.HasPrefix(value, "executable:") {
		return nil, false, fmt.Errorf("%w: invalid predictor %q (exec predictor does not accept parameters, use predictor:\"exec\")", ErrInvalidStructTag, value)
	}

	if strings.HasPrefix(value, "none:") || strings.HasPrefix(value, "off:") {
		return nil, false, fmt.Errorf("%w: invalid predictor %q (none predictor does not accept parameters, use predictor:\"none\")", ErrInvalidStructTag, value)
	}

	return nil, false, nil
}

func parseFileFilterPredictor(value string) (*PredictorSpec, error) {
	patternStr := value[len("file:"):]
	if patternStr == "" {
		return nil, fmt.Errorf("%w: invalid predictor syntax %q: file filter pattern cannot be empty (expected file:*.ext)", ErrInvalidStructTag, value)
	}

	if strings.HasPrefix(patternStr, ",") || strings.HasSuffix(patternStr, ",") || strings.Contains(patternStr, ",,") {
		return nil, fmt.Errorf("%w: invalid predictor syntax %q: trailing, leading, or empty pattern element in file filter", ErrInvalidStructTag, value)
	}

	patterns := slicex.SplitAndTrim(patternStr, ",")
	if slices.Contains(patterns, "") {
		return nil, fmt.Errorf("%w: invalid predictor syntax %q: empty pattern element in file filter", ErrInvalidStructTag, value)
	}

	return &PredictorSpec{Kind: PredictorKindFile, Name: "file", Patterns: patterns}, nil
}

func validateColonPrefixTypo(value, prefix string) error {
	dist := levenshtein.Distance(strings.ToLower(prefix), "file")

	if dist <= 2 || prefix == "files" {
		return fmt.Errorf("%w: invalid predictor %q (unknown predictor prefix %q, did you mean \"file\"?)", ErrInvalidStructTag, value, prefix)
	}

	return fmt.Errorf("%w: invalid predictor %q (unknown predictor prefix %q)", ErrInvalidStructTag, value, prefix)
}

func validateBuiltinNameTypo(value string) error {
	switch value {
	case "files":
		return fmt.Errorf("%w: unknown predictor %q (did you mean \"file\"?)", ErrInvalidStructTag, value)
	case "dirs", "directory", "directories":
		return fmt.Errorf("%w: unknown predictor %q (did you mean \"dir\"?)", ErrInvalidStructTag, value)
	case "executables":
		return fmt.Errorf("%w: unknown predictor %q (did you mean \"exec\"?)", ErrInvalidStructTag, value)
	}

	return nil
}

func validateCustomPredictor(value string, registeredNames []string) (*PredictorSpec, error) {
	if strings.ContainsAny(value, " \t\r\n,") {
		return nil, fmt.Errorf("%w: invalid predictor %q (predictor name cannot contain spaces or commas)", ErrInvalidStructTag, value)
	}

	if slices.Contains(registeredNames, value) {
		return &PredictorSpec{Kind: PredictorKindCustom, Name: value}, nil
	}

	candidates := slices.Concat([]string{"file", "dir", "exec", "symlink", "none", "off"}, registeredNames)

	suggestion := levenshtein.Suggest(value, candidates, 3)
	if suggestion != "" {
		return nil, fmt.Errorf("%w: unregistered predictor %q (did you mean %q?). Register it with app.Predictor(%q, action)", ErrInvalidStructTag, value, suggestion, value)
	}

	return nil, fmt.Errorf("%w: unregistered predictor %q. Register it with app.Predictor(%q, action)", ErrInvalidStructTag, value, value)
}

func inferPredictorFromOptions(targetType reflect.Type, typePredictors map[reflect.Type]predict.Predictor) *PredictorSpec {
	if len(typePredictors) == 0 || targetType == nil {
		return nil
	}

	elemType := targetType
	if elemType.Kind() == reflect.Slice {
		elemType = elemType.Elem()
	}

	if pred, ok := typePredictors[elemType]; ok {
		return &PredictorSpec{
			Kind:      PredictorKindPredictable,
			Name:      elemType.String(),
			Predictor: pred,
		}
	}

	if elemType.Kind() == reflect.Pointer {
		if pred, ok := typePredictors[elemType.Elem()]; ok {
			return &PredictorSpec{
				Kind:      PredictorKindPredictable,
				Name:      elemType.Elem().String(),
				Predictor: pred,
			}
		}
	} else {
		ptrType := reflect.PointerTo(elemType)
		if pred, ok := typePredictors[ptrType]; ok {
			return &PredictorSpec{
				Kind:      PredictorKindPredictable,
				Name:      elemType.String(),
				Predictor: pred,
			}
		}
	}

	return nil
}

// inferImplements asserts the filterable/predictable interface T on elemType —
// first in value form, then in pointer form — and builds the predictor through
// build. It is the shared skeleton of the predictor-inference helpers.
func inferImplements[T any](elemType reflect.Type, build func(T) predict.Predictor) *PredictorSpec {
	if target, ok := assertValueOrPointer[T](elemType); ok {
		return &PredictorSpec{
			Kind:      PredictorKindPredictable,
			Name:      "predictable",
			Predictor: build(target),
		}
	}

	return nil
}

func inferFilterablePredictor(elemType reflect.Type) *PredictorSpec {
	if spec := inferImplements[predict.FileFilterable](elemType, func(f predict.FileFilterable) predict.Predictor {
		return predict.FilesFiltered(f.MatchFile)
	}); spec != nil {
		return spec
	}

	if spec := inferImplements[predict.DirFilterable](elemType, func(f predict.DirFilterable) predict.Predictor {
		// Directory-only completion is PathsFiltered with a directory predicate;
		// NoSpace keeps the trailing-space suppression directory candidates need.
		return predict.PathsFiltered(func(path string, entry os.DirEntry) bool {
			return isDirectoryEntry(path, entry) && f.MatchDir(path, entry)
		}).NoSpace()
	}); spec != nil {
		return spec
	}

	return inferImplements[predict.PathFilterable](elemType, func(f predict.PathFilterable) predict.Predictor {
		return predict.PathsFiltered(f.MatchPath)
	})
}

func inferPredictable(elemType reflect.Type, tags FieldTags) *PredictorSpec {
	if spec := inferImplements[predictableTagged](elemType, func(p predictableTagged) predict.Predictor {
		return p.Predict(tags)
	}); spec != nil {
		return spec
	}

	return inferImplements[predictable](elemType, func(p predictable) predict.Predictor {
		return p.Predict()
	})
}

// isDirectoryEntry mirrors predict's directory detection: real directories and
// symlinks that resolve to directories are accepted, so DirFilterable keeps
// offering symlinked directories now that inference composes the predicate over
// PathsFiltered.
func isDirectoryEntry(path string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}

	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}

	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
