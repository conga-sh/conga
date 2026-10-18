package main

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/conga-sh/conga"
)

// loadPipelines is the conga.ConfigLoader: it reads the pipeline files, merges
// their domain data, and returns the configuration document derived from the
// files' configuration keys and the selected pipeline's overrides.
func (c *CLI) loadPipelines(ctx conga.ConfigContext) (*conga.ConfigDocument, error) {
	paths := pipelinePaths(ctx)

	files := make([]loadedPipelineFile, 0, len(paths))

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read pipeline file %q: %w", path, err)
		}

		file, err := parsePipelineFile(path, data)
		if err != nil {
			return nil, err
		}

		files = append(files, loadedPipelineFile{path: path, file: file})
	}

	selected := selectedPipeline(ctx)
	c.pipelines = mergePipelineDefinitions(files)

	if selected != "" {
		if _, ok := c.pipelines[selected]; !ok {
			return nil, fmt.Errorf("%w %q in %s", errPipelineNotFound, selected, strings.Join(paths, ", "))
		}
	}

	document := &conga.ConfigDocument{}

	for _, loaded := range files {
		applyPipelineConfig(document, loaded.path, loaded.file.Parallelism, loaded.file.LogLevel, loaded.file.Env)

		if selected == "" {
			continue
		}

		definition, ok := loaded.file.Pipelines[selected]
		if !ok {
			continue
		}

		applyPipelineConfig(document, loaded.path+" › pipeline "+selected, definition.Parallelism, definition.LogLevel, definition.Env)
	}

	return document, nil
}

// applyPipelineConfig records one file's (or one pipeline's) configuration
// keys in the document. Scalars replace earlier writes; env entries merge key
// by key so a later layer overrides only the variables it names.
func applyPipelineConfig(document *conga.ConfigDocument, origin string, parallelism *int, logLevel *string, env map[string]string) {
	if parallelism != nil {
		document.Set("parallelism", *parallelism, origin)
	}

	if logLevel != nil {
		document.Set("log-level", *logLevel, origin)
	}

	for _, key := range slices.Sorted(maps.Keys(env)) {
		document.SetKey("env", key, env[key], origin)
	}
}

// pipelinePaths returns the pipeline files to load: repeated command-line
// values first, then FLOWCTL_FILE, then the default file.
func pipelinePaths(ctx conga.ConfigContext) []string {
	if paths := ctx.FlagValues("file"); len(paths) > 0 {
		return paths
	}

	if value, ok := ctx.LookupEnv("FLOWCTL_FILE"); ok && value != "" {
		return strings.Split(value, ",")
	}

	return []string{defaultPipelineFile}
}

// selectedPipeline returns the pipeline to run: the command-line flag first,
// then FLOWCTL_PIPELINE.
func selectedPipeline(ctx conga.ConfigContext) string {
	if name, ok := ctx.FlagValue("pipeline"); ok {
		return name
	}

	value, _ := ctx.LookupEnv("FLOWCTL_PIPELINE")

	return value
}
