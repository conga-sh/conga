//go:build shelltest

package shell_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestZsh_Subcommands(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl ")

	assert.True(t, result.HasItem("deploy"), "expected deploy subcommand")
	assert.True(t, result.HasItem("logs"), "expected logs subcommand")
	assert.True(t, result.HasItem("completion"), "expected completion subcommand")

	desc, ok := result.ItemDescription("deploy")
	assert.True(t, ok)
	assert.Equal(t, "Deploy service to a cluster", desc)

	// Verify group tag is commands
	assert.True(t, len(result.GroupItems("commands")) > 0, "expected commands group to contain candidates")
}

func TestZsh_SubcommandPrefix(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl d")

	assert.True(t, result.HasItem("deploy"))
	assert.False(t, result.HasItem("logs"))
}

func TestZsh_Flags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --")

	assert.True(t, result.HasItem("--env"))
	assert.True(t, result.HasItem("--region"))
	assert.True(t, result.HasItem("--cluster"))
	assert.True(t, result.HasItem("--image"))
	assert.True(t, result.HasItem("--features"))
	assert.True(t, result.HasItem("--manifest"))
	assert.True(t, result.HasItem("--out-dir"))
	assert.True(t, result.HasItem("--dry-run"))
	assert.True(t, result.HasItem("--no-dry-run"))
	assert.True(t, result.HasItem("--profile"))
	assert.True(t, result.HasItem("--kubeconfig"))
	assert.True(t, result.HasItem("--help"))
	assert.False(t, result.HasItem("--version"))

	// Verify group tag is flags
	assert.True(t, len(result.GroupItems("flags")) > 0, "expected flags group to contain candidates")

	// Ensure files were not called by fallback
	assert.Zero(t, result.FilesCalled, "expected no file completions for --")

	t.Run("logs flags", func(t *testing.T) {
		t.Parallel()

		logsResult := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --")

		assert.True(t, logsResult.HasItem("--labels"))
	})
}

func TestZsh_NegatableFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --no-")

	assert.True(t, result.HasItem("--no-dry-run"))

	desc, ok := result.ItemDescription("--no-dry-run")
	assert.True(t, ok)
	assert.Equal(t, "Disable Simulate deployment", desc)
}

func TestZsh_AttachedFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --env=")

	assert.True(t, result.HasItem("staging") || result.HasItem("--env=staging"))
	assert.True(t, result.HasItem("production") || result.HasItem("--env=production"))
	assert.True(t, result.HasItem("qa") || result.HasItem("--env=qa"))
}

func TestZsh_SeparateFlagValue(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --env ")

	assert.True(t, result.HasItem("staging"))
	assert.True(t, result.HasItem("production"))
	assert.True(t, result.HasItem("qa"))
	assert.False(t, result.HasItem("--"), "must not contain trailing delimiter -- in candidates")
}

func TestZsh_ContextAwarePredictors(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("production clusters", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --env production --cluster ")

		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.True(t, result.HasItem("prod-ap-southeast-1"))
		assert.False(t, result.HasItem("qa-test-1"))
		assert.False(t, result.HasItem("staging-cluster-alpha"))
	})

	t.Run("qa clusters", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --env qa --cluster ")

		assert.True(t, result.HasItem("qa-test-1"))
		assert.True(t, result.HasItem("qa-test-2"))
		assert.False(t, result.HasItem("prod-us-east-1"))
	})
}

