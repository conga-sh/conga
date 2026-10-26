//go:build shelltest

package shell_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestBash_Subcommands(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl ")

	assert.True(t, result.HasItem("deploy"), "expected deploy subcommand")
	assert.True(t, result.HasItem("logs"), "expected logs subcommand")
	assert.True(t, result.HasItem("completion"), "expected completion subcommand")
}

func TestBash_SubcommandPrefix(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl d")

	assert.True(t, result.HasItem("deploy"))
	assert.False(t, result.HasItem("logs"))
}

func TestBash_Flags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --")

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

	logsResult := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --")

	assert.True(t, logsResult.HasItem("--labels"))
}

func TestBash_NegatableFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --no-")

	assert.True(t, result.HasItem("--no-dry-run"))
	assert.False(t, result.HasItem("--env"))
}

func TestBash_AttachedFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env=")

	// Wordbreak characters like '=' trigger quoting in the bash driver
	assert.True(t, result.HasItem(`"--env=staging"`) || result.HasItem("--env=staging") || result.HasItem("staging"))
	assert.True(t, result.HasItem(`"--env=production"`) || result.HasItem("--env=production") || result.HasItem("production"))
	assert.True(t, result.HasItem(`"--env=qa"`) || result.HasItem("--env=qa") || result.HasItem("qa"))
}

func TestBash_SeparateFlagValue(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env ")

	assert.True(t, result.HasItem("staging"))
	assert.True(t, result.HasItem("production"))
	assert.True(t, result.HasItem("qa"))
}

func TestBash_ContextAwarePredictors(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("production clusters", func(t *testing.T) {
		t.Parallel()

		// Initial TAB returns common prefix "prod-"
		resPrefix := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env production --cluster ")
		assert.True(t, resPrefix.HasItem("prod-"))

		// Once common prefix is entered, TAB returns all matching candidates
		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env production --cluster prod-")
		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.True(t, result.HasItem("prod-ap-southeast-1"))
		assert.False(t, result.HasItem("qa-test-1"))
		assert.False(t, result.HasItem("staging-cluster-alpha"))
	})

	t.Run("qa clusters", func(t *testing.T) {
		t.Parallel()

		// Initial TAB returns common prefix "qa-test-"
		resPrefix := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env qa --cluster ")
		assert.True(t, resPrefix.HasItem("qa-test-"))

		// Once common prefix is entered, TAB returns all matching candidates
		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env qa --cluster qa-test-")
		assert.True(t, result.HasItem("qa-test-1"))
		assert.True(t, result.HasItem("qa-test-2"))
		assert.False(t, result.HasItem("prod-us-east-1"))
	})
}

func TestBash_Wordbreaks(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --labels team=")

	// Quoted by the bash driver due to '=' wordbreak char
	assert.True(t, result.HasItem(`"team=core"`) || result.HasItem("team=core") || result.HasItem("core"))
	assert.True(t, result.HasItem(`"team=frontend"`) || result.HasItem("team=frontend") || result.HasItem("frontend"))
	assert.True(t, result.HasItem(`"team=infra"`) || result.HasItem("team=infra") || result.HasItem("infra"))
}

func TestBash_PositionalEnumAndFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs ")

	// Positional arguments
	assert.True(t, result.HasItem("auth"))
	assert.True(t, result.HasItem("billing"))
	assert.True(t, result.HasItem("api"))
	assert.True(t, result.HasItem("frontend"))

	// Flags are gated behind hyphen in flat shells
	assert.False(t, result.HasItem("--env"))
	assert.False(t, result.HasItem("--follow"))
	assert.False(t, result.HasItem("--help"))

	// Flags are discovered when hyphen is typed
	resFlags := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --")
	assert.True(t, resFlags.HasItem("--env"))
	assert.True(t, resFlags.HasItem("--follow"))
	assert.True(t, resFlags.HasItem("--help"))
}

func TestBash_SuccessiveTabsDescriptions(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	// In Bash, COMP_TYPE=63 simulates successive tabs which outputs descriptions
	result := workspace.RunBashCompletionType(t, "cloudctl", "cloudctl ", "63")

	hasDeployWithDesc := false

	for _, item := range result.Items {
		if strings.HasPrefix(item, "deploy") && strings.Contains(item, "Deploy service") {
			hasDeployWithDesc = true

			break
		}
	}

	assert.True(t, hasDeployWithDesc, "expected deploy candidate with description in successive tab mode (COMP_TYPE=63), got %v", result.Items)
}

