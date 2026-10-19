package main

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"
)

// defaultPipelineFile is the pipeline file loaded when neither a flag nor an
// environment variable names one. Keep it in sync with the File flag default.
const defaultPipelineFile = "pipeline.yaml"

// pipelineVersion is the only supported pipeline file version.
const pipelineVersion = "1"

// errPipelineNotFound reports a --pipeline naming no defined pipeline.
var errPipelineNotFound = errors.New("pipeline not found")

// errUnsupportedVersion reports a pipeline file written for another version.
var errUnsupportedVersion = errors.New("unsupported pipeline file version")

// pipelineFile is one parsed pipeline file: the configuration keys that feed
// the config tier plus the pipeline definitions that stay domain data.
type pipelineFile struct {
	Version     string                 `yaml:"version"`
	Parallelism *int                   `yaml:"parallelism"`
	LogLevel    *string                `yaml:"log_level"`
	Env         map[string]string      `yaml:"env"`
	Pipelines   map[string]pipelineDef `yaml:"pipelines"`
}

// pipelineDef is one named pipeline definition. Its configuration keys
// override the file's top-level configuration for the selected pipeline.
type pipelineDef struct {
	Desc        string            `yaml:"desc"`
	Needs       []string          `yaml:"needs"`
	Steps       []string          `yaml:"steps"`
	Parallelism *int              `yaml:"parallelism"`
	LogLevel    *string           `yaml:"log_level"`
	Env         map[string]string `yaml:"env"`
}

// mergedPipeline is the domain view of one pipeline after every file merged.
type mergedPipeline struct {
	desc  string
	needs []string
	steps []string
}

// loadedPipelineFile is one parsed file paired with the path it was read from.
type loadedPipelineFile struct {
	path string
	file pipelineFile
}

// parsePipelineFile decodes one domain document. KnownFields rejects keys the
// runner does not understand, including misspelled configuration keys that a
// generic decoder would silently bind or ignore.
func parsePipelineFile(path string, data []byte) (pipelineFile, error) {
	var file pipelineFile

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(&file); err != nil {
		return pipelineFile{}, fmt.Errorf("parse pipeline file %q: %w", path, err)
	}

	if file.Version != pipelineVersion {
		return pipelineFile{}, fmt.Errorf("%q: %w %q (want %q)", path, errUnsupportedVersion, file.Version, pipelineVersion)
	}

	return file, nil
}

// mergePipelineDefinitions merges the domain data across files: a later file
// wins the description, steps concatenate in file order, and needs concatenate
// while dropping duplicates. The configuration engine merges slices
// nearest-file-wins, so this merge has to live in the loader.
func mergePipelineDefinitions(files []loadedPipelineFile) map[string]mergedPipeline {
	merged := make(map[string]mergedPipeline)

	for _, loaded := range files {
		for name, definition := range loaded.file.Pipelines {
			entry := merged[name]

			if definition.Desc != "" {
				entry.desc = definition.Desc
			}

			entry.needs = appendMissing(entry.needs, definition.Needs...)
			entry.steps = append(entry.steps, definition.Steps...)
			merged[name] = entry
		}
	}

	return merged
}

// appendMissing appends values that existing does not hold yet, preserving the
// first-seen order.
func appendMissing(existing []string, values ...string) []string {
	for _, value := range values {
		if !slices.Contains(existing, value) {
			existing = append(existing, value)
		}
	}

	return existing
}