func TestZsh_FileExtensionFiltering(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	// Create files and directories in workspace
	workspace.WriteFile(t, "app.yaml", []byte("name: app"))
	workspace.WriteFile(t, "config.yml", []byte("port: 8080"))
	workspace.WriteFile(t, "readme.md", []byte("# Readme"))
	workspace.WriteFile(t, "data.json", []byte("{}"))
	workspace.Mkdir(t, "nested")
	workspace.Mkdir(t, "nested/child")
	workspace.WriteFile(t, "nested/manifest.yaml", []byte("kind: Pod"))
	workspace.WriteFile(t, "nested/ignore.txt", []byte("ignore"))

	t.Run("attached flag completion in root dir", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --manifest=")

		assert.True(t, result.HasItem("app.yaml") || result.HasItem("--manifest=app.yaml"), "expected app.yaml candidate")
		assert.True(t, result.HasItem("config.yml") || result.HasItem("--manifest=config.yml"), "expected config.yml candidate")
		assert.True(t, result.HasItem("nested/") || result.HasItem("--manifest=nested/"), "expected nested/ directory candidate")
		assert.False(t, result.HasItem("readme.md") || result.HasItem("--manifest=readme.md"), "did not expect readme.md candidate")
		assert.False(t, result.HasItem("data.json") || result.HasItem("--manifest=data.json"), "did not expect data.json candidate")
		assert.False(t, result.HasItem("*.yaml") || result.HasItem("--manifest=*.yaml"), "did not expect literal glob *.yaml")
		assert.False(t, result.HasItem("*.yml") || result.HasItem("--manifest=*.yml"), "did not expect literal glob *.yml")
	})

	t.Run("separate flag completion in root dir", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --manifest ")

		assert.True(t, result.HasItem("app.yaml"), "expected app.yaml candidate")
		assert.True(t, result.HasItem("config.yml"), "expected config.yml candidate")
		assert.True(t, result.HasItem("nested/"), "expected nested/ directory candidate")
		assert.False(t, result.HasItem("readme.md"), "did not expect readme.md candidate")
		assert.False(t, result.HasItem("data.json"), "did not expect data.json candidate")
		assert.False(t, result.HasItem("*.yaml"), "did not expect literal glob *.yaml")
		assert.False(t, result.HasItem("*.yml"), "did not expect literal glob *.yml")
	})

	t.Run("nested directory traversal with separate flag", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --manifest nested/")

		assert.True(t, result.HasItem("manifest.yaml") || result.HasItem("nested/manifest.yaml"), "expected manifest.yaml candidate")
		assert.True(t, result.HasItem("child/") || result.HasItem("nested/child/"), "expected child/ directory candidate")
		assert.False(t, result.HasItem("ignore.txt") || result.HasItem("nested/ignore.txt"), "did not expect ignore.txt candidate")
		assert.False(t, result.HasItem("*.yaml"), "did not expect literal glob *.yaml")
	})

	t.Run("nested directory traversal with attached flag", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --manifest=nested/")

		assert.True(t, result.HasItem("manifest.yaml") || result.HasItem("nested/manifest.yaml") || result.HasItem("--manifest=nested/manifest.yaml"), "expected manifest.yaml candidate")
		assert.True(t, result.HasItem("child/") || result.HasItem("nested/child/") || result.HasItem("--manifest=nested/child/"), "expected child/ candidate")
		assert.False(t, result.HasItem("ignore.txt") || result.HasItem("nested/ignore.txt"), "did not expect ignore.txt candidate")
		assert.False(t, result.HasItem("*.yaml"), "did not expect literal glob *.yaml")
	})
}

func TestZsh_DirectoryFiltering(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	workspace.Mkdir(t, "build")
	workspace.Mkdir(t, "build/sub")
	workspace.Mkdir(t, "dist")
	workspace.WriteFile(t, "app.go", []byte("package main"))

	t.Run("attached flag directory filtering", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --out-dir=")

		assert.True(t, result.HasItem("build/") || result.HasItem("--out-dir=build/"), "expected build/ candidate")
		assert.True(t, result.HasItem("dist/") || result.HasItem("--out-dir=dist/"), "expected dist/ candidate")
		assert.False(t, result.HasItem("app.go") || result.HasItem("--out-dir=app.go"), "did not expect app.go candidate")
	})

	t.Run("separate flag directory filtering", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --out-dir ")

		assert.True(t, result.HasItem("build/"), "expected build/ candidate")
		assert.True(t, result.HasItem("dist/"), "expected dist/ candidate")
		assert.False(t, result.HasItem("app.go"), "did not expect app.go candidate")
	})

	t.Run("nested directory traversal", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --out-dir build/")

		assert.True(t, result.HasItem("sub/") || result.HasItem("build/sub/"), "expected sub/ candidate")
		assert.False(t, result.HasItem("app.go"), "did not expect app.go candidate")
	})
}

func TestZsh_MultiParts(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("top-level tag keys", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --labels ")

		assert.True(t, result.HasItem("team") || result.HasItem("team="))
		assert.True(t, result.HasItem("tier") || result.HasItem("tier="))
		assert.True(t, result.HasItem("owner") || result.HasItem("owner="))
	})

	t.Run("nested tag values", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --labels team=")

		assert.True(t, result.HasItem("core") || result.HasItem("team=core"))
		assert.True(t, result.HasItem("frontend") || result.HasItem("team=frontend"))
		assert.True(t, result.HasItem("infra") || result.HasItem("team=infra"))
	})
}

