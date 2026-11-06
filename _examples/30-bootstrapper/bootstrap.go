package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
		if errors.Is(err, fs.ErrNotExist) && !c.File.Changed() {
			// The default pipeline.yaml is optional; a file named with -f
			// or FLOW_FILE must exist.
			continue
		}

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
			return fmt.Errorf("%s has %w %q", strings.Join(paths, ", "), errNoPipeline, selected)
		}
	}

	document := &conga.ConfigDocument{}

	for _, loaded := range files {
		applySettings(document, loaded.path, "", loaded.file.pipelineSettings)

		if selected == "" {
			continue
		}

		definition, ok := loaded.file.Pipelines[selected]
		if !ok {
			continue
		}

		applySettings(document, loaded.path, "pipeline "+selected, definition.pipelineSettings)
	}

	handle.SetConfig(document)

	return nil
}

// applySettings records one file's (or one pipeline's) settings in the
// document. Scalars replace earlier writes; env entries merge key by key so a
// later layer overrides only the variables it names. The scope names the
// pipeline section when the settings come from a selected pipeline.
func applySettings(document *conga.ConfigDocument, origin, scope string, settings pipelineSettings) {
	if settings.Parallelism != nil {
		document.SetScoped("parallelism", *settings.Parallelism, origin, scope)
	}

	if settings.LogLevel != nil {
		document.SetScoped("log-level", *settings.LogLevel, origin, scope)
	}

	for _, key := range slices.Sorted(maps.Keys(settings.Env)) {
		document.SetKeyScoped("env", key, settings.Env[key], origin, scope)
	}
}