func TestBash_AliasAndDescriptionSidebands(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("aliases shown by default", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl ")

		assert.True(t, result.HasItem("deploy"))
		assert.True(t, result.HasItem("d"))
	})

	t.Run("aliases suppressed", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl ", "CONGA_COMPLETION_ALIASES=0")

		assert.True(t, result.HasItem("deploy"))
		assert.False(t, result.HasItem("d"))
	})

	t.Run("descriptions suppressed in successive-tab listing", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletionType(t, "cloudctl", "cloudctl ", "63", "CONGA_COMPLETION_DESCRIPTIONS=0")

		assert.True(t, result.HasItem("deploy"))

		for _, item := range result.Items {
			assert.NotContains(t, item, "Deploy service")
		}
	})
}

func TestBash_FileExtensionFiltering(t *testing.T) {
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

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --manifest=")

		assert.True(t, result.HasItem(`"--manifest=app.yaml"`) || result.HasItem("--manifest=app.yaml") || result.HasItem("app.yaml"), "expected app.yaml candidate")
		assert.True(t, result.HasItem(`"--manifest=config.yml"`) || result.HasItem("--manifest=config.yml") || result.HasItem("config.yml"), "expected config.yml candidate")
		assert.True(t, result.HasItem(`"--manifest=nested/"`) || result.HasItem("--manifest=nested/") || result.HasItem("nested/"), "expected nested/ directory candidate")
		assert.False(t, result.HasItem("readme.md") || result.HasItem(`"--manifest=readme.md"`) || result.HasItem("--manifest=readme.md"), "did not expect readme.md candidate")
		assert.False(t, result.HasItem("data.json") || result.HasItem(`"--manifest=data.json"`) || result.HasItem("--manifest=data.json"), "did not expect data.json candidate")
		assert.False(t, result.HasItem("*.yaml") || result.HasItem(`"--manifest=*.yaml"`), "did not expect literal glob *.yaml")
		assert.False(t, result.HasItem("*.yml") || result.HasItem(`"--manifest=*.yml"`), "did not expect literal glob *.yml")
	})

	t.Run("separate flag completion in root dir", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --manifest ")

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

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --manifest nested/")

		assert.True(t, result.HasItem("manifest.yaml") || result.HasItem("nested/manifest.yaml"), "expected manifest.yaml candidate")
		assert.True(t, result.HasItem("child/") || result.HasItem("nested/child/"), "expected child/ directory candidate")
		assert.False(t, result.HasItem("ignore.txt") || result.HasItem("nested/ignore.txt"), "did not expect ignore.txt candidate")
		assert.False(t, result.HasItem("*.yaml"), "did not expect literal glob *.yaml")
	})

	t.Run("nested directory traversal with attached flag", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --manifest=nested/")

		assert.True(t, result.HasItem("manifest.yaml") || result.HasItem("nested/manifest.yaml") || result.HasItem(`"--manifest=nested/manifest.yaml"`), "expected manifest.yaml candidate")
		assert.True(t, result.HasItem("child/") || result.HasItem("nested/child/") || result.HasItem(`"--manifest=nested/child/"`), "expected child/ candidate")
		assert.False(t, result.HasItem("ignore.txt") || result.HasItem("nested/ignore.txt"), "did not expect ignore.txt candidate")
		assert.False(t, result.HasItem("*.yaml"), "did not expect literal glob *.yaml")
	})
}