func TestZsh_PositionalEnumAndFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs ")

	// Positional arguments
	assert.True(t, result.HasItem("auth"))
	assert.True(t, result.HasItem("billing"))
	assert.True(t, result.HasItem("api"))
	assert.True(t, result.HasItem("frontend"))

	// Grouped flags
	assert.True(t, result.HasItem("--env"))
	assert.True(t, result.HasItem("--follow"))
	assert.True(t, result.HasItem("--help"))

	// Verify both groups exist
	assert.True(t, len(result.GroupItems("arguments")) > 0, "expected arguments group")
	assert.True(t, len(result.GroupItems("flags")) > 0, "expected flags group")
}

func TestZsh_UnconstrainedPositionalAndFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy ")

	// Verify active help hint message
	assert.True(t, slices.Contains(result.Messages, "<service>: Service name to deploy"), "expected active help message for service argument")

	// Verify flags are discoverable
	assert.True(t, result.HasItem("--env"))
	assert.True(t, result.HasItem("--region"))
	assert.True(t, result.HasItem("--cluster"))
	assert.True(t, result.HasItem("--manifest"))
	assert.True(t, result.HasItem("--out-dir"))
	assert.True(t, result.HasItem("--dry-run"))
	assert.True(t, result.HasItem("--help"))
	assert.True(t, len(result.GroupItems("flags")) > 0, "expected flags group")
}

func TestZsh_ShortFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("single dash short and long flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy -")

		assert.True(t, result.HasItem("-e"), "expected -e short flag")
		assert.True(t, result.HasItem("-c"), "expected -c short flag")
		assert.True(t, result.HasItem("--env"), "expected --env long flag")
		assert.True(t, result.HasItem("--cluster"), "expected --cluster long flag")
	})

	t.Run("attached short flag", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy -e=")

		assert.True(t, result.HasItem("staging") || result.HasItem("-e=staging"))
		assert.True(t, result.HasItem("production") || result.HasItem("-e=production"))
		assert.True(t, result.HasItem("qa") || result.HasItem("-e=qa"))
	})

	t.Run("separate short flag", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy -e ")

		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
		assert.True(t, result.HasItem("qa"))
	})

	t.Run("boolean short flag followed by flag value", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs -f -e ")

		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
		assert.True(t, result.HasItem("qa"))
	})
}

func TestZsh_SubcommandAlias(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl d ")

	// Verify alias "d" completes flags and arguments for DeployCmd
	assert.True(t, result.HasItem("--env"))
	assert.True(t, result.HasItem("--cluster"))
	assert.True(t, slices.Contains(result.Messages, "<service>: Service name to deploy"), "expected active help message on alias")
}

func TestZsh_NestedSubcommands(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("completion shell candidates", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl completion ")

		assert.True(t, result.HasItem("zsh"))
		assert.True(t, result.HasItem("bash"))
		assert.True(t, result.HasItem("fish"))
		assert.True(t, result.HasItem("powershell"))
		assert.True(t, result.HasItem("elvish"))
		assert.True(t, result.HasItem("nushell"))
	})

	t.Run("completion prefix matching", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl completion z")

		assert.True(t, result.HasItem("zsh"))
		assert.False(t, result.HasItem("bash"))
	})

	t.Run("help subcommand active help", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl help ")

		assert.True(t, slices.Contains(result.Messages, "<[<command>...]>: Subcommand to show help for"), "expected help command active help message")
		assert.True(t, result.HasItem("--help"))
		assert.False(t, result.HasItem("--version"))
	})
}

