//go:build sizetest

package size_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

const (
	minimalExamplePath    = "../../_examples/01-minimal-hello"
	versionExamplePath    = "../../_examples/10-builtins-help-version"
	whyExamplePath        = "../../_examples/13-twelve-factor-precedence"
	completionExamplePath = "../../_examples/19-shell-completion"
)

func TestMinimalHello_DotenvPayloadEliminated(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")

	for _, symbol := range []string{
		"github.com/conga-sh/conga/internal/dotenv.Load",
		"github.com/conga-sh/conga/internal/dotenv.loadDefaultHierarchy",
		"github.com/conga-sh/conga/internal/dotenv.loadSingleFile",
	} {
		assert.False(t, symbols[symbol], "linker must drop the dotenv loader payload: "+symbol)
	}
}

func TestMinimalHello_ConfigResolverEliminated(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")

	for symbol := range symbols {
		assert.False(t, strings.Contains(symbol, "internal/configengine.Resolver."), "linker must drop the config resolver: "+symbol)
	}
}

func TestMinimalHello_ConfigLoaderEliminated(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")

	for symbol := range symbols {
		assert.False(t, strings.Contains(symbol, "resolveLoaderConfig"), "linker must drop the programmatic config loader: "+symbol)
		assert.False(t, strings.Contains(symbol, "loadDocumentConfigData"), "linker must drop loader document validation: "+symbol)
	}
}

func TestMinimalHello_WhyEliminated(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")

	for symbol := range symbols {
		assert.False(t, strings.Contains(symbol, "internal/why."), "linker must drop the why report: "+symbol)
	}
}

func TestMinimalHello_ScanBuiltinRequestInlined(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")
	// scanBuiltinRequest must stay within the compiler's inlining budget (<= 80)
	// so it inlines into App.execute and avoids emitting a standalone symbol.
	assert.False(t, symbols["github.com/conga-sh/conga.scanBuiltinRequest"], "scanBuiltinRequest must stay inlined")
}

func TestMinimalHello_CompletionEliminated(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")

	for _, symbol := range []string{
		"github.com/conga-sh/conga/internal/completion.(*Session).Format",
		"github.com/conga-sh/conga/internal/completion.(*Session).Patch",
		"github.com/conga-sh/conga/internal/completion.Generate",
		"github.com/conga-sh/conga/internal/completion.NewSession",
		"github.com/conga-sh/conga/internal/completion.NewSessionWithEnv",
		"github.com/conga-sh/conga/internal/completion.ParseShell",
		"github.com/conga-sh/conga/internal/completion.ShellDescriptions",
		"github.com/conga-sh/conga/internal/completion.SupportedShells",
		"github.com/conga-sh/conga/internal/completion/bash.(*Session).Format",
		"github.com/conga-sh/conga/internal/completion/bash.(*Session).Patch",
		"github.com/conga-sh/conga/internal/completion/bash.NewSessionWithEnv",
		"github.com/conga-sh/conga/internal/completion/zsh.Format",
		"github.com/conga-sh/conga/internal/spec.Complete",
	} {
		assert.False(t, symbols[symbol], "linker must drop the completion payload: "+symbol)
	}
}

func TestMinimalHello_PredictorInferenceEliminated(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")

	for _, symbol := range []string{
		"github.com/conga-sh/conga/internal/spec.InferPredictorFromType",
		"github.com/conga-sh/conga/internal/spec.inferFilterablePredictor",
		"github.com/conga-sh/conga/internal/spec.isDirectoryEntry",
		"github.com/conga-sh/conga/predict.FilesFiltered",
		"github.com/conga-sh/conga/predict.isEntryDir",
		"github.com/conga-sh/conga/predict.scanFilesystem",
		"os.Stat",
		"os.Lstat",
	} {
		assert.False(t, symbols[symbol], "linker must drop completion-only predictor inference: "+symbol)
	}
}

func TestMinimalHello_OptInInitializersEliminated(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")

	for _, symbol := range []string{
		"github.com/conga-sh/conga/internal/dotenv.init",
		"github.com/conga-sh/conga/internal/configengine.init",
		"github.com/conga-sh/conga/internal/style.init",
		"github.com/conga-sh/conga/internal/filex.init",
		"github.com/conga-sh/conga/predict.init",
		"github.com/conga-sh/conga/internal/completion.init",
		"github.com/conga-sh/conga/internal/completion/wire.init",
		"github.com/conga-sh/conga/internal/completion/bash.init",
		"github.com/conga-sh/conga/internal/completion/elvish.init",
		"github.com/conga-sh/conga/internal/completion/fish.init",
		"github.com/conga-sh/conga/internal/completion/nushell.init",
		"github.com/conga-sh/conga/internal/completion/powershell.init",
		"github.com/conga-sh/conga/internal/completion/zsh.init",
	} {
		assert.False(t, symbols[symbol], "linker must drop opt-in package initializers: "+symbol)
	}
}

