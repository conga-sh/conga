package spec_test

import (
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

type completionImplicitLeaf struct {
	DryRun mockFlag[bool]     `short:"n"`
	File   mockReqArg[string] `enum:"a.ts,b.ts"`
}

type completionImplicitRoot struct {
	Exec  mockCmd[completionImplicitLeaf] `implicit:"true" name:"run"`
	Build mockCmd[completionImplicitLeaf] `name:"build"`
}

type completionRunnableImplicitRoot struct {
	Exec mockCmd[completionImplicitLeaf] `implicit:"true" name:"run"`
}

type completionDeepLeaf struct {
	Deep mockFlag[string] `name:"deep"`
}

type completionDeepMid struct {
	JSON mockCmd[completionDeepLeaf] `name:"json"`
	Run  mockCmd[completionDeepLeaf] `implicit:"true" name:"run"`
}

type completionDeepRoot struct {
	Mid mockCmd[completionDeepMid] `implicit:"true" name:"mid"`
}

type completionHiddenImplicitLeaf struct {
	SecretFlag mockFlag[string]   `name:"hidden-flag"`
	File       mockReqArg[string] `enum:"secret-a.ts,secret-b.ts"`
}

type completionHiddenImplicitRoot struct {
	Verbose mockFlag[bool]                        `name:"verbose" short:"v"`
	Exec    mockCmd[completionHiddenImplicitLeaf] `hidden:"true"  implicit:"true" name:"run"`
	Build   mockCmd[completionImplicitLeaf]       `name:"build"`
}

type completionDeprecatedImplicitLeaf struct {
	File mockReqArg[string] `enum:"old-a.ts,old-b.ts"`
}

type completionDeprecatedImplicitRoot struct {
	Exec  mockCmd[completionDeprecatedImplicitLeaf] `deprecated:"use build instead" implicit:"true" name:"run"`
	Build mockCmd[completionImplicitLeaf]           `name:"build"`
}

type completionSiblingLeaf struct {
	Frozen mockFlag[bool] `name:"frozen"`
}

type completionRunnableSiblingRoot struct {
	Exec  mockCmd[completionImplicitLeaf] `implicit:"true" name:"run"`
	Build mockCmd[completionSiblingLeaf]  `name:"build"`
}

type completionDeepRunnableMid struct {
	MidFlag mockFlag[bool]              `name:"mid-flag"`
	JSON    mockCmd[completionDeepLeaf] `name:"json"`
	Run     mockCmd[completionDeepLeaf] `implicit:"true" name:"run"`
}

type completionDeepRunnableRoot struct {
	Mid mockCmd[completionDeepRunnableMid] `implicit:"true" name:"mid"`
}

type completionDeepFlagLeaf struct {
	Deep mockFlag[bool]     `name:"deep"`
	File mockReqArg[string] `enum:"deep-a.ts,deep-b.ts"`
}

type completionDeepFlagMid struct {
	JSON mockCmd[completionDeepLeaf]     `name:"json"`
	Run  mockCmd[completionDeepFlagLeaf] `implicit:"true" name:"run"`
}

type completionDeepFlagRoot struct {
	Mid mockCmd[completionDeepFlagMid] `implicit:"true" name:"mid"`
}

type completionExplicitTask struct{}

type completionExplicitRunBranch struct {
	Deep mockFlag[bool]                  `name:"deep"`
	Task mockCmd[completionExplicitTask] `name:"task"`
}

type completionExplicitMid struct {
	JSON mockCmd[completionExplicitTask]      `name:"json"`
	Run  mockCmd[completionExplicitRunBranch] `implicit:"true" name:"run"`
}

type completionExplicitRoot struct {
	Mid mockCmd[completionExplicitMid] `name:"mid"`
}

type completionVersionShortShadowLeaf struct {
	Verbose mockFlag[bool] `short:"V"`
}

type completionVersionShortShadowRoot struct {
	Exec mockCmd[completionVersionShortShadowLeaf] `implicit:"true" name:"run"`
}

type completionVersionLongShadowLeaf struct {
	Version mockFlag[bool]
}

type completionVersionLongShadowRoot struct {
	Exec mockCmd[completionVersionLongShadowLeaf] `implicit:"true" name:"run"`
}

type completionDashLeaf struct {
	Rest mockDashArgs
}

type completionDashMid struct {
	MidFlag mockFlag[bool]              `name:"mid-flag"`
	Run     mockCmd[completionDashLeaf] `implicit:"true" name:"run"`
}

type completionDashRoot struct {
	Rest mockDashArgs
	Mid  mockCmd[completionDashMid] `implicit:"true" name:"mid"`
}

type completionDashPredictLeaf struct {
	Rest mockDashArgs `predictor:"leaf-dash"`
}

type completionDashPredictMid struct {
	MidFlag mockFlag[bool]                     `name:"mid-flag"`
	Run     mockCmd[completionDashPredictLeaf] `implicit:"true" name:"run"`
}

type completionDashPredictRoot struct {
	Rest mockDashArgs                      `predictor:"root-dash"`
	Mid  mockCmd[completionDashPredictMid] `implicit:"true"       name:"mid"`
}

func buildCompletionImplicitRoot(t *testing.T, root any) *spec.Command {
	t.Helper()

	cmd, err := spec.Build(root, "tool", spec.Options{ClassifyContainer: classifyTestContainer})
	assert.NoError(t, err)

	return cmd
}

func buildCompletionRunnableImplicitRoot(t *testing.T) *spec.Command {
	t.Helper()

	cmd, err := spec.Build(&completionRunnableImplicitRoot{}, "tool", spec.Options{
		ClassifyContainer: classifyTestContainer,
		HelpShort:         'h',
		IsRunner: func(target any) bool {
			_, ok := target.(*completionRunnableImplicitRoot)

			return ok
		},
	})
	assert.NoError(t, err)

	return cmd
}

func buildCompletionRunnableSiblingRoot(t *testing.T) *spec.Command {
	t.Helper()

	cmd, err := spec.Build(&completionRunnableSiblingRoot{}, "tool", spec.Options{
		ClassifyContainer: classifyTestContainer,
		IsRunner: func(target any) bool {
			_, ok := target.(*completionRunnableSiblingRoot)

			return ok
		},
	})
	assert.NoError(t, err)

	return cmd
}

func buildCompletionDeepRunnableRoot(t *testing.T) *spec.Command {
	t.Helper()

	cmd, err := spec.Build(&completionDeepRunnableRoot{}, "tool", spec.Options{
		ClassifyContainer: classifyTestContainer,
		IsRunner: func(target any) bool {
			_, ok := target.(*completionDeepRunnableMid)

			return ok
		},
	})
	assert.NoError(t, err)

	return cmd
}

func buildCompletionExplicitRoot(t *testing.T) *spec.Command {
	t.Helper()

	cmd, err := spec.Build(&completionExplicitRoot{}, "tool", spec.Options{
		ClassifyContainer: classifyTestContainer,
		IsRunner: func(target any) bool {
			_, ok := target.(*completionExplicitMid)

			return ok
		},
	})
	assert.NoError(t, err)

	return cmd
}

func buildCompletionDashRoot(t *testing.T) *spec.Command {
	t.Helper()

	cmd, err := spec.Build(&completionDashRoot{}, "tool", spec.Options{
		ClassifyContainer: classifyTestContainer,
		IsRunner: func(any) bool {
			return true
		},
	})
	assert.NoError(t, err)

	return cmd
}

func TestComplete_ImplicitCommandCandidates(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

	result := spec.Complete(root, []string{""}, spec.CompleteOptions{})

	values := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		values = append(values, item.Value)
	}

	assert.True(t, slices.Contains(values, "run"))
	assert.True(t, slices.Contains(values, "build"))
	assert.True(t, slices.Contains(values, "a.ts"))
	assert.True(t, slices.Contains(values, "b.ts"))
}