func TestZsh_QuotedArguments(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("single quoted prefix candidate", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs 'au")

		assert.True(t, result.HasItem("auth"))
		assert.False(t, result.HasItem("billing"))
	})

	t.Run("double quoted prefix candidate", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", `cloudctl logs "bi`)

		assert.True(t, result.HasItem("billing"))
		assert.False(t, result.HasItem("auth"))
	})

	t.Run("flags after single-quoted positional arg", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy 'my-service' --")

		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
		assert.True(t, result.HasItem("--manifest"))
	})

	t.Run("flags after double-quoted positional arg", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", `cloudctl deploy "my-service" --`)

		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
		assert.True(t, result.HasItem("--manifest"))
	})
}

func TestZsh_MenuSelectAndGroupHeadings(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	setupScript := `
zmodload -i zsh/complist
zstyle ':completion:*' menu select
zstyle ':completion:*' group-name ''
zstyle ':completion:*:descriptions' format '=== %d ==='
`

	result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl logs ", setupScript)

	assert.True(t, result.HasItem("auth"))
	assert.True(t, result.HasItem("billing"))
	assert.True(t, result.HasItem("--env"))
	assert.True(t, result.HasItem("--follow"))

	assert.True(t, len(result.GroupItems("arguments")) > 0, "expected arguments group")
	assert.True(t, len(result.GroupItems("flags")) > 0, "expected flags group")
}

func TestZsh_ListColorsAndANSIStyling(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl ")

	assert.True(t, result.HasItem("deploy"))
	assert.True(t, result.HasItem("logs"))

	// Verify list-colors zstyle was configured
	assert.True(t, strings.Contains(result.ListColors, "=(#b)"), "expected list-colors pattern in zstyle output")
	assert.True(t, strings.Contains(result.ListColors, "deploy"), "expected deploy in list-colors")
}

func TestZsh_AliasAndDescriptionSidebands(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("aliases shown by default", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl ")

		assert.True(t, result.HasItem("deploy"))
		assert.True(t, result.HasItem("d"))
	})

	t.Run("aliases suppressed", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl ", "CONGA_COMPLETION_ALIASES=0")

		assert.True(t, result.HasItem("deploy"))
		assert.False(t, result.HasItem("d"))
	})

	t.Run("descriptions suppressed", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl ", "CONGA_COMPLETION_DESCRIPTIONS=0")

		description, ok := result.ItemDescription("deploy")
		assert.True(t, ok)
		assert.Equal(t, "", description)
	})

	t.Run("show-aliases zstyle suppresses aliases", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl ", `zstyle ':completion:*' show-aliases false`)

		assert.True(t, result.HasItem("deploy"))
		assert.False(t, result.HasItem("d"))
	})

	t.Run("verbose zstyle suppresses descriptions", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl ", `zstyle ':completion:*' verbose false`)

		description, ok := result.ItemDescription("deploy")
		assert.True(t, ok)
		assert.Equal(t, "", description)
	})
}

func TestZsh_ListSeparatorSideband(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("default separator patterns", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl ")

		assert.Contains(t, result.ListColors, "([ ]## -- *)", "expected the default separator patterns")
	})

	t.Run("list-separator zstyle drives list-colors patterns", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl ", `zstyle ':completion:*' list-separator '<->'`)

		assert.Contains(t, result.ListColors, `([ ]## \<-\> *)`, "expected the user separator in list-colors patterns")
		assert.NotContains(t, result.ListColors, "([ ]## -- *)", "expected the default separator to be replaced")
	})
}

func TestZsh_UserZstylesSurvive(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("group-name", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl deploy ", `zstyle ':completion:*' group-name '%d'`)

		assert.True(t, result.GroupNameSet, "expected group-name to be set")
		assert.Equal(t, "%d", result.GroupName)
	})

	t.Run("list-colors", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl deploy ", `zstyle ':completion:*' list-colors 'ln=target:di=01;34'`)

		assert.Equal(t, "ln=target:di=01;34", result.ListColors)
	})

	t.Run("empty list-colors keeps engine colors", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl deploy ", `zstyle ':completion:*' list-colors ''`)

		assert.True(t, strings.Contains(result.ListColors, "=(#b)"), "expected engine list-colors when the user value is empty")
	})

	t.Run("engine default group-name without user config", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy ")

		assert.True(t, result.GroupNameSet, "expected the engine default group-name to be set")
		assert.Equal(t, "", result.GroupName)
	})
}

func TestZsh_FrameworkCompatibility_OhMyZsh(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	omzSetup := `
zstyle ':completion:*' completer _expand _complete _ignored _approximate
zstyle ':completion:*' matcher-list 'm:{a-zA-Z}={A-Za-z}' 'r:|[._-]=* r:|=*' 'l:|=* r:|=*'
zstyle ':completion:*' list-colors ''
zstyle ':completion:*' menu select=2
zstyle ':completion:*:*:*:*:descriptions' format '%F{blue}-- %d --%f'
zstyle ':completion:*:*:*:*:corrections' format '%F{yellow}!- %d (errors: %e) -!%f'
zstyle ':completion:*:*:*:*:messages' format '%F{purple} -- %d --%f'
zstyle ':completion:*:*:*:*:warnings' format '%F{red}-- no matches found --%f'
zstyle ':completion:*' group-name ''
zstyle ':completion:*' verbose yes
`

	t.Run("subcommands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl ", omzSetup)
		assert.True(t, result.HasItem("deploy"))
		assert.True(t, result.HasItem("logs"))
		assert.True(t, result.HasItem("completion"))
	})

	t.Run("flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl deploy --", omzSetup)
		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
		assert.True(t, result.HasItem("--dry-run"))
	})

	t.Run("dynamic context predictors", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl deploy --env production --cluster ", omzSetup)
		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.False(t, result.HasItem("staging-cluster-alpha"))
	})
}

