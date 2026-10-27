//go:build shelltest

package shell_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// TestZshUserVisible_Suite1_SubcommandsAndBareCommand tests user-visible command line buffer
// states, subcommand discovery, and AutoMenu behavior on root commands.
func TestZshUserVisible_Suite1_SubcommandsAndBareCommand(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("bare_root_command_tab1_leaves_buffer_untouched", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl ")

		// Tab 1: Buffer must strictly remain "cloudctl " with NO --help or flag auto-inserted
		assert.Equal(t, "cloudctl ", result.BufferTab1, "Tab 1 must not auto-insert flags or first candidate on bare root command")

		// Subcommands must be listed
		assert.True(t, result.HasItem("deploy"), "expected deploy subcommand")
		assert.True(t, result.HasItem("logs"), "expected logs subcommand")
		assert.True(t, result.HasItem("completion"), "expected completion subcommand")

		// Tab 2 (AutoMenu): Must cycle to subcommands first, NOT jump to --help
		assert.True(t, strings.HasPrefix(result.BufferTab2, "cloudctl deploy"), "Tab 2 AutoMenu must select subcommands first, got %q", result.BufferTab2)
	})

	t.Run("subcommand_prefix_tab1_expands_buffer", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl dep")

		// Tab 1: Unambiguous prefix expands directly to "cloudctl deploy "
		assert.Equal(t, "cloudctl deploy ", result.BufferTab1)
		assert.Equal(t, len("cloudctl deploy "), result.CursorTab1)
	})

	t.Run("subcommand_alias_resolves", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl d")

		// Short alias 'd' completes
		assert.True(t, result.HasItem("deploy") || result.HasItem("d"))
	})
}

// TestZshUserVisible_Suite2_PositionalArgumentsAndActiveHelp tests user-visible buffer
// mutation and ActiveHelp message display for positional arguments.
func TestZshUserVisible_Suite2_PositionalArgumentsAndActiveHelp(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("unconstrained_positional_renders_active_help_and_preserves_buffer", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy ")

		// Tab 1: Buffer must remain "cloudctl deploy " without auto-inserting '-' or '--cluster'
		assert.Equal(t, "cloudctl deploy ", result.BufferTab1, "Tab 1 on unconstrained positional must not insert dash or flags")

		// ActiveHelp usage hint must be rendered
		assert.True(t, slices.Contains(result.Messages, "<service>: Service name to deploy"), "expected ActiveHelp message in %v", result.Messages)

		// Flags are withheld at a bare space, uniformly with every shell.
		assert.False(t, result.HasItem("--env"), "flags must be suppressed on bare space in zsh")
		assert.False(t, result.HasItem("--cluster"), "flags must be suppressed on bare space in zsh")
	})

	t.Run("positional_enum_tab1_and_tab2_menu_selection", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs ")

		// Tab 1: Ambiguous enum list preserves buffer
		assert.Equal(t, "cloudctl logs ", result.BufferTab1)

		// Enum candidates present
		assert.True(t, result.HasItem("auth"))
		assert.True(t, result.HasItem("billing"))
		assert.True(t, result.HasItem("api"))
		assert.True(t, result.HasItem("frontend"))

		// Tab 2: AutoMenu selects first enum candidate
		assert.Equal(t, "cloudctl logs auth ", result.BufferTab2)
	})

	t.Run("positional_prefix_tab1_expands_unambiguous_value", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs bi")

		// Tab 1: Unambiguous prefix expands directly to "cloudctl logs billing "
		assert.Equal(t, "cloudctl logs billing ", result.BufferTab1)
	})
}

// TestZshUserVisible_Suite3_FlagsAndModifiers tests user-visible flag completion,
// prefix expansion, and automatic negatable flags.
func TestZshUserVisible_Suite3_FlagsAndModifiers(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("longhand_flag_prefix_expands_buffer", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --e")

		// Tab 1: Expands to "cloudctl deploy --env "
		assert.Equal(t, "cloudctl deploy --env ", result.BufferTab1)
	})

	t.Run("shorthand_flag_prefix_expands_buffer", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy -e")

		// Tab 1: Expands shorthand flag
		assert.Equal(t, "cloudctl deploy -e ", result.BufferTab1)
	})

	t.Run("flag_list_on_double_dash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --")

		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
		assert.True(t, result.HasItem("--dry-run"))
		assert.True(t, result.HasItem("--manifest"))
	})

	t.Run("documented_exception_automatic_negatable_boolean_flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --no-")

		// CONGA automatically produces --no-<flag> with "Disable <help>" description
		assert.True(t, result.HasItem("--no-dry-run"))
		desc, ok := result.ItemDescription("--no-dry-run")
		assert.True(t, ok)
		assert.Equal(t, "Disable Simulate deployment", desc)

		// Tab 1 expands unique negatable flag
		assert.Equal(t, "cloudctl deploy --no-dry-run ", result.BufferTab1)
	})
}

