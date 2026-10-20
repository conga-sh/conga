package analyzer_test

import (
	"flag"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/conga-sh/conga/internal/analyzer"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	testData := analysistest.TestData()
	analysistest.Run(t, testData, analyzer.Analyzer, "./valid", "./invalid", "./anchorpkg", "./cascading", "./cascadingclean", "./cascadingnoflag", "./cascadingmasked", "./cascadinginnocent", "./cascadingambiguous", "./envprefixambiguous", "./completiondisabled", "./completionenabled", "./completionenabledstmt", "./completionmulti", "./completionescape", "./completionargeak", "./completionforeign", "./featuretypes", "./featurechecks", "./featureenabled", "./featuredisabled", "./featureenvvalid", "./featureenvabsence", "./featureenvconflict", "./featureenvcollision", "./configconflict", "./configconflictsiblings", "./loaderclean", "./loaderconflictpaths", "./loaderconflictcascading", "./loaderconflictdecoder", "./loaderconflictconfigfile", "./loaderstraightline", "./loaderambiguous", "./loadermultianchor", "./loaderescape", "./bootstrapclean", "./bootstrapnoloader", "./bootstrapfeature", "./bootstraptype", "./bootstraprecursive", "./sessionroot")
}

func TestAnalyzerStrictFlag(t *testing.T) {
	t.Parallel()

	testData := analysistest.TestData()
	analysistest.Run(t, testData, newStrictAnalyzer(), "./strictflag")
}

// newStrictAnalyzer clones the conga-lint analyzer with the strict override
// enabled, avoiding mutation of the shared global flag set.
func newStrictAnalyzer() *analysis.Analyzer {
	var flags flag.FlagSet
	flags.Bool("strict", true, "force strict tag validation regardless of detected app.StrictTagValidation calls")

	return &analysis.Analyzer{
		Name:     analyzer.Analyzer.Name,
		Doc:      analyzer.Analyzer.Doc,
		Run:      analyzer.Analyzer.Run,
		Requires: analyzer.Analyzer.Requires,
		Flags:    flags,
	}
}