func TestZsh_FrameworkCompatibility_Prezto(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	preztoSetup := `
zstyle ':completion:*:*:*:*:*' menu select
zstyle ':completion:*' matcher-list 'm:{a-zA-Z}={A-Za-z}' 'r:|[._-]=* r:|=*' 'l:|=* r:|=*'
zstyle ':completion:*' completer _complete _match _approximate
zstyle ':completion:*:match:*' original only
zstyle ':completion:*:approximate:*' max-errors 1 numeric
zstyle ':completion:*:*:*:*:descriptions' format '%B%d%b'
`

	result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl deploy --env ", preztoSetup)
	assert.True(t, result.HasItem("staging"))
	assert.True(t, result.HasItem("production"))
	assert.True(t, result.HasItem("qa"))
}

func TestZsh_FrameworkCompatibility_FzfTab(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	fzfTabSetup := `
# Emulate fzf-tab hook by wrapping _describe
functions[_fzf_orig_describe]=$functions[_describe]
_describe() {
    _fzf_orig_describe "$@"
}
`

	result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl logs ", fzfTabSetup)
	assert.True(t, result.HasItem("auth"))
	assert.True(t, result.HasItem("billing"))
	assert.True(t, result.HasItem("--env"))
}

func TestZsh_FrameworkCompatibility_ZshAutocomplete(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	zshAutoSetup := `
zstyle ':autocomplete:*' min-input 0
zstyle ':autocomplete:*' insert-unambiguous yes
zstyle ':completion:*' menu yes select
`

	result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl deploy --", zshAutoSetup)
	assert.True(t, result.HasItem("--env"))
	assert.True(t, result.HasItem("--cluster"))
}

func TestZsh_ShellAliases(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("single letter alias", func(t *testing.T) {
		t.Parallel()

		aliasSetup := `
alias c=cloudctl
compdef _cloudctl_completion c
`

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "c deploy --env ", aliasSetup)
		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
		assert.True(t, result.HasItem("qa"))
	})

	t.Run("subcommand alias with complete_aliases", func(t *testing.T) {
		t.Parallel()

		aliasSetup := `
setopt complete_aliases
alias c=cloudctl
compdef _cloudctl_completion c
`

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "c logs ", aliasSetup)
		assert.True(t, result.HasItem("auth"))
		assert.True(t, result.HasItem("billing"))
		assert.True(t, result.HasItem("--env"))
	})

	t.Run("aliased invocation with flags", func(t *testing.T) {
		t.Parallel()

		aliasSetup := `
alias c=cloudctl
compdef _cloudctl_completion c
`

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "c deploy --env production --cluster ", aliasSetup)
		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.False(t, result.HasItem("staging-cluster-alpha"))
	})
}

func TestZsh_ShellOptions(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("glob options", func(t *testing.T) {
		t.Parallel()

		optSetup := `
setopt glob_complete
setopt extended_glob
setopt nullglob
setopt no_case_glob
`

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl deploy --env ", optSetup)
		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
	})

	t.Run("word and menu options", func(t *testing.T) {
		t.Parallel()

		optSetup := `
setopt complete_in_word
setopt always_to_end
setopt auto_menu
setopt bash_autolist
`

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl logs --labels team=", optSetup)
		assert.True(t, result.HasItem("core") || result.HasItem("team=core"))
		assert.True(t, result.HasItem("frontend") || result.HasItem("team=frontend"))
	})

	t.Run("compatibility options", func(t *testing.T) {
		t.Parallel()

		optSetup := `
setopt ksh_glob
setopt sh_word_split
`

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl logs ", optSetup)
		assert.True(t, result.HasItem("auth"))
		assert.True(t, result.HasItem("--follow"))
	})
}

