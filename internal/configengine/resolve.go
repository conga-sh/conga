package configengine

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/filex"
	"github.com/conga-sh/conga/internal/pathx"
	"github.com/conga-sh/conga/internal/sentinel"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
)

const maxConfigFileSize = 16 * 1024 * 1024

// Resolver resolves and loads configuration files for one application
// invocation. It maps author settings, decoder registrations, and CLI or
// environment targets onto concrete files, validates their contents against
// the command schema, and reports where the effective values came from.
type Resolver struct {
	// AppName is the resolved application name used for conventional XDG probing.
	AppName string
	// EnvPrefix is the global environment variable prefix (including any trailing underscore).
	EnvPrefix string
	// Settings carries the app-level resolution mode declared via the root conga
	// ConfigOptions options.
	Settings spec.ConfigSettings
	// Registry supplies the registered configuration decoders.
	Registry *Registry
}

// EffectiveSearchPaths returns the search paths driving configuration file
// resolution: the author-declared tier when present, otherwise the built-in
// conventional tier (${XDG_CONFIG_HOME:-~/.config}/<app-name>), or nil when
// the tier is unavailable — the application name cannot be resolved, or no
// configuration decoder is registered (config-incapable apps never touch the
// filesystem).
func (r Resolver) EffectiveSearchPaths() []string {
	if len(r.Settings.Tier) > 0 {
		return r.Settings.Tier
	}

	if tier := r.defaultTier(); tier != "" {
		return []string{tier}
	}

	return nil
}

// CandidatePaths returns the expanded candidate configuration paths evaluated
// for an invocation, in reported precedence order: explicit paths when named,
// otherwise the configuration environment variable targets, otherwise the
// search tier plus the explicit option path.
func (r Resolver) CandidatePaths(explicitPaths []string, cmd *spec.Command) []string {
	if len(explicitPaths) > 0 {
		return slicex.Map(explicitPaths, pathx.Expand)
	}

	if envPaths := EnvCandidates(r.EnvPrefix, cmd); len(envPaths) > 0 {
		return envPaths
	}

	return r.defaultSearchCandidatePaths()
}

// Resolve resolves, loads, and validates the effective configuration files.
// Explicit targets (CLI flag values, the configuration environment variable,
// or the explicit option) must exist and be readable: a named path that is
// missing is a fatal error. Tier paths are probed opportunistically: the first
// existing file wins, or — in cascading mode — every existing file is
// collected in precedence order (lowest to highest) for merging. Returns the
// loaded paths, the validated key-value map, and the value source; an empty
// file set yields a source of SourceNone.
func (r Resolver) Resolve(explicitTargets []string, cmd *spec.Command) ([]string, map[string]string, container.Source, error) {
	envKeys := EnvKeys(r.EnvPrefix, cmd)
	extensions := r.Registry.Extensions()

	separator := ","
	splitEnv := false

	if cmd != nil && cmd.ConfigFileFlag != nil {
		separator = cmp.Or(cmd.ConfigFileFlag.Sep, ",")
		splitEnv = cmd.ConfigFileFlag.IsSlice
	}

	files, err := findConfigFiles(explicitTargets, envKeys, r.EffectiveSearchPaths(), r.AppName, extensions, separator, splitEnv, r.Settings.Cascading)
	if err != nil {
		return nil, nil, container.SourceNone, err
	}

	values := make(map[string]string)

	for _, file := range files {
		if err := r.loadAndValidateFile(file, cmd, values); err != nil {
			// Partial values are returned so provenance reporting can still
			// describe the files that loaded before the failure.
			return nil, values, container.SourceNone, err
		}
	}

	if len(files) == 0 {
		return nil, nil, container.SourceNone, nil
	}

	return files, values, resolveConfigSource(len(explicitTargets) > 0, envKeys), nil
}

// ProbedFiles lists the concrete configuration file locations resolution
// probes for one search-path tier: an existing directory probes the generic
// "config" stem and the application name stem (when resolvable) inside the
// directory, a missing path is probed as a stem sibling of the tier, and both
// are extended by every registered decoder extension. An existing file is
// resolved as a whole path and yields no probed locations. All locations are
// reported as full paths.
func (r Resolver) ProbedFiles(candidate string, extensions []string) []string {
	clean := filepath.Clean(pathx.Expand(candidate))

	fileInfo, err := os.Stat(clean)
	if err != nil {
		return stemProbes(clean, extensions)
	}

	if !fileInfo.IsDir() {
		return nil
	}

	return directoryProbes(clean, r.AppName, extensions)
}