func TestBash_DirectoryFiltering(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	workspace.Mkdir(t, "build")
	workspace.Mkdir(t, "build/sub")
	workspace.Mkdir(t, "dist")
	workspace.WriteFile(t, "app.go", []byte("package main"))

	t.Run("attached flag directory filtering", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --out-dir=")

		assert.True(t, result.HasItem("build/") || result.HasItem(`"--out-dir=build/"`) || result.HasItem("--out-dir=build/"), "expected build/ candidate")
		assert.True(t, result.HasItem("dist/") || result.HasItem(`"--out-dir=dist/"`) || result.HasItem("--out-dir=dist/"), "expected dist/ candidate")
		assert.False(t, result.HasItem("app.go") || result.HasItem(`"--out-dir=app.go"`), "did not expect app.go candidate")
	})

	t.Run("separate flag directory filtering", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --out-dir ")

		assert.True(t, result.HasItem("build/"), "expected build/ candidate")
		assert.True(t, result.HasItem("dist/"), "expected dist/ candidate")
		assert.False(t, result.HasItem("app.go"), "did not expect app.go candidate")
	})

	t.Run("nested directory traversal", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --out-dir build/")

		assert.True(t, result.HasItem("sub/") || result.HasItem("build/sub/"), "expected sub/ candidate")
		assert.False(t, result.HasItem("app.go"), "did not expect app.go candidate")
	})
}

func TestBash_SubcommandAlias(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl d --")

	assert.True(t, result.HasItem("--env"))
	assert.True(t, result.HasItem("--cluster"))
	assert.True(t, result.HasItem("--manifest"))
	assert.True(t, result.HasItem("--dry-run"))
}

func TestBash_ShortFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("short flag enumeration", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy -")

		assert.True(t, result.HasItem("-e"))
		assert.True(t, result.HasItem("-c"))
		assert.True(t, result.HasItem("--help"))
	})

	t.Run("separate short flag value completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy -e ")

		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
		assert.True(t, result.HasItem("qa"))
	})

	t.Run("attached short flag value completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy -e=")

		assert.True(t, result.HasItem(`"-e=staging"`) || result.HasItem("-e=staging") || result.HasItem("staging"))
		assert.True(t, result.HasItem(`"-e=production"`) || result.HasItem("-e=production") || result.HasItem("production"))
	})

	t.Run("short flag context-aware prediction", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy -e production -c prod-")

		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.True(t, result.HasItem("prod-ap-southeast-1"))
		assert.False(t, result.HasItem("qa-test-1"))
	})
}

func TestBash_MultiParts(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("top-level tag keys", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --labels ")

		assert.True(t, result.HasItem("team") || result.HasItem("team=") || result.HasItem(`"team="`))
		assert.True(t, result.HasItem("tier") || result.HasItem("tier=") || result.HasItem(`"tier="`))
		assert.True(t, result.HasItem("owner") || result.HasItem("owner=") || result.HasItem(`"owner="`))
	})

	t.Run("nested tag values", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --labels team=")

		assert.True(t, result.HasItem(`"team=core"`) || result.HasItem("team=core") || result.HasItem("core"))
		assert.True(t, result.HasItem(`"team=frontend"`) || result.HasItem("team=frontend") || result.HasItem("frontend"))
		assert.True(t, result.HasItem(`"team=infra"`) || result.HasItem("team=infra") || result.HasItem("infra"))
	})
}

func TestBash_QuotedArguments(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("double quoted flag context", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", `cloudctl deploy --env "production" --cluster prod-`)

		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.False(t, result.HasItem("qa-test-1"))
	})

	t.Run("single quoted flag context", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --env 'qa' --cluster qa-test-")

		assert.True(t, result.HasItem("qa-test-1"))
		assert.True(t, result.HasItem("qa-test-2"))
		assert.False(t, result.HasItem("prod-us-east-1"))
	})
}

func TestBash_PureFlagsCommandWithRequiredFlag(t *testing.T) {
	t.Parallel()

	workspace := setupFlagConstraints(t)

	result := workspace.RunBashCompletion(t, "06-flag-constraints", "06-flag-constraints --")

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
	assert.Equal(t, "", result.StdoutLeak)
}

