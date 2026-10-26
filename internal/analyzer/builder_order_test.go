package analyzer_test

import (
	"flag"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/conga-sh/conga/internal/analyzer"
)

// TestBuilderOrderAnalyzer runs the opt-in builder call order check with the
// compiled-in default order.
func TestBuilderOrderAnalyzer(t *testing.T) {
	t.Parallel()

	testData := analysistest.TestData()
	analysistest.Run(t, testData, newBuilderOrderAnalyzer(""), "./builderordervalid", "./builderorderinvalid")
}

// TestBuilderOrderAnalyzerSpecOverride runs the check with a custom spec that
// reorders the identity methods and leaves every other method unconstrained.
func TestBuilderOrderAnalyzerSpecOverride(t *testing.T) {
	t.Parallel()

	analysistest.Run(t, analysistest.TestData(), newBuilderOrderAnalyzer("Version,Name"), "./builderorderspec")
}

// TestBuilderOrderAnalyzerFixes applies the suggested reorderings and compares
// the results against the golden files.
func TestBuilderOrderAnalyzerFixes(t *testing.T) {
	t.Parallel()

	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), newBuilderOrderAnalyzer(""), "./builderorderfix")
}

// newBuilderOrderAnalyzer clones the conga-lint analyzer with the builder call
// order check enabled, applying a spec override when one is supplied.
func newBuilderOrderAnalyzer(specification string) *analysis.Analyzer {
	var flags flag.FlagSet
	flags.Bool("builder-order", true, "check builder call order at conga.New construction sites")

	if specification != "" {
		flags.String("builder-order-spec", specification, "canonical builder call order override")
	}

	return &analysis.Analyzer{
		Name:     analyzer.Analyzer.Name,
		Doc:      analyzer.Analyzer.Doc,
		Run:      analyzer.Analyzer.Run,
		Requires: analyzer.Analyzer.Requires,
		Flags:    flags,
	}
}