func TestComplete_ImplicitCommandFlagBeforeOperand(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

	result := spec.Complete(root, []string{"--dry"}, spec.CompleteOptions{})

	values := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		values = append(values, item.Value)
	}

	assert.True(t, slices.Contains(values, "--dry-run"))
}

func TestAnalyzeCompletion_ImplicitCommandChain(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

	analysis := spec.AnalyzeCompletion(root, []string{""})

	assert.Equal(t, []string{"tool", "run"}, commandNames(analysis.CommandChain))
}

func TestAnalyzeCompletion_RunnableParentKeepsChain(t *testing.T) {
	t.Parallel()

	root := buildCompletionRunnableImplicitRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{""})

	assert.Equal(t, []string{"tool"}, commandNames(analysis.CommandChain))
}

func TestAnalyzeCompletion_FlagSelectsRunnableImplicit(t *testing.T) {
	t.Parallel()

	root := buildCompletionRunnableImplicitRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"-n", ""})

	assert.Equal(t, []string{"tool", "run"}, commandNames(analysis.CommandChain))
}

func TestAnalyzeCompletion_ChildFlagBeforeHelpSelectsImplicit(t *testing.T) {
	t.Parallel()

	root := buildCompletionRunnableImplicitRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"-n", "--help", ""})

	assert.Equal(t, []string{"tool", "run"}, commandNames(analysis.CommandChain))
}

