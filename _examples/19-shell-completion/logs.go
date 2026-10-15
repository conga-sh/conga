package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/predict"
)

// severityPredictor is registered as the "severities" predictor below and is
// also evaluated programmatically by --list-candidates, demonstrating Invoke,
// Raw, Filter, and Tag outside the completion engine. Tag is observable on the
// programmatic result; flag completion overwrites it with the engine's own
// "values" tag.
var severityPredictor = predict.Values("debug", "info", "warn", "error").
	KeepOrder().
	Tag("severities").
	Usage("log severity threshold (source order preserved)")

// LogsCmd is the predictor SDK playground: every flag demonstrates one
// predict.Predictor constructor or combinator, registered in appendLogPredictors.
type LogsCmd struct {
	Env            conga.OptionalFlag[string]            `default:"staging"                                                          enum:"staging,production,qa" help:"Target environment" short:"e"`
	Follow         conga.OptionalFlag[bool]              `help:"Follow log stream"                                                   short:"f"`
	ListCandidates conga.OptionalFlag[bool]              `help:"Print the valid severity values (programmatic predictor evaluation)"`
	Severity       conga.OptionalFlag[string]            `help:"Minimum severity (KeepOrder preserves source order)"                 predictor:"severities"`
	Sink           conga.OptionalFlag[string]            `help:"Export sink (rich Items: display, style, tag)"                       predictor:"sinks"`
	SinkMode       conga.OptionalFlag[string]            `help:"Sink buffering mode (Map with descriptions)"                         predictor:"sink-modes"`
	Labels         conga.OptionalFlag[map[string]string] `help:"Log labels (KeyValue)"                                               predictor:"labels"           short:"l"`
	Mute           conga.OptionalFlag[[]string]          `help:"Service to suppress (FlagValues dedup)"                              predictor:"mute-services"`
	OutFile        conga.OptionalFlag[string]            `help:"Output log file (Files with extension filter)"                       predictor:"log-files"`
	GlobLog        conga.OptionalFlag[string]            `help:"Log file matching a glob pattern (FilesGlob)"                        predictor:"glob-logs"`
	LogDir         conga.OptionalFlag[string]            `help:"Log output directory (Dirs)"                                         predictor:"log-dirs"`
	Archive        conga.OptionalFlag[string]            `help:"Compressed archive to read (FilesFiltered)"                          predictor:"archives"`
	NodeDir        conga.OptionalFlag[string]            `help:"Node log directory (DirsFiltered)"                                   predictor:"node-dirs"`
	VersionDir     conga.OptionalFlag[string]            `help:"Versioned output directory (DirsGlob)"                               predictor:"version-dirs"`
	LogPath        conga.OptionalFlag[string]            `help:"Any manifest or directory (PathsFiltered)"                           predictor:"log-paths"`
	Preset         conga.OptionalFlag[string]            `help:"Saved query preset (FileLines)"                                      predictor:"presets"`
	LabelKey       conga.OptionalFlag[string]            `help:"Label key to filter on (FileContents + custom parser)"               predictor:"label-keys"`
	Bucket         conga.OptionalFlag[string]            `help:"Archive bucket (Prefix)"                                             predictor:"buckets"`
	Collector      conga.OptionalFlag[string]            `help:"Syslog collector host (Suffix)"                                      predictor:"collectors"`
	Endpoint       conga.OptionalFlag[string]            `help:"Ingestion endpoint (Batch)"                                          predictor:"endpoints"`
	Shard          conga.OptionalFlag[string]            `help:"Log shard (Timeout with fallback)"                                   predictor:"shards"`
	Pager          conga.OptionalFlag[string]            `help:"Pager program (Getenv from $PAGER)"                                  predictor:"pagers"`
	Editor         conga.OptionalFlag[string]            `help:"Editor binary (Executables from $PATH)"                              predictor:"editors"`
	Hook           conga.OptionalFlag[string]            `help:"Hook script (Symlinks only)"                                         predictor:"log-hooks"`
	Silent         conga.OptionalFlag[string]            `help:"No completion at all (None)"                                         predictor:"silent"`

	Service conga.RequiredArg[string] `enum:"auth,billing,api,frontend" help:"Target service name"`
}

func (l *LogsCmd) Run(ctx context.Context, session *conga.Session) error {
	if l.ListCandidates.Get() {
		stdout := session.Stdout()

		severityResult := severityPredictor.Invoke(predict.Context{})
		filteredResult := severityPredictor.FilterText("w").Invoke(predict.Context{})

		filteredValues := make([]string, 0, len(filteredResult.Items))
		for _, item := range filteredResult.Items {
			filteredValues = append(filteredValues, item.Value)
		}

		fmt.Fprintf(stdout, "Severities: %v\n", severityPredictor.Raw())
		fmt.Fprintf(stdout, "Severities matching \"w\": %v\n", filteredValues)
		fmt.Fprintf(stdout, "KeepOrder directive: %t\n", severityResult.Directive.Has(predict.DirectiveKeepOrder))

		if len(severityResult.Items) > 0 {
			fmt.Fprintf(stdout, "Tag: %s\n", severityResult.Items[0].Tag)
		}

		return nil
	}

	inspector.Print(session, session.Root[CLI](), l)

	return nil
}

