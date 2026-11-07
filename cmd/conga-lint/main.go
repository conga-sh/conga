// Command conga-lint runs CONGA's static schema analyzer over Go packages.
//
// A plain report run (analyzer flags, -test, and package patterns) prints each
// diagnostic with its file path relative to the working directory, matching
// the paths go vet prints. Every other invocation, including -fix, -diff,
// -json, -c, -debug, -help, and the go vet -vettool protocol, runs the
// standard golang.org/x/tools singlechecker driver unchanged.
//
// Building and running conga-lint requires a Go 1.27+ toolchain so method-level
// generic type parameters are supported at parse time.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/analysis/singlechecker"
	"golang.org/x/tools/go/packages"

	"github.com/conga-sh/conga/internal/analyzer"
)

// Exit codes shared with the singlechecker driver: 1 when loading or analysis
// fails, 3 when analysis succeeds and reports diagnostics.
const (
	exitFailure     = 1
	exitDiagnostics = 3
)

// errNoPackages reports package patterns that match nothing, worded as the
// singlechecker driver words it.
var errNoPackages = errors.New("matched no packages")

func main() {
	if exitCode, handled := runRelative(analyzer.Analyzer, os.Args[1:], os.Stderr); handled {
		os.Exit(exitCode)
	}

	singlechecker.Main(analyzer.Analyzer)
}

// runRelative runs a plain report invocation and prints diagnostics with
// working-directory-relative paths. It reports handled=false, without side
// effects beyond analyzer flag values, for any invocation it does not
// recognise, so the caller can delegate to singlechecker.
func runRelative(lintAnalyzer *analysis.Analyzer, arguments []string, output io.Writer) (int, bool) {
	if isVetInvocation(arguments) {
		return 0, false
	}

	flagSet := flag.NewFlagSet(lintAnalyzer.Name, flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	lintAnalyzer.Flags.VisitAll(func(analyzerFlag *flag.Flag) {
		flagSet.Var(analyzerFlag.Value, analyzerFlag.Name, analyzerFlag.Usage)
	})

	includeTests := flagSet.Bool("test", true, "indicates whether test files should be analyzed, too")

	if err := flagSet.Parse(arguments); err != nil {
		return 0, false
	}

	patterns := flagSet.Args()
	if len(patterns) == 0 {
		return 0, false
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return 0, false
	}

	return analyzePatterns(lintAnalyzer, patterns, *includeTests, workingDirectory, output), true
}

// isVetInvocation reports whether the arguments follow the go vet -vettool
// protocol, which passes a single JSON configuration file ending in .cfg.
func isVetInvocation(arguments []string) bool {
	return len(arguments) == 1 && strings.HasSuffix(arguments[0], ".cfg")
}

// analyzePatterns loads the packages matching patterns, runs the analyzer, and
// writes errors and diagnostics to output, returning the singlechecker exit code.
func analyzePatterns(lintAnalyzer *analysis.Analyzer, patterns []string, includeTests bool, workingDirectory string, output io.Writer) int {
	analyzers := []*analysis.Analyzer{lintAnalyzer}

	mode := packages.LoadSyntax
	if needFacts(analyzers) {
		mode = packages.LoadAllSyntax
	}

	config := &packages.Config{
		Mode:  mode | packages.NeedModule,
		Tests: includeTests,
	}

	initial, err := packages.Load(config, patterns...)
	if err == nil && len(initial) == 0 {
		err = fmt.Errorf("%s %w", strings.Join(patterns, " "), errNoPackages)
	}

	if err != nil {
		fmt.Fprintf(output, "%s: %v\n", lintAnalyzer.Name, err)

		return exitFailure
	}

	exitCode := 0
	if packages.PrintErrors(initial) > 0 {
		exitCode = exitFailure
	}

	graph, err := checker.Analyze(analyzers, initial, nil)
	if err != nil {
		fmt.Fprintf(output, "%s: %v\n", lintAnalyzer.Name, err)

		return exitFailure
	}

	return max(exitCode, printRelativeDiagnostics(graph, workingDirectory, output))
}

// printRelativeDiagnostics mirrors checker.Graph.PrintText without context
// lines: it prints action errors and de-duplicated root diagnostics, with each
// file path made relative to workingDirectory when the file lies beneath it.
func printRelativeDiagnostics(graph *checker.Graph, workingDirectory string, output io.Writer) int {
	type diagnosticKey struct {
		position string
		end      string
		message  string
	}

	seen := make(map[diagnosticKey]bool)
	failedActions, rootDiagnostics := 0, 0

	for action := range graph.All() {
		if action.Err != nil {
			fmt.Fprintf(output, "%s: %v\n", action.Analyzer.Name, action.Err)

			failedActions++

			continue
		}

		if !action.IsRoot {
			continue
		}

		rootDiagnostics += len(action.Diagnostics)

		for _, diagnostic := range action.Diagnostics {
			position := action.Package.Fset.Position(diagnostic.Pos)
			key := diagnosticKey{
				position: position.String(),
				end:      action.Package.Fset.Position(diagnostic.End).String(),
				message:  diagnostic.Message,
			}

			if seen[key] {
				continue
			}

			seen[key] = true
			position.Filename = relativePath(workingDirectory, position.Filename)

			fmt.Fprintf(output, "%s: %s\n", position, diagnostic.Message)
		}
	}

	switch {
	case failedActions > 0:
		return exitFailure
	case rootDiagnostics > 0:
		return exitDiagnostics
	default:
		return 0
	}
}

// relativePath returns filename relative to workingDirectory when it lies
// beneath it, and filename unchanged otherwise.
func relativePath(workingDirectory, filename string) string {
	relative, err := filepath.Rel(workingDirectory, filename)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filename
	}

	return relative
}

// needFacts reports whether any analyzer, or any analyzer it requires, uses
// facts, in which case dependencies must be loaded from source too.
func needFacts(analyzers []*analysis.Analyzer) bool {
	seen := make(map[*analysis.Analyzer]bool)

	queue := append([]*analysis.Analyzer(nil), analyzers...)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if seen[current] {
			continue
		}

		seen[current] = true

		if len(current.FactTypes) > 0 {
			return true
		}

		queue = append(queue, current.Requires...)
	}

	return false
}