// ConfigExamples suggests example configuration file locations the user can
// create for the application to see, derived from the first search-path tier
// (the highest-precedence tier): the in-directory candidates plus the stem
// sibling, deduplicated. Returns nil when no search tier participates in the
// candidate list.
func (r Resolver) ConfigExamples(candidates []string, tiers []string, extensions []string) []string {
	tier := ""

	for _, candidate := range candidates {
		if slices.Contains(tiers, candidate) {
			tier = candidate

			break
		}
	}

	if tier == "" {
		return nil
	}

	probes := append(directoryProbes(tier, r.AppName, extensions), stemProbes(tier, extensions)...)

	seen := make(map[string]bool, len(probes))

	examples := make([]string, 0, len(probes))

	for _, probe := range probes {
		if !seen[probe] {
			seen[probe] = true
			examples = append(examples, probe)
		}
	}

	return examples
}

// SplitList splits a configuration list value (an environment variable
// payload or a multi-value flag entry) by the configured separator,
// defaulting to a comma. The "none", "off", and "-" separators disable
// splitting and yield the whitespace-trimmed value as a single item; blank
// values and empty split items are dropped.
func SplitList(value string, separator string) []string {
	effectiveSeparator := cmp.Or(separator, ",")

	if effectiveSeparator == "none" || effectiveSeparator == "off" || effectiveSeparator == "-" {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return []string{trimmed}
		}

		return nil
	}

	return slicex.SplitAndTrim(value, effectiveSeparator)
}

// EnvKeys returns the environment variables that can carry configuration file
// targets: the config-file flag's declared env tags, or the global
// `<PREFIX>_CONFIG` variable.
func EnvKeys(envPrefix string, cmd *spec.Command) []string {
	if cmd != nil && cmd.ConfigFileFlag != nil && len(cmd.ConfigFileFlag.EnvVars) > 0 {
		return cmd.ConfigFileFlag.EnvVars
	}

	if envPrefix != "" {
		return []string{envPrefix + "CONFIG"}
	}

	return nil
}

// EnvCandidates returns the expanded candidate configuration paths named by
// the first set configuration environment variable, or nil when none is set.
func EnvCandidates(envPrefix string, cmd *spec.Command) []string {
	var separator string
	if cmd != nil && cmd.ConfigFileFlag != nil {
		separator = cmd.ConfigFileFlag.Sep
	}

	for _, envKey := range EnvKeys(envPrefix, cmd) {
		envValue := os.Getenv(envKey)
		if envValue == "" {
			continue
		}

		items := SplitList(envValue, separator)

		expanded := make([]string, 0, len(items))

		for _, item := range items {
			expanded = append(expanded, pathx.Expand(item))
		}

		return expanded
	}

	return nil
}

// defaultTier returns the built-in conventional configuration search tier,
// `${XDG_CONFIG_HOME:-~/.config}/<app-name>`, or "" when the tier is
// unavailable.
func (r Resolver) defaultTier() string {
	if r.AppName == "" || r.Registry == nil || len(r.Registry.Extensions()) == 0 {
		return ""
	}

	return pathx.Expand("${XDG_CONFIG_HOME:-~/.config}/" + r.AppName)
}

func (r Resolver) defaultSearchCandidatePaths() []string {
	seen := make(map[string]bool)

	var paths []string

	addPath := func(path string) {
		if path == "" {
			return
		}

		expanded := pathx.Expand(path)
		if !seen[expanded] {
			seen[expanded] = true
			paths = append(paths, expanded)
		}
	}

	for _, path := range r.EffectiveSearchPaths() {
		addPath(path)
	}

	if r.Settings.Explicit != "" {
		addPath(r.Settings.Explicit)
	}

	return paths
}

