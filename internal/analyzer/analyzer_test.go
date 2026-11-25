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

func TestAnalyzer_RangeBoundDiagnosticsCarryNoteAndHint(t *testing.T) {
	t.Parallel()

	results := analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, "./rangenotes")

	assert.Equal(t, 1, len(results))

	related := map[string][]string{}

	for _, diagnostic := range results[0].Diagnostics {
		lines := make([]string, 0, len(diagnostic.Related))

		for _, information := range diagnostic.Related {
			lines = append(lines, information.Message)
		}

		related[diagnostic.Message] = lines
	}

	assert.Equal(t, map[string][]string{
		`struct RangeNotesApp field Syntax: invalid integer min bound "banana"`: {},
		`struct RangeNotesApp field Float: invalid float min bound "abc"`:       {},
		`struct RangeNotesApp field Duration: invalid duration min bound "1x"`:  {},
		`struct RangeNotesApp field Zero: invalid integer min bound "07"`: {
			"note: a leading zero is ambiguous",
			"hint: write 0o7 for octal or 7 for decimal",
		},
		`struct RangeNotesApp field Huge: invalid integer min bound "99999999999999999999"`: {
			"note: is out of range",
		},
		`struct RangeNotesApp field Narrow: invalid float max bound "1e39"`: {
			"note: is out of range",
		},
		`struct RangeNotesApp field Size: invalid byte size min bound "5XB"`: {
			"note: accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB",
		},
	}, related)
}
