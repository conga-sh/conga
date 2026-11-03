package spec_test

import (
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
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
		IsRunner: func(target any) bool {
			_, ok := target.(*completionRunnableImplicitRoot)

			return ok
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

func commandNames(commands []*spec.Command) []string {
	names := make([]string, 0, len(commands))
	for _, cmd := range commands {
		names = append(names, cmd.Name)
	}

	return names
}