func TestBash_DomainTypes_Completion(t *testing.T) {
	t.Parallel()

	workspace := setupDomainTypes(t)

	// Bare space: completion subcommand must NOT appear, flags must appear
	bareRes := workspace.RunBashCompletion(t, "05-domain-types", "05-domain-types ")
	assert.False(t, bareRes.HasItem("completion"))
	assert.True(t, bareRes.HasItem("--input-file"))
	assert.True(t, bareRes.HasItem("--completion"))
	assert.True(t, bareRes.HasItem("-i"))
	assert.Equal(t, "", bareRes.StdoutLeak)

	// Typing '--comp': must complete --completion flag
	compRes := workspace.RunBashCompletion(t, "05-domain-types", "05-domain-types --comp")
	assert.True(t, compRes.HasItem("--completion"), "expected --completion candidate")
	assert.Equal(t, "05-domain-types --completion", compRes.BufferTab1)
	assert.Equal(t, "", compRes.StdoutLeak)

	// Typing '--completion z': must complete the zsh shell value
	valueRes := workspace.RunBashCompletion(t, "05-domain-types", "05-domain-types --completion z")
	assert.True(t, valueRes.HasItem("zsh"), "expected zsh shell candidate")
	assert.Equal(t, "05-domain-types --completion zsh", valueRes.BufferTab1)
	assert.Equal(t, "", valueRes.StdoutLeak)

	// Typing '--completion ': must list every supported shell
	listRes := workspace.RunBashCompletion(t, "05-domain-types", "05-domain-types --completion ")
	assert.True(t, listRes.HasItem("bash"), "expected bash shell candidate")
	assert.True(t, listRes.HasItem("zsh"), "expected zsh shell candidate")
	assert.True(t, listRes.HasItem("fish"), "expected fish shell candidate")
	assert.True(t, listRes.HasItem("powershell"), "expected powershell shell candidate")
	assert.Equal(t, "", listRes.StdoutLeak)
}

func TestBash_DeprecatedFiltering(t *testing.T) {
	t.Parallel()

	workspace := setupMemberDeprecated(t)

	t.Run("deprecated flag excluded from flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "member-deprecated", "member-deprecated --")

		assert.True(t, result.HasItem("--modern"))
		assert.False(t, result.HasItem("--old"))
		assert.Equal(t, "", result.StdoutLeak)

		prefixRes := workspace.RunBashCompletion(t, "member-deprecated", "member-deprecated --o")

		assert.False(t, prefixRes.HasItem("--old"))
		assert.Equal(t, "member-deprecated --o", prefixRes.BufferTab1)
		assert.Equal(t, "", prefixRes.StdoutLeak)
	})

	t.Run("deprecated subcommand excluded from commands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "member-deprecated", "member-deprecated ")

		assert.True(t, result.HasItem("deploy"))
		assert.False(t, result.HasItem("legacy"))
		assert.Equal(t, "", result.StdoutLeak)

		prefixRes := workspace.RunBashCompletion(t, "member-deprecated", "member-deprecated l")

		assert.False(t, prefixRes.HasItem("legacy"))
		assert.Equal(t, "member-deprecated l", prefixRes.BufferTab1)
		assert.Equal(t, "", prefixRes.StdoutLeak)
	})
}

func TestBash_AppDeprecatedCompletion(t *testing.T) {
	t.Parallel()

	workspace := setupAppDeprecated(t)

	result := workspace.RunBashCompletion(t, "app-deprecated", "app-deprecated ")

	assert.True(t, result.HasItem("deploy"))
	assert.Equal(t, "", result.StdoutLeak)
}

