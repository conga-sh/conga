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
func (c *CLI) loadPipelines(_ conga.ConfigContext, root *CLI) (*conga.ConfigDocument, error) {
	paths := pipelinePaths(root)

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

	selected := selectedPipeline(root)
	c.pipelines = mergePipelineDefinitions(files)

	if selected != "" {
		if _, ok := c.pipelines[selected]; !ok {
			return nil, fmt.Errorf("%w %q in %s", errPipelineNotFound, selected, strings.Join(paths, ", "))
		}
	}

	document := &conga.ConfigDocument{}

	for _, loaded := range files {
		applyPipelineConfig(document, loaded.path, "", loaded.file.Parallelism, loaded.file.LogLevel, loaded.file.Env)

		if selected == "" {
			continue
		}

		definition, ok := loaded.file.Pipelines[selected]
		if !ok {
			continue
		}

		applyPipelineConfig(document, loaded.path, "pipeline "+selected, definition.Parallelism, definition.LogLevel, definition.Env)
	}

	return document, nil
}

// applyPipelineConfig records one file's (or one pipeline's) configuration
// keys in the document. Scalars replace earlier writes; env entries merge key
// by key so a later layer overrides only the variables it names. The scope
// names the pipeline section when the values come from a selected pipeline.
func applyPipelineConfig(document *conga.ConfigDocument, origin, scope string, parallelism *int, logLevel *string, env map[string]string) {
	if parallelism != nil {
		document.SetScoped("parallelism", *parallelism, origin, scope)
	}

	if logLevel != nil {
		document.SetScoped("log-level", *logLevel, origin, scope)
	}

	for _, key := range slices.Sorted(maps.Keys(env)) {
		document.SetKeyScoped("env", key, env[key], origin, scope)
	}
}

// pipelinePaths returns the pipeline files to load. The bootstrap binding
// already applied the CLI > FLOWCTL_FILE > default precedence.
func pipelinePaths(root *CLI) []string {
	return root.File.Get()
}

// selectedPipeline returns the pipeline to run. The bootstrap binding already
// applied the CLI > FLOWCTL_PIPELINE precedence.
func selectedPipeline(root *CLI) string {
	return root.Pipeline.Get()
}