// TestZshUserVisible_Suite4_FlagValuesAndContext tests user-visible flag values,
// attached vs separate flags, and context-dependent completion.
func TestZshUserVisible_Suite4_FlagValuesAndContext(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("enum_flag_values_and_active_help", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --env ")

		// Tab 1: Buffer stays clean waiting for value
		assert.Equal(t, "cloudctl deploy --env ", result.BufferTab1)

		// ActiveHelp flag help message displayed
		assert.True(t, slices.Contains(result.Messages, "Target deployment environment"))

		// Enum values listed
		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
		assert.True(t, result.HasItem("qa"))
	})

	t.Run("attached_flag_prefix_expansion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --env=pr")

		// Tab 1: Attached flag expands to "--env=production "
		assert.Equal(t, "cloudctl deploy --env=production ", result.BufferTab1)
	})

	t.Run("context_aware_dynamic_cluster_completion", func(t *testing.T) {
		t.Parallel()

		// Context: --env production
		prodRes := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --env production --cluster ")
		assert.True(t, prodRes.HasItem("prod-us-east-1"))
		assert.True(t, prodRes.HasItem("prod-eu-west-1"))
		assert.False(t, prodRes.HasItem("qa-test-1"))

		// Context: --env qa
		qaRes := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --env qa --cluster ")
		assert.True(t, qaRes.HasItem("qa-test-1"))
		assert.True(t, qaRes.HasItem("qa-test-2"))
		assert.False(t, qaRes.HasItem("prod-us-east-1"))
	})
}

// TestZshUserVisible_Suite5_ComplexTokensAndMapDeduplication tests user-visible MultiParts,
// delimited slice completion, and documented smart deduplication exceptions.
func TestZshUserVisible_Suite5_ComplexTokensAndMapDeduplication(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("multiparts_key_and_value_segments", func(t *testing.T) {
		t.Parallel()

		keyResult := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --labels ")
		assert.True(t, keyResult.HasItem("team="))
		assert.True(t, keyResult.HasItem("tier="))
		assert.True(t, keyResult.HasItem("owner="))

		valueResult := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --labels team=")
		assert.True(t, valueResult.HasItem("core"))
		assert.True(t, valueResult.HasItem("frontend"))
		assert.True(t, valueResult.HasItem("infra"))
	})

	t.Run("documented_exception_map_key_deduplication_across_flags", func(t *testing.T) {
		t.Parallel()

		// When team=core is already configured, re-triggering completion on a second --labels flag excludes team=
		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --labels team=core --labels ")

		assert.False(t, result.HasItem("team="), "team= must be excluded as it is already assigned")
		assert.True(t, result.HasItem("tier="))
		assert.True(t, result.HasItem("owner="))
	})

	t.Run("documented_exception_map_value_deduplication", func(t *testing.T) {
		t.Parallel()

		// When owner=alice is already assigned, typing owner= excludes alice
		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --labels owner=alice --labels owner=")

		assert.False(t, result.HasItem("alice"), "alice must be excluded as it is already assigned")
		assert.True(t, result.HasItem("bob"))
		assert.True(t, result.HasItem("carol"))
	})

	t.Run("delimited_slice_flags_deduplication", func(t *testing.T) {
		t.Parallel()

		// When canary is selected in slice flag, canary, excludes canary from next options
		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --features canary,")

		assert.False(t, result.HasItem("canary,canary"), "canary must not be suggested again")
		assert.True(t, result.HasItem("canary,tracing") || result.HasItem("tracing"))
	})

	t.Run("file_and_directory_filtering", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --manifest ")
		assert.Equal(t, "cloudctl deploy --manifest ", result.BufferTab1)
	})
}

// TestZshUserVisible_Suite6_QuotingAndEscaping tests completion with quotes and escaped characters.
func TestZshUserVisible_Suite6_QuotingAndEscaping(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("single_quoted_positional_prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs 'au")
		assert.True(t, result.HasItem("auth"))
	})

	t.Run("double_quoted_positional_prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs \"bi")
		assert.True(t, result.HasItem("billing"))
	})

	t.Run("flags_after_quoted_positional_argument", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs 'auth' --")
		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--follow"))
	})
}