func TestAnalyzeCompletion_SiblingFlagDoesNotSelectImplicit(t *testing.T) {
	t.Parallel()

	root := buildCompletionRunnableSiblingRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"--frozen", ""})

	assert.Equal(t, []string{"tool"}, commandNames(analysis.CommandChain))
}

func TestAnalyzeCompletion_DeepChainFlagSelectsNeededDepth(t *testing.T) {
	t.Parallel()

	t.Run("mid flag stops at the runnable intermediate", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionDeepRunnableRoot(t)

		analysis := spec.AnalyzeCompletion(root, []string{"--mid-flag", ""})

		assert.Equal(t, []string{"tool", "mid"}, commandNames(analysis.CommandChain))
	})

	t.Run("leaf flag descends to the implicit leaf", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionDeepRunnableRoot(t)

		analysis := spec.AnalyzeCompletion(root, []string{"--deep", ""})

		assert.Equal(t, []string{"tool", "mid", "run"}, commandNames(analysis.CommandChain))
	})
}

func TestAnalyzeCompletion_FlagBeforeExplicitNameRoutesToChild(t *testing.T) {
	t.Parallel()

	root := buildCompletionExplicitRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"--deep", "mid", ""})

	assert.Equal(t, []string{"tool", "mid", "run"}, commandNames(analysis.CommandChain))
}

func TestComplete_FlagBeforeExplicitNameSelectsChild(t *testing.T) {
	t.Parallel()

	root := buildCompletionExplicitRoot(t)

	result := spec.Complete(root, []string{"--deep", "mid", ""}, spec.CompleteOptions{})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "task"))
	assert.False(t, slices.Contains(values, "run"))
}

func TestAnalyzeCompletion_PendingFlagBeforeTerminatorRoutesToChild(t *testing.T) {
	t.Parallel()

	root := buildCompletionDeepRunnableRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"--deep", "--", ""})

	assert.Equal(t, []string{"tool", "mid", "run"}, commandNames(analysis.CommandChain))
}

func TestAnalyzeCompletion_BuiltinShortInClusterSelectsImplicit(t *testing.T) {
	t.Parallel()

	root := buildCompletionRunnableImplicitRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"-hn", ""})

	assert.Equal(t, []string{"tool", "run"}, commandNames(analysis.CommandChain))
}

func TestAnalyzeCompletion_ChildFlagBeforeVersionRoutesToChild(t *testing.T) {
	t.Parallel()

	root := buildCompletionRunnableImplicitRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"-n", "--version", ""})

	assert.Equal(t, []string{"tool", "run"}, commandNames(analysis.CommandChain))
}