func TestZsh_CustomCompleterChains(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("approximate chain", func(t *testing.T) {
		t.Parallel()

		chainSetup := `
zstyle ':completion:*' completer _complete _ignored _approximate
`

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl deploy --env ", chainSetup)
		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
	})

	t.Run("expand chain", func(t *testing.T) {
		t.Parallel()

		chainSetup := `
zstyle ':completion:*' completer _oldlist _expand _complete
`

		result := workspace.RunZshCompletionWithSetup(t, "cloudctl", "cloudctl completion ", chainSetup)
		assert.True(t, result.HasItem("zsh"))
		assert.True(t, result.HasItem("bash"))
		assert.True(t, result.HasItem("fish"))
	})
}

func TestZsh_PureFlagsCommandWithRequiredFlag(t *testing.T) {
	t.Parallel()

	workspace := setupFlagConstraints(t)

	result := workspace.RunZshCompletion(t, "06-flag-constraints", "06-flag-constraints --")

	assert.True(t, result.HasItem("--api-key"), "expected required flag --api-key")
	assert.True(t, result.HasItem("--json"))
	assert.True(t, result.HasItem("--yaml"))
	assert.True(t, result.HasItem("--table"))
	assert.True(t, result.HasItem("--username"))
	assert.True(t, result.HasItem("--password"))
	assert.True(t, result.HasItem("--port"))
	assert.True(t, result.HasItem("--env"))
	assert.True(t, result.HasItem("--zones"))
	assert.True(t, result.HasItem("--help"))
	assert.Equal(t, "06-flag-constraints --", result.BufferTab1)
}

func TestZsh_DomainTypes_Completion(t *testing.T) {
	t.Parallel()

	workspace := setupDomainTypes(t)

	// Bare space: completion subcommand must NOT appear, flags must appear
	bareRes := workspace.RunZshCompletion(t, "05-domain-types", "05-domain-types ")
	assert.False(t, bareRes.HasItem("completion"))
	assert.True(t, bareRes.HasItem("--input-file"))
	assert.True(t, bareRes.HasItem("--completion"))
	assert.True(t, bareRes.HasItem("-i"))

	// Typing '--comp': must complete --completion flag
	compRes := workspace.RunZshCompletion(t, "05-domain-types", "05-domain-types --comp")
	assert.True(t, compRes.HasItem("--completion"), "expected --completion candidate")
	assert.Equal(t, "05-domain-types --completion ", compRes.BufferTab1)

	// Typing '--completion z': must complete the zsh shell value
	valueRes := workspace.RunZshCompletion(t, "05-domain-types", "05-domain-types --completion z")
	assert.True(t, valueRes.HasItem("zsh"), "expected zsh shell candidate")
	assert.Equal(t, "05-domain-types --completion zsh ", valueRes.BufferTab1)

	// Typing '--completion ': must list every supported shell
	listRes := workspace.RunZshCompletion(t, "05-domain-types", "05-domain-types --completion ")
	assert.True(t, listRes.HasItem("bash"), "expected bash shell candidate")
	assert.True(t, listRes.HasItem("zsh"), "expected zsh shell candidate")
	assert.True(t, listRes.HasItem("fish"), "expected fish shell candidate")
	assert.True(t, listRes.HasItem("powershell"), "expected powershell shell candidate")
}

func TestZsh_DeprecatedFiltering(t *testing.T) {
	t.Parallel()

	workspace := setupMemberDeprecated(t)

	t.Run("deprecated flag excluded from flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "member-deprecated", "member-deprecated --")

		assert.True(t, result.HasItem("--modern"))
		assert.False(t, result.HasItem("--old"))

		prefixRes := workspace.RunZshCompletion(t, "member-deprecated", "member-deprecated --o")

		assert.False(t, prefixRes.HasItem("--old"))
		assert.Equal(t, "member-deprecated --o", prefixRes.BufferTab1)
	})

	t.Run("deprecated subcommand excluded from commands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "member-deprecated", "member-deprecated ")

		assert.True(t, result.HasItem("deploy"))
		assert.False(t, result.HasItem("legacy"))

		prefixRes := workspace.RunZshCompletion(t, "member-deprecated", "member-deprecated l")

		assert.False(t, prefixRes.HasItem("legacy"))
		assert.Equal(t, "member-deprecated l", prefixRes.BufferTab1)
	})
}

func TestZsh_AppDeprecatedCompletion(t *testing.T) {
	t.Parallel()

	workspace := setupAppDeprecated(t)

	result := workspace.RunZshCompletion(t, "app-deprecated", "app-deprecated ")

	assert.True(t, result.HasItem("deploy"))
}