func (r Resolver) loadAndValidateFile(path string, cmd *spec.Command, dest map[string]string) error {
	cleanPath := filepath.Clean(path)

	data, err := filex.ReadLimited(cleanPath, maxConfigFileSize)
	if err != nil {
		return &configError{
			path:  path,
			msg:   fmt.Sprintf("cannot read config file %q: %v", path, err),
			cause: fmt.Errorf("%w: %w", sentinel.ErrConfigRead, err),
		}
	}

	extension := strings.ToLower(filepath.Ext(cleanPath))

	decoder := r.Registry.Decoder(extension)
	if decoder == nil {
		return &configError{
			path:  path,
			msg:   fmt.Sprintf("unsupported config file format %q for %q", extension, path),
			cause: fmt.Errorf("%w: unsupported format %q", sentinel.ErrConfigParse, extension),
		}
	}

	rawMap, err := decoder(data)
	if err != nil {
		return &configError{
			path:  path,
			msg:   fmt.Sprintf("cannot parse config file %q: %v", path, err),
			cause: fmt.Errorf("%w: %w", sentinel.ErrConfigParse, err),
		}
	}

	return validateCommandConfig("", rawMap, cmd, dest, cleanPath)
}

// findConfigFiles resolves configuration files from explicit targets and the
// search tier. Explicit targets (CLI flag values, the <PREFIX>_CONFIG
// environment variable, or an explicit option) must exist and be readable: a
// named path that is missing is a fatal error. Tier paths are probed
// opportunistically: the first existing file wins, or — in cascading mode —
// every existing file is collected in precedence order (lowest to highest)
// for merging. Returns an empty set when nothing resolves.
func findConfigFiles(explicitTargets []string, envKeys []string, tier []string, appName string, decoders []string, separator string, splitEnv bool, cascading bool) ([]string, error) {
	targets := explicitTargets
	if len(targets) == 0 {
		targets = resolveEnvConfigTargets(envKeys, separator, splitEnv)
	}

	if len(targets) > 0 {
		resolved := make([]string, 0, len(targets))

		for _, target := range targets {
			clean, err := resolveExplicitConfigFile(target)
			if err != nil {
				return nil, err
			}

			resolved = append(resolved, clean)
		}

		return resolved, nil
	}

	if cascading {
		return collectTierConfigFiles(tier, appName, decoders)
	}

	file, err := findFirstTierConfigFile(tier, appName, decoders)
	if err != nil {
		return nil, err
	}

	if file == "" {
		return nil, nil
	}

	return []string{file}, nil
}

// resolveEnvConfigTargets resolves the first matching configuration environment
// variable into explicit targets. Single-value flags treat the variable as one
// literal path; multi-valued flags split it by the configured separator. It
// returns nil when no environment variable is set.
func resolveEnvConfigTargets(envKeys []string, separator string, split bool) []string {
	for _, envKey := range envKeys {
		value := os.Getenv(envKey)
		if value == "" {
			continue
		}

		if !split {
			return []string{value}
		}

		return SplitList(value, separator)
	}

	return nil
}

// resolveExplicitConfigFile expands and verifies a single explicit config file
// path, returning its clean path or a configError if it cannot be read.
func resolveExplicitConfigFile(target string) (string, error) {
	clean := filepath.Clean(pathx.Expand(target))
	if _, err := os.Stat(clean); err != nil {
		return "", &configError{
			path:  target,
			msg:   fmt.Sprintf("cannot read config file %q: %v", target, err),
			cause: fmt.Errorf("%w: %w", sentinel.ErrConfigRead, err),
		}
	}

	return clean, nil
}

// findFirstTierConfigFile probes the search tier in precedence order and
// returns the first existing configuration file, or "" when none resolves.
func findFirstTierConfigFile(tier []string, appName string, decoders []string) (string, error) {
	for _, path := range tier {
		file, err := resolveTierConfigFile(path, appName, decoders)
		if err != nil {
			return "", err
		}

		if file != "" {
			return file, nil
		}
	}

	return "", nil
}

// collectTierConfigFiles probes every search path tier and returns all existing
// config files ordered from lowest precedence to highest precedence
// (system -> user -> local).
func collectTierConfigFiles(searchPaths []string, appName string, decoders []string) ([]string, error) {
	var tierFiles []string

	for _, path := range searchPaths {
		file, err := resolveTierConfigFile(path, appName, decoders)
		if err != nil {
			return nil, err
		}

		if file != "" {
			tierFiles = append(tierFiles, file)
		}
	}

	slices.Reverse(tierFiles)

	return tierFiles, nil
}

