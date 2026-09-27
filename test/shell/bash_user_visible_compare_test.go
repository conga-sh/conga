//go:build shelltest

package shell_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// TestBashUserVisible_Suite1_SubcommandsAndBareCommand tests user-visible command line buffer
// states and subcommand discovery on root commands.
func TestBashUserVisible_Suite1_SubcommandsAndBareCommand(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("bare_root_command_lists_subcommands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl ")

		assert.True(t, result.HasItem("deploy"), "expected deploy subcommand")
		assert.True(t, result.HasItem("logs"), "expected logs subcommand")
		assert.True(t, result.HasItem("completion"), "expected completion subcommand")
	})

	t.Run("subcommand_prefix_expands_buffer", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl dep")

		assert.True(t, result.HasItem("deploy"))
		assert.False(t, result.HasItem("logs"))
	})

	t.Run("subcommand_alias_resolves", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl d")

		assert.True(t, result.HasItem("deploy"))
	})
}

// TestBashUserVisible_Suite2_PositionalArguments tests user-visible completion
// for positional arguments in bash.
func TestBashUserVisible_Suite2_PositionalArguments(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("unconstrained_positional_preserves_buffer_without_dash", func(t *testing.T) {
		t.Parallel()

		// Tab 1 (COMP_TYPE=9): Buffer remains clean without cursor displacement or dash insertion
		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy ")
		assert.Equal(t, "cloudctl deploy ", result.BufferTab1, "Tab 1 on unconstrained positional must not insert dash")
		assert.False(t, result.HasItem("--env"), "flags must be suppressed on bare space in bash")
		assert.Equal(t, "", result.StdoutLeak, "completion function must not emit raw stdout/newlines in bash")
	})

	t.Run("unconstrained_positional_flag_discovery_with_dash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy -")

		// Typing '-' allows flag discovery
		assert.True(t, result.HasItem("--env") || result.HasItem("-e"))
		assert.True(t, result.HasItem("--cluster") || result.HasItem("-c"))
		assert.Equal(t, "", result.StdoutLeak, "completion function must not emit raw stdout/newlines in bash")
	})

	t.Run("positional_enum_completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs ")

		// Tab 1: Buffer preserved
		assert.Equal(t, "cloudctl logs ", result.BufferTab1)

		assert.True(t, result.HasItem("auth"))
		assert.True(t, result.HasItem("billing"))
		assert.True(t, result.HasItem("api"))
		assert.True(t, result.HasItem("frontend"))
	})

	t.Run("positional_prefix_filters_enum", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs bi")

		assert.True(t, result.HasItem("billing"))
		assert.False(t, result.HasItem("auth"))
	})
}

// TestBashUserVisible_Suite3_FlagsAndModifiers tests user-visible flag completion,
// prefix expansion, and automatic negatable flags in bash.
func TestBashUserVisible_Suite3_FlagsAndModifiers(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("longhand_flag_prefix_lists_flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --e")

		assert.True(t, result.HasItem("--env"))
	})

	t.Run("shorthand_flag_prefix_lists_flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy -e")

		assert.True(t, result.HasItem("-e"))
	})

	t.Run("flag_list_on_double_dash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --")

		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
		assert.True(t, result.HasItem("--dry-run"))
		assert.True(t, result.HasItem("--manifest"))
	})

	t.Run("documented_exception_automatic_negatable_boolean_flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --no-")

		// CONGA automatically produces --no-<flag> with "Disable <help>" description
		assert.True(t, result.HasItem("--no-dry-run"))
	})
}

// TestBashUserVisible_Suite4_FlagValuesAndContext tests user-visible flag values,
// attached vs separate flags, and context-dependent completion.
func TestBashUserVisible_Suite4_FlagValuesAndContext(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("enum_flag_values", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env ")

		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
		assert.True(t, result.HasItem("qa"))
	})

	t.Run("attached_flag_prefix_expansion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env=pr")

		assert.True(t, result.HasItem("production"))
	})

	t.Run("context_aware_dynamic_cluster_completion", func(t *testing.T) {
		t.Parallel()

		prodRes := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env production --cluster prod-")
		assert.True(t, prodRes.HasItem("prod-us-east-1"))
		assert.True(t, prodRes.HasItem("prod-eu-west-1"))
		assert.False(t, prodRes.HasItem("qa-test-1"))

		qaRes := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env qa --cluster qa-test-")
		assert.True(t, qaRes.HasItem("qa-test-1"))
		assert.True(t, qaRes.HasItem("qa-test-2"))
		assert.False(t, qaRes.HasItem("prod-us-east-1"))
	})
}

