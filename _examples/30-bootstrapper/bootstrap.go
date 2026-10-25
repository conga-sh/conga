package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/conga-sh/conga"
)

// Bootstrap reads the pipeline files, merges their domain data, and supplies
// the configuration document derived from the files' configuration keys and
// the selected pipeline's overrides. It runs after the bootstrap pass binds
// the bootstrap-tagged File and Pipeline fields, before the normal pass
// rebinds every field with the configuration tier available.
func (c *CLI) Bootstrap(_ context.Context, handle *conga.Bootstrap) error {
	paths := c.File.Get()

	files := make([]loadedPipelineFile, 0, len(paths))

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read pipeline file %q: %w", path, err)
		}

		file, err := parsePipelineFile(path, data)
		if err != nil {
			return err
		}

		files = append(files, loadedPipelineFile{path: path, file: file})
	}

	selected := c.Pipeline.Get()
	c.pipelines = mergePipelineDefinitions(files)

	if selected != "" {
		if _, ok := c.pipelines[selected]; !ok {
			return fmt.Errorf("%w %q in %s", errPipelineNotFound, selected, strings.Join(paths, ", "))
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

	handle.SetConfig(document)

	return nil
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