func TestAnalyzeCompletion_TerminatorOperandsFollowFlagSelectedChain(t *testing.T) {
	t.Parallel()

	root := buildCompletionDashRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"--mid-flag", "--", "x", ""})

	assert.Equal(t, []string{"tool", "mid", "run"}, commandNames(analysis.CommandChain))
	assert.Equal(t, []string{"x"}, analysis.DashArgs)
}

func TestAnalyzeCompletion_BuiltinRequestKeepsChain(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

	analysis := spec.AnalyzeCompletion(root, []string{"--help", ""})

	assert.Equal(t, []string{"tool"}, commandNames(analysis.CommandChain))
}

func TestAnalyzeCompletion_ImplicitCommandPositionals(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

	analysis := spec.AnalyzeCompletion(root, []string{"script.ts", ""})

	assert.Equal(t, []string{"tool", "run"}, commandNames(analysis.CommandChain))
	assert.Equal(t, []string{"script.ts"}, analysis.PositionalArgs)
}

func TestComplete_ImplicitCommandTerminatorCandidates(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

	result := spec.Complete(root, []string{"--", ""}, spec.CompleteOptions{})

	values := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		values = append(values, item.Value)
	}

	assert.True(t, slices.Contains(values, "a.ts"))
	assert.True(t, slices.Contains(values, "b.ts"))
}

func TestAnalyzeCompletion_ImplicitTerminatorChain(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

	analysis := spec.AnalyzeCompletion(root, []string{"--", "script.ts", ""})

	assert.Equal(t, []string{"tool", "run"}, commandNames(analysis.CommandChain))
	assert.Equal(t, []string{"script.ts"}, analysis.PositionalArgs)
}

func TestAnalyzeCompletion_BuiltinVersionKeepsRootChain(t *testing.T) {
	t.Parallel()

	t.Run("long flag", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

		analysis := spec.AnalyzeCompletion(root, []string{"--version", ""})

		assert.Equal(t, []string{"tool"}, commandNames(analysis.CommandChain))
	})

	t.Run("short flag", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})
		root.VersionShort = 'V'

		analysis := spec.AnalyzeCompletion(root, []string{"-V", ""})

		assert.Equal(t, []string{"tool"}, commandNames(analysis.CommandChain))
	})
}

func TestAnalyzeCompletion_BuiltinBeforeTerminatorStopsAtRequiredDepth(t *testing.T) {
	t.Parallel()

	root := buildCompletionDeepRunnableRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"--mid-flag", "--version", "--", "x", ""})

	assert.Equal(t, []string{"tool", "mid"}, commandNames(analysis.CommandChain))
	assert.Equal(t, []string{"x"}, analysis.PositionalArgs)
}

func TestAnalyzeCompletion_VersionBuiltinNotShadowedByImplicitCommand(t *testing.T) {
	t.Parallel()

	t.Run("short flag", func(t *testing.T) {
		t.Parallel()

		root, err := spec.Build(&completionVersionShortShadowRoot{}, "tool", spec.Options{
			ClassifyContainer: classifyTestContainer,
			VersionShort:      'V',
		})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(root, []string{"-V", ""})

		assert.Equal(t, []string{"tool"}, commandNames(analysis.CommandChain))
	})

	t.Run("long flag", func(t *testing.T) {
		t.Parallel()

		root, err := spec.Build(&completionVersionLongShadowRoot{}, "tool", spec.Options{
			ClassifyContainer: classifyTestContainer,
		})
		assert.NoError(t, err)

		analysis := spec.AnalyzeCompletion(root, []string{"--version", ""})

		assert.Equal(t, []string{"tool"}, commandNames(analysis.CommandChain))
	})
}

func TestComplete_DeepImplicitChainFlag(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionDeepRoot{})

	result := spec.Complete(root, []string{"--de"}, spec.CompleteOptions{})

	values := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		values = append(values, item.Value)
	}

	assert.True(t, slices.Contains(values, "--deep"))
}