// appendLogPredictors registers the predictor SDK playground used by the logs
// subcommand: every constructor and combinator of the predict package appears
// exactly once.
func appendLogPredictors(app *conga.App[CLI]) *conga.App[CLI] {
	labelsRegistry := map[string][]string{
		"team":  {"core", "frontend", "infra", "security"},
		"tier":  {"frontend", "backend", "database", "cache"},
		"owner": {"alice", "bob", "carol", "david"},
	}

	services := []string{"api", "auth", "billing", "frontend"}

	return app.
		Predictor("severities", severityPredictor).
		Predictor("sinks", predict.Items(
			predict.NewItem("stdout", "Write to standard output").
				WithDisplay("stdout (console)").
				WithStyle("dim").
				WithTag("sinks"),
			predict.NewItem("file", "Write to a log file").WithTag("sinks"),
			predict.NewItem("syslog", "Forward to a collector").
				WithDescription("Forward to a syslog collector (UDP 514)").
				WithTag("sinks"),
		)).
		Predictor("sink-modes", predict.Map(map[string]string{
			"buffered":   "Coalesce writes into 64KiB buffers",
			"direct":     "Write through on every event",
			"unbuffered": "Flush after every line",
		}).Usage("sink buffering mode")).
		Predictor("labels", predict.KeyValue(labelsRegistry)).
		Predictor("mute-services", predict.Callback(func(compCtx predict.Context) predict.Predictor {
			muted := compCtx.FlagValues("mute")

			remaining := slices.DeleteFunc(slices.Clone(services), func(service string) bool {
				return slices.Contains(muted, service)
			})

			return predict.Values(remaining...).Usage("services (already muted excluded)")
		})).
		Predictor("log-files", predict.Files(".log").Usage("rotated log files")).
		Predictor("glob-logs", predict.FilesGlob("*.log", "*.txt").
			Usage("log files and notes by glob pattern")).
		Predictor("log-dirs", predict.Dirs().Usage("directories holding log output")).
		Predictor("archives", predict.FilesFiltered(func(path string, entry os.DirEntry) bool {
			return strings.HasSuffix(entry.Name(), ".gz")
		}).Usage("compressed log archives")).
		Predictor("node-dirs", predict.DirsFiltered(func(path string, entry os.DirEntry) bool {
			return strings.HasPrefix(entry.Name(), "202")
		}).Usage("dated node directories")).
		Predictor("version-dirs", predict.DirsGlob("v*").Usage("versioned output directories")).
		Predictor("log-paths", predict.PathsFiltered(func(path string, entry os.DirEntry) bool {
			return entry.IsDir() || strings.HasSuffix(entry.Name(), ".yaml")
		}).Usage("manifest files or directories")).
		Predictor("presets", predict.FileLines("presets.conf").Usage("saved query presets")).
		Predictor("label-keys", predict.FileContents("labels.conf", parseLabelKeys)).
		Predictor("buckets", predict.Values("logs-primary", "logs-replica").
			Prefix("s3://").
			Style("cyan").
			Usage("archive buckets")).
		Predictor("collectors", predict.Values("collector-a", "collector-b").
			Suffix(":5514").
			Usage("syslog collectors")).
		Predictor("endpoints", predict.Batch(
			predict.Values("http://logs.local", "https://logs.internal"),
			predict.Callback(func(compCtx predict.Context) predict.Predictor {
				return predict.Values("unix:///var/run/log.sock").Usage("local socket")
			}),
		).Usage("log ingestion endpoints")).
		Predictor("shards", predict.Callback(func(compCtx predict.Context) predict.Predictor {
			return slowShardRegistry()
		}).Timeout(50*time.Millisecond, predict.Values("shard-0", "shard-1", "shard-2").
			Usage("log shards (fallback after timeout)"))).
		Predictor("pagers", predict.Callback(func(compCtx predict.Context) predict.Predictor {
			if pager := compCtx.Getenv("PAGER"); pager != "" {
				return predict.Values(pager).Usage("pager from $PAGER")
			}

			return predict.Values("less", "more").Usage("default pagers")
		})).
		Predictor("editors", predict.Executables().Usage("editor binary from $PATH")).
		Predictor("log-hooks", predict.Symlinks().Usage("symlinked hook scripts")).
		Predictor("silent", predict.None())
}

// parseLabelKeys extracts completion items from a labels.conf file whose
// lines are KEY=VALUE pairs, demonstrating a custom parser backend for
// predict.FileContents.
func parseLabelKeys(content []byte) []predict.Item {
	var items []predict.Item

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, _, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			continue
		}

		items = append(items, predict.NewItem(key, "label from labels.conf"))
	}

	if scanner.Err() != nil {
		return nil
	}

	return items
}

// slowShardRegistry simulates a registry too slow to answer within the
// predictor's Timeout, so the fallback list is served instead.
func slowShardRegistry() predict.Predictor {
	time.Sleep(300 * time.Millisecond)

	return predict.Values("shard-9", "shard-8").Usage("slow registry results")
}