func TestZsh_XORFlagSuppression(t *testing.T) {
	t.Parallel()

	workspace := setupFlagConstraints(t)

	t.Run("json suppresses yaml and table", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "06-flag-constraints", "06-flag-constraints --json --")

		assert.False(t, result.HasItem("--yaml"))
		assert.False(t, result.HasItem("--table"))
		assert.True(t, result.HasItem("--api-key"))
		assert.True(t, result.HasItem("--port"))
	})

	t.Run("yaml suppresses json and table", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "06-flag-constraints", "06-flag-constraints --yaml --")

		assert.False(t, result.HasItem("--json"))
		assert.False(t, result.HasItem("--table"))
		assert.True(t, result.HasItem("--api-key"))
		assert.True(t, result.HasItem("--port"))
	})
}

func TestZsh_NonRepeatableFlagSuppression(t *testing.T) {
	t.Parallel()

	workspace := setupFlagConstraints(t)

	t.Run("scalar flag is suppressed after consumption", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "06-flag-constraints", "06-flag-constraints --port 8080 --")

		assert.False(t, result.HasItem("--port"))
		assert.True(t, result.HasItem("--api-key"))
		assert.True(t, result.HasItem("--zones"))
	})

	t.Run("slice flag remains available after consumption", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "06-flag-constraints", "06-flag-constraints --zones us-east-1a --")

		assert.True(t, result.HasItem("--zones"))
		assert.True(t, result.HasItem("--port"))
	})
}

func TestZsh_RootFlagInheritanceAndPreSubcommandPlacement(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("inherited root flag completion after subcommand", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --pro")

		assert.True(t, result.HasItem("--profile"))
	})

	t.Run("pre-subcommand root flag affects subcommand predictor", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl --profile production exec ")

		assert.True(t, result.HasItem("api-prod-8a"))
		assert.True(t, result.HasItem("billing-prod-3b"))
		assert.True(t, result.HasItem("auth-prod-7c"))
		assert.False(t, result.HasItem("api-stage-alpha"))
	})
}

func TestZsh_ContextualPositionalPredictor(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("default profile and namespace pods", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl exec ")

		assert.True(t, result.HasItem("api-stage-alpha"))
		assert.True(t, result.HasItem("auth-stage-beta"))
		assert.False(t, result.HasItem("api-prod-8a"))
	})

	t.Run("database namespace pods without common prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl exec --namespace database ")

		assert.True(t, result.HasItem("pg-stage-0"))
		assert.True(t, result.HasItem("redis-stage-0"))
		assert.False(t, result.HasItem("api-stage-alpha"))
	})
}

func TestZsh_TerminatorDashArgs(t *testing.T) {
	t.Parallel()

	workspace := setupTrailingArgs(t)

	t.Run("flags suppressed after double dash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "runner", "runner test mypkg -- --")

		assert.False(t, result.HasItem("--verbose"))
		assert.False(t, result.HasItem("--help"))
	})
}

func TestZsh_HiddenFlags(t *testing.T) {
	t.Parallel()

	workspace := setupFlagPrimitives(t)

	result := workspace.RunZshCompletion(t, "02-flag-primitives", "02-flag-primitives --")

	assert.True(t, result.HasItem("--verbosity"))
	assert.True(t, result.HasItem("--debug"))
	assert.True(t, result.HasItem("--port"))
	assert.False(t, result.HasItem("--secret-key"), "hidden flag must not appear in completion candidates")
}

func TestZsh_DomainFilesystemTypes(t *testing.T) {
	t.Parallel()

	workspace := setupDomainTypes(t)

	err := os.WriteFile(filepath.Join(workspace.WorkDir, "sample.txt"), []byte("data"), 0o600)
	assert.NoError(t, err)

	err = os.Mkdir(filepath.Join(workspace.WorkDir, "output_dir"), 0o700)
	assert.NoError(t, err)

	t.Run("input file completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "05-domain-types", "05-domain-types --input-file ")

		assert.True(t, result.HasItem("sample.txt"))
	})

	t.Run("output dir completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "05-domain-types", "05-domain-types --output-dir ")

		assert.True(t, result.HasItem("output_dir/") || result.HasItem("output_dir"))
		assert.False(t, result.HasItem("sample.txt"))
	})
}