func TestComplete_FlagSelectedDeepImplicitPositionals(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionDeepFlagRoot{})

	result := spec.Complete(root, []string{"--deep", ""}, spec.CompleteOptions{})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "deep-a.ts"))
	assert.True(t, slices.Contains(values, "deep-b.ts"))
}

func TestComplete_FlagSelectedImplicitNarrowsSiblingSurface(t *testing.T) {
	t.Parallel()

	root := buildCompletionRunnableSiblingRoot(t)

	result := spec.Complete(root, []string{"-n", ""}, spec.CompleteOptions{})

	values := itemValues(result.Items)

	assert.True(t, slices.Contains(values, "a.ts"))
	assert.True(t, slices.Contains(values, "b.ts"))
	assert.False(t, slices.Contains(values, "build"))
}

func TestComplete_FlagSelectedHiddenImplicitStaysSuppressed(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionHiddenImplicitRoot{})

	result := spec.Complete(root, []string{"--hidden-flag=value", ""}, spec.CompleteOptions{})

	values := itemValues(result.Items)

	assert.False(t, slices.Contains(values, "secret-a.ts"))
	assert.False(t, slices.Contains(values, "secret-b.ts"))
	assert.True(t, slices.Contains(values, "build"))
}

func TestAnalyzeCompletion_BuiltinBeforeTerminatorKeepsChain(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

	analysis := spec.AnalyzeCompletion(root, []string{"--help", "--", ""})

	assert.Equal(t, []string{"tool"}, commandNames(analysis.CommandChain))
}

func TestComplete_BuiltinBeforeTerminatorOmitsImplicitOperands(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionImplicitRoot{})

	result := spec.Complete(root, []string{"--help", "--", ""}, spec.CompleteOptions{})

	values := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		values = append(values, item.Value)
	}

	assert.False(t, slices.Contains(values, "a.ts"))
	assert.False(t, slices.Contains(values, "b.ts"))
}

func TestAnalyzeCompletion_BareTerminatorKeepsRunnableChain(t *testing.T) {
	t.Parallel()

	t.Run("no operand follows", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionRunnableImplicitRoot(t)

		analysis := spec.AnalyzeCompletion(root, []string{"--", ""})

		assert.Equal(t, []string{"tool"}, commandNames(analysis.CommandChain))
	})

	t.Run("operand follows", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionRunnableImplicitRoot(t)

		analysis := spec.AnalyzeCompletion(root, []string{"--", "x", ""})

		assert.Equal(t, []string{"tool", "run"}, commandNames(analysis.CommandChain))
	})
}

func TestComplete_BareTerminatorTreatsFragmentAsOperand(t *testing.T) {
	t.Parallel()

	root := buildCompletionRunnableImplicitRoot(t)

	result := spec.Complete(root, []string{"--", ""}, spec.CompleteOptions{})

	values := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		values = append(values, item.Value)
	}

	assert.True(t, slices.Contains(values, "a.ts"))
	assert.True(t, slices.Contains(values, "b.ts"))
}

func TestAnalyzeCompletion_NegativeNumberRoutesToImplicit(t *testing.T) {
	t.Parallel()

	root := buildCompletionRunnableImplicitRoot(t)

	analysis := spec.AnalyzeCompletion(root, []string{"-5", ""})

	assert.Equal(t, []string{"tool", "run"}, commandNames(analysis.CommandChain))
	assert.Equal(t, []string{"-5"}, analysis.PositionalArgs)
}