func TestMinimalHello_FeatureResolutionPayloadEliminated(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")

	for _, symbol := range []string{
		"github.com/conga-sh/conga.featureValues",
		"github.com/conga-sh/conga.featureValuesLenient",
		"github.com/conga-sh/conga.collectFeatureValues",
		"github.com/conga-sh/conga.bindFeatureValues",
		"github.com/conga-sh/conga.bindFeatureMap",
		"github.com/conga-sh/conga.resolveFeatureGates",
		"github.com/conga-sh/conga.completionFeatureGates",
		"github.com/conga-sh/conga.pruneFeaturesForCompletion",
		"github.com/conga-sh/conga.materializeFeatureValues",
		"github.com/conga-sh/conga.unknownFeatureError",
		"github.com/conga-sh/conga.invalidFeatureValueError",
		"github.com/conga-sh/conga.pruneInactiveFeatures",
		"github.com/conga-sh/conga.joinSubcommandPrefix",
		"github.com/conga-sh/conga.collectPrunedSubtree",
		"github.com/conga-sh/conga.validateSubcommandFeatures",
		"github.com/conga-sh/conga.checkFeatureFlagCLIRestrictions",
		"github.com/conga-sh/conga.checkFeatureConfigRestrictions",
		"github.com/conga-sh/conga/internal/spec.MatchFeature",
		"github.com/conga-sh/conga.featureDefaults",
		"github.com/conga-sh/conga/internal/help.RenderFeatures",
		"github.com/conga-sh/conga/internal/spec.(*FeatureField).ListedInHelp",
	} {
		assert.False(t, symbols[symbol], "linker must drop feature runtime payload: "+symbol)
	}

	for symbol := range symbols {
		assert.False(t, strings.Contains(symbol, "resolveFeatures"), "linker must drop resolveFeatures: "+symbol)
		assert.False(t, strings.Contains(symbol, "rejectPrunedConfig"), "linker must drop rejectPrunedConfig: "+symbol)
		assert.False(t, strings.Contains(symbol, "pruneForCompletion"), "linker must drop pruneForCompletion: "+symbol)
	}
}

func TestMinimalHello_VersionPayloadRetained(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, minimalExamplePath)

	assert.True(t, symbols["main.main"], "sanity check: the analysis build must expose symbols")

	for _, symbol := range []string{
		"runtime/debug.ParseBuildInfo",
		"runtime/debug.ReadBuildInfo",
		"github.com/conga-sh/conga/internal/help.GetBuildInfo",
		"github.com/conga-sh/conga/internal/help.VersionSummary",
	} {
		assert.True(t, symbols[symbol], "the default-on --version flag must link the version payload: "+symbol)
	}
}

func TestVersionExample_RetainsVersionPayload(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, versionExamplePath)

	assert.True(t, symbols["github.com/conga-sh/conga/internal/help.VersionSummary"], "the version flag must link the version summary")
	assert.True(t, symbols["runtime/debug.ReadBuildInfo"], "the version flag must link the build-info probe")
}

func TestOptedInWhyExample_RetainsReport(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, whyExamplePath)

	assert.True(t, symbols["github.com/conga-sh/conga/internal/why.Report.Format"], "app.EnableWhy() must link the why report")
}

func TestOptedInCompletionExample_RetainsDriver(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, completionExamplePath)

	assert.True(t, symbols["github.com/conga-sh/conga/internal/completion.Generate"], "app.EnableCompletion() must link the completion driver")
	assert.True(t, symbols["github.com/conga-sh/conga/internal/completion/bash.(*Session).Format"], "app.EnableCompletion() must link the bash driver")
}

func TestOptedInCompletionExample_RetainsInference(t *testing.T) {
	t.Parallel()

	symbols := buildAndListSymbols(t, completionExamplePath)

	assert.True(t, symbols["github.com/conga-sh/conga/internal/spec.InferPredictorFromType"], "app.EnableCompletion() must link predictor inference")
	assert.True(t, symbols["os.Stat"], "app.EnableCompletion() must link the filesystem probes predictor inference composes")
}

// buildAndListSymbols compiles pkgPath without stripping flags and returns the
// set of symbol names in the resulting executable. The linker eliminates
// unreferenced builder methods and their package references, so absence of a
// symbol here proves the subsystem was dead-code eliminated.
func buildAndListSymbols(t *testing.T, pkgPath string) map[string]bool {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), "analysis-binary")

	buildCommand := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-o", binaryPath, pkgPath)
	if output, err := buildCommand.CombinedOutput(); err != nil {
		t.Fatalf("failed to build %s: %v\nOutput: %s", pkgPath, err, string(output))
	}

	nmCommand := exec.CommandContext(t.Context(), "go", "tool", "nm", binaryPath)

	output, err := nmCommand.Output()
	if err != nil {
		t.Fatalf("failed to list symbols for %s: %v", binaryPath, err)
	}

	symbols := make(map[string]bool)

	for line := range strings.Lines(string(output)) {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			symbols[fields[len(fields)-1]] = true
		}
	}

	return symbols
}
