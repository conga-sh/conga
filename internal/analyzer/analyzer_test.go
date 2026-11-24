package analyzer_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/conga-sh/conga/internal/analyzer"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, "./valid", "./invalid", "./anchorpkg", "./cascading", "./cascadingclean", "./cascadingnoflag", "./cascadingmasked", "./cascadinginnocent", "./cascadingambiguous", "./envprefixambiguous", "./completiondisabled", "./completionenabled", "./completionenabledstmt", "./completionmulti", "./completionescape", "./completionargeak", "./completionforeign", "./featuretypes", "./featurechecks", "./featureenabled", "./featuredisabled", "./featureenvvalid", "./featureenvabsence", "./featureenvconflict", "./featureenvcollision", "./featureimplicit", "./configconflict", "./configenvcollision", "./configenvdecoder", "./configenvnodecoder", "./configenvclean", "./configconflictsiblings", "./bootstrapclean", "./bootstrapnobootstrapper", "./bootstrapfeature", "./bootstraptype", "./bootstraprecursive", "./sessionroot", "./shortsambiguous", "./shortsdisabled", "./shortcollisions", "./shortpaircollision", "./shortsinnocent", "./shortversionlegal", "./versionchecks", "./enumtarget", "./enumtargetclean")
}

func TestAnalyzer_LeadingZeroBoundReportsNoteAndHintAsRelatedLines(t *testing.T) {
	t.Parallel()

	results := analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, "./invalid")

	assert.Equal(t, 1, len(results))

	var related []string

	for _, diagnostic := range results[0].Diagnostics {
		if diagnostic.Message != `struct LeadingZeroRangeApp field Width: invalid integer min bound "010"` {
			continue
		}

		for _, information := range diagnostic.Related {
			related = append(related, information.Message)
		}
	}

	assert.Equal(t, []string{
		"note: a leading zero is ambiguous",
		"hint: write 0o10 for octal or 10 for decimal",
	}, related)
}