func TestComplete_HiddenImplicitCommandSurfaceSuppressed(t *testing.T) {
	t.Parallel()

	t.Run("operand candidates", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionImplicitRoot(t, &completionHiddenImplicitRoot{})

		result := spec.Complete(root, []string{""}, spec.CompleteOptions{})

		values := make([]string, 0, len(result.Items))
		for _, item := range result.Items {
			values = append(values, item.Value)
		}

		assert.False(t, slices.Contains(values, "secret-a.ts"))
		assert.False(t, slices.Contains(values, "secret-b.ts"))
		assert.False(t, slices.Contains(values, "run"))
		assert.True(t, slices.Contains(values, "build"))
	})

	t.Run("flag candidates", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionImplicitRoot(t, &completionHiddenImplicitRoot{})

		hiddenResult := spec.Complete(root, []string{"--hidden"}, spec.CompleteOptions{})

		hiddenValues := make([]string, 0, len(hiddenResult.Items))
		for _, item := range hiddenResult.Items {
			hiddenValues = append(hiddenValues, item.Value)
		}

		assert.False(t, slices.Contains(hiddenValues, "--hidden-flag"))

		visibleResult := spec.Complete(root, []string{"--verb"}, spec.CompleteOptions{})

		visibleValues := make([]string, 0, len(visibleResult.Items))
		for _, item := range visibleResult.Items {
			visibleValues = append(visibleValues, item.Value)
		}

		assert.True(t, slices.Contains(visibleValues, "--verbose"))
	})
}

func TestComplete_DeprecatedImplicitCommandSurfaceSuppressed(t *testing.T) {
	t.Parallel()

	root := buildCompletionImplicitRoot(t, &completionDeprecatedImplicitRoot{})

	result := spec.Complete(root, []string{""}, spec.CompleteOptions{})

	values := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		values = append(values, item.Value)
	}

	assert.False(t, slices.Contains(values, "old-a.ts"))
	assert.False(t, slices.Contains(values, "old-b.ts"))
	assert.False(t, slices.Contains(values, "run"))
	assert.True(t, slices.Contains(values, "build"))
}

func TestComplete_VersionFlagLiteralRootOnly(t *testing.T) {
	t.Parallel()

	t.Run("child-only flag removes the root-only candidate", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionRunnableImplicitRoot(t)

		result := spec.Complete(root, []string{"-n", "-"}, spec.CompleteOptions{})

		assert.False(t, slices.Contains(itemValues(result.Items), "--version"))
	})

	t.Run("root flag keeps the root-only candidate", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionImplicitRoot(t, &completionHiddenImplicitRoot{})

		result := spec.Complete(root, []string{"-v", "-"}, spec.CompleteOptions{})

		assert.True(t, slices.Contains(itemValues(result.Items), "--version"))
	})

	t.Run("implicit operand enters the child", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionRunnableImplicitRoot(t)

		result := spec.Complete(root, []string{"script.ts", "-"}, spec.CompleteOptions{})

		assert.False(t, slices.Contains(itemValues(result.Items), "--version"))
	})

	t.Run("explicit name enters the child", func(t *testing.T) {
		t.Parallel()

		root := buildCompletionRunnableImplicitRoot(t)

		result := spec.Complete(root, []string{"run", "-"}, spec.CompleteOptions{})

		assert.False(t, slices.Contains(itemValues(result.Items), "--version"))
	})
}

func TestComplete_TerminatorFollowsFlagSelectedChain(t *testing.T) {
	t.Parallel()

	predictors := map[string]predict.Predictor{
		"root-dash": predict.Items(predict.Item{Value: "ROOT-DASH"}),
		"leaf-dash": predict.Items(predict.Item{Value: "LEAF-DASH"}),
	}

	root, err := spec.Build(&completionDashPredictRoot{}, "tool", spec.Options{
		ClassifyContainer:    classifyTestContainer,
		RegisteredPredictors: predictors,
		IsRunner: func(target any) bool {
			_, ok := target.(*completionDashPredictRoot)

			return ok
		},
	})
	assert.NoError(t, err)

	result := spec.Complete(root, []string{"--mid-flag", "--", "x", ""}, spec.CompleteOptions{RegisteredPredictors: predictors})

	assert.Equal(t, []string{"LEAF-DASH"}, itemValues(result.Items))
}

func commandNames(commands []*spec.Command) []string {
	names := make([]string, 0, len(commands))
	for _, cmd := range commands {
		names = append(names, cmd.Name)
	}

	return names
}

func itemValues(items []predict.Item) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.Value)
	}

	return values
}
