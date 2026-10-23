package analyzer_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/conga-sh/conga/internal/analyzer"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	testData := analysistest.TestData()
	analysistest.Run(t, testData, analyzer.Analyzer, "./valid", "./invalid", "./anchorpkg", "./cascading", "./cascadingclean", "./cascadingnoflag", "./cascadingmasked", "./cascadinginnocent", "./cascadingambiguous", "./envprefixambiguous", "./completiondisabled", "./completionenabled", "./completionenabledstmt", "./completionmulti", "./completionescape", "./completionargeak", "./completionforeign", "./featuretypes", "./featurechecks", "./featureenabled", "./featuredisabled", "./featureenvvalid", "./featureenvabsence", "./featureenvconflict", "./featureenvcollision", "./configconflict", "./configconflictsiblings", "./loaderclean", "./loaderconflictpaths", "./loaderconflictcascading", "./loaderconflictdecoder", "./loaderconflictconfigfile", "./loaderstraightline", "./loaderambiguous", "./loadermultianchor", "./loaderescape", "./bootstrapclean", "./bootstrapnoloader", "./bootstrapfeature", "./bootstraptype", "./bootstraprecursive", "./sessionroot")
}