// TestBashUserVisible_Suite5_ComplexTokensAndMapDeduplication tests user-visible MultiParts,
// delimited slice completion, and documented smart deduplication exceptions.
func TestBashUserVisible_Suite5_ComplexTokensAndMapDeduplication(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("multiparts_key_and_value_segments", func(t *testing.T) {
		t.Parallel()

		keyResult := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --labels ")
		assert.True(t, keyResult.HasItem("team=") || keyResult.HasItem(`"team="`))
		assert.True(t, keyResult.HasItem("tier=") || keyResult.HasItem(`"tier="`))

		valueResult := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --labels team=")
		assert.True(t, valueResult.HasItem("core") || valueResult.HasItem(`"core"`))
		assert.True(t, valueResult.HasItem("frontend") || valueResult.HasItem(`"frontend"`))
	})

	t.Run("documented_exception_map_key_deduplication_across_flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --labels team=core --labels ")

		// team has already been specified in prior flag invocation
		assert.False(t, result.HasItem("team=") || result.HasItem(`"team="`), "team key must be excluded")
		assert.True(t, result.HasItem("tier=") || result.HasItem(`"tier="`))
		assert.True(t, result.HasItem("owner=") || result.HasItem(`"owner="`))
	})

	t.Run("documented_exception_map_value_deduplication", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --labels owner=alice --labels owner=")

		// The bash driver strips the repeated prefix because of '=' wordbreak handling.
		assert.False(t, result.HasItem("alice") || result.HasItem(`"owner=alice,owner=alice"`) || result.HasItem("owner=alice,owner=alice"), "alice must be excluded as it is already assigned")

		for _, expected := range []string{"bob", "carol"} {
			found := false

			for _, item := range result.Items {
				if item == expected || item == `"`+expected+`"` || strings.HasSuffix(item, expected) {
					found = true

					break
				}
			}

			assert.True(t, found, "expected value %s in bash completion output %v", expected, result.Items)
		}
	})

	t.Run("delimited_slice_flags_deduplication", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --features canary,")

		assert.False(t, slices.Contains(result.Items, "canary,canary"), "canary must not be suggested again")
		assert.True(t, result.HasItem("canary,tracing") || result.HasItem("tracing"))
	})
}

// TestBashUserVisible_Suite6_QuotingAndEscaping tests completion with quotes and escaped characters.
func TestBashUserVisible_Suite6_QuotingAndEscaping(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("single_quoted_positional_prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs 'au")
		assert.True(t, result.HasItem("auth"))
	})

	t.Run("double_quoted_positional_prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs \"bi")
		assert.True(t, result.HasItem("billing"))
	})

	t.Run("flags_after_quoted_positional_argument", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy 'my-service' --")
		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
	})
}

// TestBashUserVisible_Suite7_SuccessiveTabsAndDescriptions tests Bash-specific successive tab
// behaviour and description display.
func TestBashUserVisible_Suite7_SuccessiveTabsAndDescriptions(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("successive_tabs_show_descriptions", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletionType(t, "cloudctl", "cloudctl ", "63")

		hasDeployWithDesc := false

		for _, item := range result.Items {
			if strings.HasPrefix(item, "deploy") && strings.Contains(item, "Deploy service") {
				hasDeployWithDesc = true

				break
			}
		}

		assert.True(t, hasDeployWithDesc, "expected deploy candidate with description in successive tab mode (COMP_TYPE=63), got %v", result.Items)
	})
}

// TestBashUserVisible_Suite8_RedirectTarget tests the bash redirect-rejection
// contract (docs/completion-wire-protocol.md §4.1): completing at a
// redirection target yields zero candidates and leaves the command line
// buffer and the terminal untouched. Redirect rejection is a bash driver
// mechanism (the COMP_LINE re-lex in the Patch hook); the other shells manage
// their own insertion and are intentionally not mirrored here.
func TestBashUserVisible_Suite8_RedirectTarget(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("redirect_target_yields_zero_candidates", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl > ")

		assert.Zero(t, result.Items, "subcommand candidates must not leak at a redirect target")
		assert.False(t, result.HasItem("deploy"))
		assert.False(t, result.HasItem("logs"))
		assert.Equal(t, "cloudctl > ", result.BufferTab1, "Tab on a redirect target must not mutate the command line buffer")
		assert.Equal(t, "", result.StdoutLeak, "completion function must not emit raw stdout/newlines in bash")
	})
}
