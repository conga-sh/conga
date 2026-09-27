//go:build shelltest

package shell_test

import (
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/test/shell"
)

// TestPowerShellUserVisible_Suite1_SubcommandsAndBareCommand tests user-visible command line buffer
// states and subcommand discovery on root commands in PowerShell.
func TestPowerShellUserVisible_Suite1_SubcommandsAndBareCommand(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("bare_root_command_lists_subcommands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl ")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl ", result.BufferTab1)
		assert.True(t, result.HasItem("deploy"), "expected deploy subcommand")
		assert.True(t, result.HasItem("logs"), "expected logs subcommand")
		assert.True(t, result.HasItem("completion"), "expected completion subcommand")
	})

	t.Run("subcommand_prefix_expands_buffer", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl dep")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl deploy ", result.BufferTab1)
		assert.True(t, result.HasItem("deploy"))
		assert.False(t, result.HasItem("logs"))
	})

	t.Run("subcommand_alias_resolves", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl d")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.True(t, result.HasItem("deploy"))
	})
}

// TestPowerShellUserVisible_Suite2_PositionalArguments tests user-visible completion
// for positional arguments in PowerShell.
func TestPowerShellUserVisible_Suite2_PositionalArguments(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("unconstrained_positional_preserves_buffer_without_dash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy ")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl deploy ", result.BufferTab1, "Tab 1 on unconstrained positional must not insert dash")
	})

	t.Run("unconstrained_positional_flag_discovery_with_dash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy -")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.True(t, result.HasItem("--env") || result.HasItem("-e"))
		assert.True(t, result.HasItem("--cluster") || result.HasItem("-c"))
	})

	t.Run("positional_enum_completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl logs ")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl logs ", result.BufferTab1)
		assert.True(t, result.HasItem("auth"))
		assert.True(t, result.HasItem("billing"))
		assert.True(t, result.HasItem("api"))
		assert.True(t, result.HasItem("frontend"))
	})

	t.Run("positional_prefix_filters_enum", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl logs bi")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl logs billing ", result.BufferTab1)
		assert.True(t, result.HasItem("billing"))
		assert.False(t, result.HasItem("auth"))
	})
}

// TestPowerShellUserVisible_Suite3_FlagsAndModifiers tests user-visible flag completion,
// prefix expansion, and automatic negatable flags in PowerShell.
func TestPowerShellUserVisible_Suite3_FlagsAndModifiers(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("longhand_flag_prefix_lists_flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy --e")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl deploy --env ", result.BufferTab1)
		assert.True(t, result.HasItem("--env"))
	})

	t.Run("shorthand_flag_prefix_lists_flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy -e")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl deploy -e ", result.BufferTab1)
		assert.True(t, result.HasItem("-e"))
	})

	t.Run("flag_list_on_double_dash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy --")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
		assert.True(t, result.HasItem("--dry-run"))
		assert.True(t, result.HasItem("--manifest"))
	})

	t.Run("documented_exception_automatic_negatable_boolean_flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy --no-")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl deploy --no-dry-run ", result.BufferTab1)
		assert.True(t, result.HasItem("--no-dry-run"))
	})
}

// TestPowerShellUserVisible_Suite4_FlagValuesAndContext tests user-visible flag values,
// attached vs separate flags, and context-dependent completion in PowerShell.
func TestPowerShellUserVisible_Suite4_FlagValuesAndContext(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("enum_flag_values", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy --env ")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl deploy --env ", result.BufferTab1)
		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
		assert.True(t, result.HasItem("qa"))
	})

	t.Run("attached_flag_prefix_expansion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy --env=pr")

		assert.Equal(t, "", result.Stderr)
		assert.Equal(t, "", result.StdoutLeak)
		assert.Equal(t, "cloudctl deploy --env=production ", result.BufferTab1)
		assert.True(t, result.HasItem("--env=production"))
	})

	t.Run("context_aware_dynamic_cluster_completion", func(t *testing.T) {
		t.Parallel()

		prodResult := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy --env production --cluster prod-")
		assert.True(t, prodResult.HasItem("prod-us-east-1"))
		assert.True(t, prodResult.HasItem("prod-eu-west-1"))
		assert.False(t, prodResult.HasItem("qa-test-1"))

		qaResult := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy --env qa --cluster qa-test-")
		assert.True(t, qaResult.HasItem("qa-test-1"))
		assert.True(t, qaResult.HasItem("qa-test-2"))
		assert.False(t, qaResult.HasItem("prod-us-east-1"))
	})
}

// TestPowerShellUserVisible_Suite5_ComplexTokensAndMapDeduplication tests user-visible MultiParts,
// delimited slice completion, and smart deduplication exceptions in PowerShell.
func TestPowerShellUserVisible_Suite5_ComplexTokensAndMapDeduplication(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("multiparts_key_and_value_segments", func(t *testing.T) {
		t.Parallel()

		keyResult := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl logs --labels ")
		assert.True(t, keyResult.HasItem("team=") || keyResult.HasItem("team"))
		assert.True(t, keyResult.HasItem("tier=") || keyResult.HasItem("tier"))

		valueResult := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl logs --labels team=")
		assert.True(t, valueResult.HasItem("core") || valueResult.HasItem("team=core"))
		assert.True(t, valueResult.HasItem("frontend") || valueResult.HasItem("team=frontend"))
	})

	t.Run("documented_exception_map_key_deduplication_across_flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl logs --labels team=core --labels ")

		assert.False(t, result.HasItem("team=") || result.HasItem("team"), "team key must be excluded as already assigned")
		assert.True(t, result.HasItem("tier=") || result.HasItem("tier"))
		assert.True(t, result.HasItem("owner=") || result.HasItem("owner"))
	})

	t.Run("documented_exception_map_value_deduplication", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl logs --labels owner=alice --labels owner=")

		assert.False(t, result.HasItem("alice") || result.HasItem("owner=alice"), "alice must be excluded as already assigned")
		assert.True(t, result.HasItem("bob") || result.HasItem("owner=bob"))
		assert.True(t, result.HasItem("carol") || result.HasItem("owner=carol"))
	})

	t.Run("delimited_slice_flags_deduplication", func(t *testing.T) {
		t.Parallel()

		// PowerShell's parser rejects a bare trailing comma (array operator in
		// argument mode), so the user must backtick-escape it; the generated
		// snippet unwraps the escape before invoking __complete
		// (documented host-shell limitation in docs/TESTING.md).
		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy --features canary`,")

		candidateValues := slicex.Map(result.Items, func(item shell.PowerShellItem) string { return item.CompletionText })

		assert.False(t, slices.Contains(candidateValues, "canary,canary"), "canary must not be suggested again")
		assert.True(t, result.HasItem("canary,tracing") || result.HasItem("tracing"))
	})
}

// TestPowerShellUserVisible_Suite6_QuotingAndEscaping tests completion with quotes and escaped characters in PowerShell.
func TestPowerShellUserVisible_Suite6_QuotingAndEscaping(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("single_quoted_positional_prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl logs 'au")

		assert.True(t, result.HasItem("auth"))
	})

	t.Run("double_quoted_positional_prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", `cloudctl logs "bi`)

		assert.True(t, result.HasItem("billing"))
	})

	t.Run("flags_after_quoted_positional_argument", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "cloudctl", "cloudctl deploy 'my-service' --")

		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
	})
}