func resolveTierConfigFile(tierPath string, appName string, decoders []string) (string, error) {
	expanded := pathx.Expand(tierPath)
	clean := filepath.Clean(expanded)

	fileInfo, err := os.Stat(clean)
	if err == nil {
		if !fileInfo.IsDir() {
			return clean, nil
		}

		return probeDirectoryConfigFile(clean, appName, decoders)
	}

	return probeStemConfigFile(clean, decoders)
}

func probeDirectoryConfigFile(dirPath string, appName string, decoders []string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("read dir %q: %w", dirPath, err)
	}

	var matched []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		extension := strings.ToLower(filepath.Ext(name))
		if extension == "" || (len(decoders) > 0 && !slices.Contains(decoders, extension)) {
			continue
		}

		stem := strings.TrimSuffix(name, filepath.Ext(name))
		if isConfigCandidate(stem, appName) {
			fullPath := filepath.Join(dirPath, name)
			matched = append(matched, fullPath)
		}
	}

	return resolveMatchedConfigCandidates(matched, dirPath, fmt.Sprintf("in directory %q", dirPath))
}

func probeStemConfigFile(stemPath string, decoders []string) (string, error) {
	var matched []string

	for _, extension := range decoders {
		candidate := stemPath + extension
		if fileInfo, err := os.Stat(candidate); err == nil && !fileInfo.IsDir() {
			matched = append(matched, filepath.Clean(candidate))
		}
	}

	return resolveMatchedConfigCandidates(matched, stemPath, fmt.Sprintf("for stem %q", stemPath))
}

// resolveMatchedConfigCandidates reduces a set of matched config file
// candidates to a single path, or returns a conflict configError when multiple
// candidates exist.
func resolveMatchedConfigCandidates(matched []string, path, location string) (string, error) {
	if len(matched) == 0 {
		return "", nil
	}

	if len(matched) == 1 {
		return filepath.Clean(matched[0]), nil
	}

	slices.Sort(matched)

	baseNames := slicex.Map(matched, filepath.Base)

	return "", &configError{
		path:  path,
		msg:   fmt.Sprintf("multiple conflicting config files found %s: %s", location, strings.Join(baseNames, ", ")),
		cause: fmt.Errorf("%w: multiple conflicting config files found: %s", sentinel.ErrConfigConflict, strings.Join(baseNames, ", ")),
	}
}

// isConfigCandidate reports whether a file stem inside a directory search tier
// can hold the application's configuration: the generic "config" stem or the
// application name itself, matched case-insensitively. Both stems still
// require a decoder-registered file extension (applied by the caller), which
// is why extensionless rc-style dotfiles (".bashrc" convention) are not
// matched: without an extension no decoder could ever parse them.
func isConfigCandidate(stem string, appName string) bool {
	if strings.EqualFold(stem, "config") {
		return true
	}

	if appName != "" {
		return strings.EqualFold(stem, strings.TrimSpace(appName))
	}

	return false
}

// directoryProbes lists the candidate configuration file locations probed
// inside a directory search tier: the generic "config" stem and the
// application name stem (when resolvable), each extended by every registered
// decoder extension.
func directoryProbes(dirPath string, appName string, extensions []string) []string {
	stems := []string{"config"}
	if trimmedName := strings.TrimSpace(appName); trimmedName != "" {
		stems = append(stems, trimmedName)
	}

	probes := make([]string, 0, len(stems)*len(extensions))

	for _, stem := range stems {
		for _, extension := range extensions {
			probes = append(probes, filepath.Join(dirPath, stem+extension))
		}
	}

	return probes
}

// stemProbes lists the candidate configuration file locations probed for a
// missing stem path: the stem extended by every registered decoder extension.
func stemProbes(stemPath string, extensions []string) []string {
	probes := make([]string, 0, len(extensions))

	for _, extension := range extensions {
		probes = append(probes, stemPath+extension)
	}

	return probes
}

// resolveConfigSource reports the value source for loaded config files: CLI
// for explicit paths, env for environment-provided paths, config otherwise.
func resolveConfigSource(explicit bool, envKeys []string) container.Source {
	if explicit {
		return container.SourceCLI
	}

	for _, envKey := range envKeys {
		if envKey != "" && os.Getenv(envKey) != "" {
			return container.SourceEnv
		}
	}

	return container.SourceConfig
}