func TestBash_XORFlagSuppression(t *testing.T) {
	t.Parallel()

	workspace := setupFlagConstraints(t)

	t.Run("json suppresses yaml and table", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "06-flag-constraints", "06-flag-constraints --json --")

		assert.False(t, result.HasItem("--yaml"))
		assert.False(t, result.HasItem("--table"))
		assert.True(t, result.HasItem("--api-key"))
		assert.True(t, result.HasItem("--port"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("yaml suppresses json and table", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "06-flag-constraints", "06-flag-constraints --yaml --")

		assert.False(t, result.HasItem("--json"))
		assert.False(t, result.HasItem("--table"))
		assert.True(t, result.HasItem("--api-key"))
		assert.True(t, result.HasItem("--port"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestBash_NonRepeatableFlagSuppression(t *testing.T) {
	t.Parallel()

	workspace := setupFlagConstraints(t)

	t.Run("scalar flag is suppressed after consumption", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "06-flag-constraints", "06-flag-constraints --port 8080 --")

		assert.False(t, result.HasItem("--port"))
		assert.True(t, result.HasItem("--api-key"))
		assert.True(t, result.HasItem("--zones"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("slice flag remains available after consumption", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "06-flag-constraints", "06-flag-constraints --zones us-east-1a --")

		assert.True(t, result.HasItem("--zones"))
		assert.True(t, result.HasItem("--port"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestBash_RootFlagInheritanceAndPreSubcommandPlacement(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("inherited root flag completion after subcommand", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --pro")

		assert.True(t, result.HasItem("--profile"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("pre-subcommand root flag affects subcommand predictor", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl --profile production exec ")

		assert.True(t, result.HasItem("api-prod-8a"))
		assert.True(t, result.HasItem("billing-prod-3b"))
		assert.True(t, result.HasItem("auth-prod-7c"))
		assert.False(t, result.HasItem("api-stage-alpha"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestBash_ContextualPositionalPredictor(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("default profile and namespace pods prefix expansion", func(t *testing.T) {
		t.Parallel()

		resPrefix := workspace.RunBashCompletion(t, "cloudctl", "cloudctl exec ")
		assert.True(t, resPrefix.HasItem("a"))
		assert.Equal(t, "cloudctl exec a", resPrefix.BufferTab1)
		assert.Equal(t, "", resPrefix.StdoutLeak)

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl exec api-")
		assert.True(t, result.HasItem("api-stage-alpha"))
		assert.False(t, result.HasItem("api-prod-8a"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("database namespace pods without common prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl exec --namespace database ")
		assert.True(t, result.HasItem("pg-stage-0"))
		assert.True(t, result.HasItem("redis-stage-0"))
		assert.False(t, result.HasItem("api-stage-alpha"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("contextual command suggestions per pod", func(t *testing.T) {
		t.Parallel()

		// Initial tab expands to common prefix "/bin/"
		resPrefix := workspace.RunBashCompletion(t, "cloudctl", "cloudctl exec api-stage-alpha ")
		assert.True(t, resPrefix.HasItem("/bin/") || resPrefix.HasItem("/bin/sh"))
		assert.Equal(t, "", resPrefix.StdoutLeak)

		// Once common prefix is entered, all matching default shells are listed
		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl exec api-stage-alpha /bin/")
		assert.True(t, result.HasItem("/bin/sh"))
		assert.True(t, result.HasItem("/bin/bash"))
		assert.Equal(t, "", result.StdoutLeak)

		prodResult := workspace.RunBashCompletion(t, "cloudctl", "cloudctl exec web-prod-1 ")
		assert.True(t, prodResult.HasItem("\"nginx -t\"") || prodResult.HasItem("nginx -t"))
		assert.Equal(t, "", prodResult.StdoutLeak)

		containerResult := workspace.RunBashCompletion(t, "cloudctl", "cloudctl exec --container app web-prod-1 ")
		assert.True(t, containerResult.HasItem("\"sh -c 'env'\"") || containerResult.HasItem("sh -c 'env'"))
		assert.Equal(t, "", containerResult.StdoutLeak)
	})
}

func TestBash_ColonMultiParts(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("image repository prefixes", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --image ")

		assert.True(t, result.HasItem("api-gateway:") || result.HasItem(`"api-gateway:"`))
		assert.True(t, result.HasItem("auth-service:") || result.HasItem(`"auth-service:"`))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("image tags after repository prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --image api-gateway:")

		assert.True(t, result.HasItem("v3.1.0") || result.HasItem("api-gateway:v3.1.0"))
		assert.True(t, result.HasItem("latest") || result.HasItem("api-gateway:latest"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("unique tag prefix offers colon candidate", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --image api-gateway:v3.2.1-")

		assert.True(t, result.HasItem("v3.2.1-rc1") || result.HasItem("api-gateway:v3.2.1-rc1"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestBash_TerminatorDashArgs(t *testing.T) {
	t.Parallel()

	workspace := setupTrailingArgs(t)

	t.Run("flags suppressed after double dash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "runner", "runner test mypkg -- --")

		assert.False(t, result.HasItem("--verbose"))
		assert.False(t, result.HasItem("--help"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestBash_HiddenFlags(t *testing.T) {
	t.Parallel()

	workspace := setupFlagPrimitives(t)

	result := workspace.RunBashCompletion(t, "02-flag-primitives", "02-flag-primitives --")

	assert.True(t, result.HasItem("--verbosity"))
	assert.True(t, result.HasItem("--debug"))
	assert.True(t, result.HasItem("--port"))
	assert.False(t, result.HasItem("--secret-key"), "hidden flag must not appear in completion candidates")
	assert.Equal(t, "", result.StdoutLeak)
}

func TestBash_DomainFilesystemTypes(t *testing.T) {
	t.Parallel()

	workspace := setupDomainTypes(t)

	sampleFile := filepath.Join(workspace.WorkDir, "sample.txt")
	err := os.WriteFile(sampleFile, []byte("data"), 0o600)
	assert.NoError(t, err)

	outputDir := filepath.Join(workspace.WorkDir, "output_dir")
	err = os.Mkdir(outputDir, 0o700)
	assert.NoError(t, err)

	t.Run("input file completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "05-domain-types", "05-domain-types --input-file ")

		assert.True(t, result.HasItem("sample.txt"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("output dir completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "05-domain-types", "05-domain-types --output-dir ")

		assert.True(t, result.HasItem("output_dir/") || result.HasItem("output_dir"))
		assert.False(t, result.HasItem("sample.txt"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestBash_DomainType_Region(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --region ")

	assert.True(t, result.HasItem("us-east-1"))
	assert.True(t, result.HasItem("eu-west-1"))
	assert.True(t, result.HasItem("ap-southeast-1"))
	assert.Equal(t, "", result.StdoutLeak)
}

func TestBash_ContainerExclusion(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl exec --container ")

	assert.True(t, result.HasItem("app"))
	assert.True(t, result.HasItem("cache"))
	assert.False(t, result.HasItem("istio-proxy"), "excluded container must not be suggested")
	assert.Equal(t, "", result.StdoutLeak)
}

func TestBash_PredictorSDK_Combinators(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("keeporder severity values", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --severity ")

		assert.True(t, result.HasItem("debug"))
		assert.True(t, result.HasItem("info"))
		assert.True(t, result.HasItem("warn"))
		assert.True(t, result.HasItem("error"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("rich items sink completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --sink ")

		assert.True(t, result.HasItem("stdout"))
		assert.True(t, result.HasItem("file"))
		assert.True(t, result.HasItem("syslog"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("prefix and suffix combinators", func(t *testing.T) {
		t.Parallel()

		bucketPrefix := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --bucket ")
		assert.True(t, bucketPrefix.HasItem("s3://logs-") || bucketPrefix.HasItem("s3://logs-primary"))
		assert.Equal(t, "", bucketPrefix.StdoutLeak)

		bucketResult := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --bucket s3://logs-")
		assert.True(t, bucketResult.HasItem("//logs-primary") || bucketResult.HasItem("s3://logs-primary"))
		assert.True(t, bucketResult.HasItem("//logs-replica") || bucketResult.HasItem("s3://logs-replica"))
		assert.Equal(t, "", bucketResult.StdoutLeak)

		collectorPrefix := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --collector ")
		assert.True(t, collectorPrefix.HasItem("collector-") || collectorPrefix.HasItem("collector-a:5514"))
		assert.Equal(t, "", collectorPrefix.StdoutLeak)

		collectorResult := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --collector collector-")
		assert.True(t, collectorResult.HasItem("collector-a:5514"))
		assert.True(t, collectorResult.HasItem("collector-b:5514"))
		assert.Equal(t, "", collectorResult.StdoutLeak)
	})

	t.Run("flagvalues deduplication on mute slice", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --mute api,")

		assert.False(t, result.HasItem("api,api"))
		assert.True(t, result.HasItem("api,auth") || result.HasItem("auth"))
		assert.True(t, result.HasItem("api,billing") || result.HasItem("billing"))
		assert.True(t, result.HasItem("api,frontend") || result.HasItem("frontend"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestBash_ModularPredictors(t *testing.T) {
	t.Parallel()

	workspace := setupNodectl(t)

	t.Run("subcommands discovery", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "nodectl", "nodectl ")

		assert.True(t, result.HasItem("system"))
		assert.True(t, result.HasItem("network"))
		assert.True(t, result.HasItem("completion"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("system kill signals", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "nodectl", "nodectl system kill --signal ")

		assert.True(t, result.HasItem("SIGTERM") || result.HasItem("15"))
		assert.True(t, result.HasItem("SIGKILL") || result.HasItem("9"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("network subcommands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "nodectl", "nodectl network ")

		assert.True(t, result.HasItem("probe"))
		assert.True(t, result.HasItem("scan"))
		assert.True(t, result.HasItem("serve"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}
