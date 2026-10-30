package analyzer_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/conga-sh/conga/internal/analyzer"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, "./valid", "./invalid", "./anchorpkg", "./cascading", "./cascadingclean", "./cascadingnoflag", "./cascadingmasked", "./cascadinginnocent", "./cascadingambiguous", "./envprefixambiguous", "./completiondisabled", "./completionenabled", "./completionenabledstmt", "./completionmulti", "./completionescape", "./completionargeak", "./completionforeign", "./featuretypes", "./featurechecks", "./featureenabled", "./featuredisabled", "./featureenvvalid", "./featureenvabsence", "./featureenvconflict", "./featureenvcollision", "./configconflict", "./configconflictsiblings", "./bootstrapclean", "./bootstrapnobootstrapper", "./bootstrapfeature", "./bootstraptype", "./bootstraprecursive", "./sessionroot", "./shortsambiguous", "./shortsdisabled", "./shortcollisions", "./shortpaircollision", "./shortsinnocent", "./shortversionlegal")
}