func TestZsh_Sidebands(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("merge flags disabled separates longhand and shorthand", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy -", "CONGA_MERGE_FLAGS=false")

		assert.True(t, len(result.GroupItems("longhand flags")) > 0, "expected longhand flags group")
		assert.True(t, len(result.GroupItems("shorthand flags")) > 0, "expected shorthand flags group")
		assert.Zero(t, len(result.GroupItems("flags")), "merged flags group must be empty when CONGA_MERGE_FLAGS=false")
	})

	t.Run("merge flags default merges all flags into flags group", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy -")

		assert.True(t, len(result.GroupItems("flags")) > 0, "expected merged flags group")
		assert.Zero(t, len(result.GroupItems("longhand flags")), "longhand flags group must be empty under default merge")
	})

	t.Run("color disabled strips list colors", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl ", "NO_COLOR=1")

		assert.Equal(t, "", result.ListColors, "list colors must be empty when color is disabled via NO_COLOR")
	})
}

func TestZsh_ColonMultiParts(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("image repository prefixes", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --image ")

		assert.True(t, result.HasItem("api-gateway:") || result.HasItem("api-gateway"))
		assert.True(t, result.HasItem("auth-service:") || result.HasItem("auth-service"))
	})

	t.Run("image tags after repository prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --image api-gateway:")

		assert.True(t, result.HasItem("v3.1.0") || result.HasItem("api-gateway:v3.1.0"))
		assert.True(t, result.HasItem("latest") || result.HasItem("api-gateway:latest"))
	})

	t.Run("unique tag prefix offers colon candidate", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --image api-gateway:v3.2.1-")

		assert.True(t, result.HasItem("v3.2.1-rc1") || result.HasItem("api-gateway:v3.2.1-rc1"))
	})
}

func TestZsh_DomainType_Region(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl deploy --region ")

	assert.True(t, result.HasItem("us-east-1"))
	assert.True(t, result.HasItem("eu-west-1"))
	assert.True(t, result.HasItem("ap-southeast-1"))
}

func TestZsh_ContainerExclusion(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl exec --container ")

	assert.True(t, result.HasItem("app"))
	assert.True(t, result.HasItem("cache"))
	assert.False(t, result.HasItem("istio-proxy"), "excluded container must not be suggested")
}

func TestZsh_PredictorSDK_Combinators(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("keeporder severity values", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --severity ")

		assert.True(t, result.HasItem("debug"))
		assert.True(t, result.HasItem("info"))
		assert.True(t, result.HasItem("warn"))
		assert.True(t, result.HasItem("error"))
	})

	t.Run("rich items sink completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --sink ")

		assert.True(t, result.HasItem("stdout (console)") || result.HasItem("stdout"))
		assert.True(t, result.HasItem("file"))
		assert.True(t, result.HasItem("syslog"))
	})

	t.Run("prefix and suffix combinators", func(t *testing.T) {
		t.Parallel()

		bucketResult := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --bucket ")
		assert.True(t, bucketResult.HasItem("logs-primary") || bucketResult.HasItem("s3://logs-primary"))
		assert.True(t, bucketResult.HasItem("logs-replica") || bucketResult.HasItem("s3://logs-replica"))

		collectorResult := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --collector ")
		assert.True(t, collectorResult.HasItem("collector-a") || collectorResult.HasItem("collector-a:5514"))
		assert.True(t, collectorResult.HasItem("collector-b") || collectorResult.HasItem("collector-b:5514"))
	})

	t.Run("flagvalues deduplication on mute slice", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "cloudctl", "cloudctl logs --mute api,")

		assert.False(t, result.HasItem("api,api"))
		assert.True(t, result.HasItem("api,auth") || result.HasItem("auth"))
		assert.True(t, result.HasItem("api,billing") || result.HasItem("billing"))
		assert.True(t, result.HasItem("api,frontend") || result.HasItem("frontend"))
	})
}

func TestZsh_ModularPredictors(t *testing.T) {
	t.Parallel()

	workspace := setupNodectl(t)

	t.Run("subcommands discovery", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "nodectl", "nodectl ")

		assert.True(t, result.HasItem("system"))
		assert.True(t, result.HasItem("network"))
		assert.True(t, result.HasItem("completion"))
	})

	t.Run("system kill signals", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "nodectl", "nodectl system kill --signal ")

		assert.True(t, result.HasItem("SIGTERM") || result.HasItem("15"))
		assert.True(t, result.HasItem("SIGKILL") || result.HasItem("9"))
	})

	t.Run("network subcommands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "nodectl", "nodectl network ")

		assert.True(t, result.HasItem("probe"))
		assert.True(t, result.HasItem("scan"))
		assert.True(t, result.HasItem("serve"))
	})
}
