//go:build shelltest

package shell_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestElvish_Subcommands(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl ")

	assert.Equal(t, "", result.StdoutLeak, "completion function must not emit raw stdout or notify in elvish")
	assert.True(t, result.HasItem("deploy"), "expected deploy subcommand")
	assert.True(t, result.HasItem("logs"), "expected logs subcommand")
	assert.True(t, result.HasItem("completion"), "expected completion subcommand")

	desc, ok := result.ItemDescription("deploy")
	assert.True(t, ok)
	assert.Equal(t, "Deploy service to a cluster", desc)
}

func TestElvish_Flags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --")

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

	logsResult := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --")

	assert.True(t, logsResult.HasItem("--labels"))
}

func TestElvish_ContextAwarePredictors(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --env production --cluster ")

	assert.True(t, result.HasItem("prod-us-east-1"))
	assert.True(t, result.HasItem("prod-eu-west-1"))
	assert.True(t, result.HasItem("prod-ap-southeast-1"))
	assert.False(t, result.HasItem("qa-test-1"))
}

func TestElvish_FileExtensionFiltering(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	workspace.WriteFile(t, "app.yaml", []byte("name: app"))
	workspace.WriteFile(t, "config.yml", []byte("port: 8080"))
	workspace.WriteFile(t, "readme.md", []byte("# Readme"))
	workspace.WriteFile(t, "data.json", []byte("{}"))
	workspace.Mkdir(t, "nested")
	workspace.Mkdir(t, "nested/child")
	workspace.WriteFile(t, "nested/manifest.yaml", []byte("kind: Pod"))
	workspace.WriteFile(t, "nested/ignore.txt", []byte("ignore"))

	t.Run("separate flag completion in root dir", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --manifest ")

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

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --manifest nested/")

		assert.True(t, result.HasItem("manifest.yaml") || result.HasItem("nested/manifest.yaml"), "expected manifest.yaml candidate")
		assert.True(t, result.HasItem("child/") || result.HasItem("nested/child/"), "expected child/ directory candidate")
		assert.False(t, result.HasItem("ignore.txt") || result.HasItem("nested/ignore.txt"), "did not expect ignore.txt candidate")
		assert.False(t, result.HasItem("*.yaml"), "did not expect literal glob *.yaml")
	})

	t.Run("attached flag completion in root dir", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --manifest=")

		assert.True(t, result.HasItem("--manifest=app.yaml") || result.HasItem("app.yaml"), "expected app.yaml candidate, got %v", result.Candidates)
		assert.True(t, result.HasItem("--manifest=config.yml") || result.HasItem("config.yml"), "expected config.yml candidate, got %v", result.Candidates)
		assert.True(t, result.HasItem("--manifest=nested/") || result.HasItem("nested/"), "expected nested/ directory candidate, got %v", result.Candidates)
		assert.False(t, result.HasItem("readme.md"), "did not expect readme.md candidate")
		assert.False(t, result.HasItem("data.json"), "did not expect data.json candidate")
		assert.False(t, result.HasItem("*.yaml"), "did not expect literal glob *.yaml")
		assert.False(t, result.HasItem("*.yml"), "did not expect literal glob *.yml")
	})

	t.Run("nested directory traversal with attached flag", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --manifest=nested/")

		assert.True(t, result.HasItem("--manifest=nested/manifest.yaml") || result.HasItem("nested/manifest.yaml"), "expected manifest.yaml candidate")
		assert.True(t, result.HasItem("--manifest=nested/child/") || result.HasItem("nested/child/"), "expected child/ candidate")
		assert.False(t, result.HasItem("ignore.txt") || result.HasItem("nested/ignore.txt"), "did not expect ignore.txt candidate")
		assert.False(t, result.HasItem("*.yaml"), "did not expect literal glob *.yaml")
	})
}

func TestElvish_DirectoryFiltering(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	workspace.Mkdir(t, "build")
	workspace.Mkdir(t, "build/sub")
	workspace.Mkdir(t, "dist")
	workspace.WriteFile(t, "app.go", []byte("package main"))

	t.Run("separate flag directory filtering", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --out-dir ")

		assert.True(t, result.HasItem("build/"), "expected build/ candidate")
		assert.True(t, result.HasItem("dist/"), "expected dist/ candidate")
		assert.False(t, result.HasItem("app.go"), "did not expect app.go candidate")
	})

	t.Run("attached flag directory filtering", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --out-dir=")

		assert.True(t, result.HasItem("--out-dir=build/") || result.HasItem("build/"), "expected build/ candidate, got %v", result.Candidates)
		assert.True(t, result.HasItem("--out-dir=dist/") || result.HasItem("dist/"), "expected dist/ candidate, got %v", result.Candidates)
		assert.False(t, result.HasItem("app.go"), "did not expect app.go candidate")
	})

	t.Run("nested directory traversal", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --out-dir build/")

		assert.True(t, result.HasItem("sub/") || result.HasItem("build/sub/"), "expected sub/ candidate")
		assert.False(t, result.HasItem("app.go"), "did not expect app.go candidate")
	})
}

func TestElvish_SubcommandPrefix(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl d")

	assert.True(t, result.HasItem("deploy"))
	assert.False(t, result.HasItem("logs"))
}

func TestElvish_SubcommandAlias(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl d --")

	assert.True(t, result.HasItem("--env"))
	assert.True(t, result.HasItem("--cluster"))
	assert.True(t, result.HasItem("--manifest"))
	assert.True(t, result.HasItem("--dry-run"))
}

func TestElvish_NegatableFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --no-")

	assert.True(t, result.HasItem("--no-dry-run"))

	desc, ok := result.ItemDescription("--no-dry-run")
	assert.True(t, ok)
	assert.Equal(t, "Disable Simulate deployment", desc)
}

func TestElvish_FlagValues(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --env ")

	assert.True(t, result.HasItem("staging"))
	assert.True(t, result.HasItem("production"))
	assert.True(t, result.HasItem("qa"))
}

func TestElvish_AttachedFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("attached long flag value", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --env=")

		assert.True(t, result.HasItem("--env=staging") || result.HasItem("staging"), "expected --env=staging candidate, got %v", result.Candidates)
		assert.True(t, result.HasItem("--env=production") || result.HasItem("production"), "expected --env=production candidate, got %v", result.Candidates)
		assert.True(t, result.HasItem("--env=qa") || result.HasItem("qa"), "expected --env=qa candidate, got %v", result.Candidates)
	})

	t.Run("attached flag value prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --env=p")

		assert.True(t, result.HasItem("--env=production") || result.HasItem("production"), "expected --env=production candidate, got %v", result.Candidates)
	})
}

func TestElvish_ShortFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("short flag enumeration", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy -")

		assert.True(t, result.HasItem("-e"))
		assert.True(t, result.HasItem("-c"))
		assert.True(t, result.HasItem("--help"))
	})

	t.Run("separate short flag value completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy -e ")

		assert.True(t, result.HasItem("staging"))
		assert.True(t, result.HasItem("production"))
		assert.True(t, result.HasItem("qa"))
	})

	t.Run("attached short flag value completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy -e=")

		assert.True(t, result.HasItem("-e=staging") || result.HasItem("staging"), "expected -e=staging candidate, got %v", result.Candidates)
		assert.True(t, result.HasItem("-e=production") || result.HasItem("production"), "expected -e=production candidate, got %v", result.Candidates)
	})

	t.Run("short flag context-aware prediction", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy -e production -c prod-")

		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.True(t, result.HasItem("prod-ap-southeast-1"))
		assert.False(t, result.HasItem("qa-test-1"))
	})
}

func TestElvish_MultiParts(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("top-level tag keys", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --labels ")

		assert.True(t, result.HasItem("team") || result.HasItem("team="))
		assert.True(t, result.HasItem("tier") || result.HasItem("tier="))
		assert.True(t, result.HasItem("owner") || result.HasItem("owner="))
	})

	t.Run("nested tag values", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --labels team=")

		assert.True(t, result.HasItem("team=core") || result.HasItem("core"), "expected team=core candidate")
		assert.True(t, result.HasItem("team=frontend") || result.HasItem("frontend"), "expected team=frontend candidate")
		assert.True(t, result.HasItem("team=infra") || result.HasItem("infra"), "expected team=infra candidate")
	})

	t.Run("map key deduplication across flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --labels team=core --labels ")

		assert.False(t, result.HasItem("team="), "team= must be excluded as it is already assigned")
		assert.True(t, result.HasItem("tier="), "expected tier= candidate")
		assert.True(t, result.HasItem("owner="), "expected owner= candidate")
	})

	t.Run("map value deduplication", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --labels owner=alice --labels owner=")

		assert.False(t, result.HasItem("owner=alice"), "alice must be excluded as it is already assigned")
		assert.False(t, result.HasItem("alice"), "bare alice must not be suggested either")
		assert.True(t, result.HasItem("owner=bob") || result.HasItem("bob"), "expected bob candidate")
		assert.True(t, result.HasItem("owner=carol") || result.HasItem("carol"), "expected carol candidate")
	})

	t.Run("delimited slice flag deduplication", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --features canary,")

		assert.False(t, result.HasItem("canary,canary"), "canary must not be suggested again")
		assert.True(t, result.HasItem("canary,tracing") || result.HasItem("tracing"), "expected tracing candidate")
	})
}

func TestElvish_PositionalEnumAndFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs ")

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
	resFlags := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --")
	assert.True(t, resFlags.HasItem("--env"))
	assert.True(t, resFlags.HasItem("--follow"))
	assert.True(t, resFlags.HasItem("--help"))
}

func TestElvish_UnconstrainedPositionalAndFlags(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("bare space retains buffer without stdout leakage", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy ")

		// Verify zero stdout leakage to prevent prompt displacement or terminal scrollback pollution
		assert.Equal(t, "", result.StdoutLeak, "completion function must not emit raw stdout or notify in elvish")

		// Verify active help text is NOT emitted as a candidate and does NOT mutate the command line buffer
		assert.False(t, result.HasItem("<service>: Service name to deploy"), "usage string must not be treated as a completion candidate")
		assert.Equal(t, "cloudctl deploy ", result.BufferTab1, "Tab 1 on unconstrained positional must not insert usage string")
		assert.Equal(t, "cloudctl deploy ", result.BufferTab2, "Tab 2 on unconstrained positional must not insert usage string")
		assert.Zero(t, result.Candidates, "expected zero completion candidates on unconstrained positional bare space")
	})

	t.Run("alias bare space retains buffer without stdout leakage", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl d ")

		assert.Equal(t, "", result.StdoutLeak, "completion on alias must produce zero stdout leakage")
		assert.False(t, result.HasItem("<service>: Service name to deploy"))
		assert.Equal(t, "cloudctl d ", result.BufferTab1)
		assert.Equal(t, "cloudctl d ", result.BufferTab2)
		assert.Zero(t, result.Candidates)
	})

	t.Run("flags discovered behind hyphen", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --")

		assert.Equal(t, "", result.StdoutLeak)
		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
		assert.True(t, result.HasItem("--help"))
	})
}

func TestElvish_QuotedArguments(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("double quoted flag context", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", `cloudctl deploy --env "production" --cluster prod-`)

		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.False(t, result.HasItem("qa-test-1"))
	})

	t.Run("single quoted flag context", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --env 'qa' --cluster qa-test-")

		assert.True(t, result.HasItem("qa-test-1"))
		assert.True(t, result.HasItem("qa-test-2"))
		assert.False(t, result.HasItem("prod-us-east-1"))
	})

	t.Run("single quoted positional prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs 'au")

		assert.True(t, result.HasItem("auth"), "expected auth candidate inside single quotes")
		assert.False(t, result.HasItem("billing"), "did not expect billing candidate")
	})

	t.Run("double quoted positional prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", `cloudctl logs "bi`)

		assert.True(t, result.HasItem("billing"), "expected billing candidate inside double quotes")
		assert.False(t, result.HasItem("auth"), "did not expect auth candidate")
	})

	t.Run("flags after quoted positional argument", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy 'my-service' --")

		assert.True(t, result.HasItem("--env"), "expected --env after quoted positional argument")
		assert.True(t, result.HasItem("--cluster"), "expected --cluster after quoted positional argument")
	})
}

func TestElvish_PureFlagsCommandWithRequiredFlag(t *testing.T) {
	t.Parallel()

	workspace := setupFlagConstraints(t)

	result := workspace.RunElvishCompletion(t, "06-flag-constraints", "06-flag-constraints --")

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
}

func TestElvish_DomainTypes_Completion(t *testing.T) {
	t.Parallel()

	workspace := setupDomainTypes(t)

	// Bare space: completion subcommand must NOT appear, flags must appear
	bareRes := workspace.RunElvishCompletion(t, "05-domain-types", "05-domain-types ")
	assert.False(t, bareRes.HasItem("completion"))
	assert.True(t, bareRes.HasItem("--input-file"))
	assert.True(t, bareRes.HasItem("--completion"))
	assert.True(t, bareRes.HasItem("-i"))

	// Typing '--comp': must complete --completion flag
	compRes := workspace.RunElvishCompletion(t, "05-domain-types", "05-domain-types --comp")
	assert.True(t, compRes.HasItem("--completion"), "expected --completion candidate")
}

func TestElvish_NestedSubcommands(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("completion shell candidates", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl completion ")

		assert.True(t, result.HasItem("bash"), "expected bash candidate")
		assert.True(t, result.HasItem("zsh"), "expected zsh candidate")
		assert.True(t, result.HasItem("fish"), "expected fish candidate")
		assert.True(t, result.HasItem("powershell"), "expected powershell candidate")
		assert.True(t, result.HasItem("elvish"), "expected elvish candidate")
		assert.True(t, result.HasItem("nushell"), "expected nushell candidate")
	})

	t.Run("completion prefix matching", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl completion z")

		assert.True(t, result.HasItem("zsh"))
		assert.False(t, result.HasItem("bash"))
	})
}

func TestElvish_DeprecatedFiltering(t *testing.T) {
	t.Parallel()

	workspace := setupMemberDeprecated(t)

	t.Run("deprecated flag excluded from flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "member-deprecated", "member-deprecated --")

		assert.True(t, result.HasItem("--modern"))
		assert.False(t, result.HasItem("--old"))

		prefixRes := workspace.RunElvishCompletion(t, "member-deprecated", "member-deprecated --o")

		assert.False(t, prefixRes.HasItem("--old"))
	})

	t.Run("deprecated subcommand excluded from commands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "member-deprecated", "member-deprecated ")

		assert.True(t, result.HasItem("deploy"))
		assert.False(t, result.HasItem("legacy"))

		prefixRes := workspace.RunElvishCompletion(t, "member-deprecated", "member-deprecated l")

		assert.False(t, prefixRes.HasItem("legacy"))
	})
}

func TestElvish_AppDeprecatedCompletion(t *testing.T) {
	t.Parallel()

	workspace := setupAppDeprecated(t)

	result := workspace.RunElvishCompletion(t, "app-deprecated", "app-deprecated ")

	assert.True(t, result.HasItem("deploy"), "expected deploy subcommand of the deprecated app")
}

func TestElvish_XORFlagSuppression(t *testing.T) {
	t.Parallel()

	workspace := setupFlagConstraints(t)

	t.Run("json suppresses yaml and table", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "06-flag-constraints", "06-flag-constraints --json --")

		assert.False(t, result.HasItem("--yaml"))
		assert.False(t, result.HasItem("--table"))
		assert.True(t, result.HasItem("--api-key"))
		assert.True(t, result.HasItem("--port"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("yaml suppresses json and table", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "06-flag-constraints", "06-flag-constraints --yaml --")

		assert.False(t, result.HasItem("--json"))
		assert.False(t, result.HasItem("--table"))
		assert.True(t, result.HasItem("--api-key"))
		assert.True(t, result.HasItem("--port"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestElvish_NonRepeatableFlagSuppression(t *testing.T) {
	t.Parallel()

	workspace := setupFlagConstraints(t)

	t.Run("scalar flag is suppressed after consumption", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "06-flag-constraints", "06-flag-constraints --port 8080 --")

		assert.False(t, result.HasItem("--port"))
		assert.True(t, result.HasItem("--api-key"))
		assert.True(t, result.HasItem("--zones"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("slice flag remains available after consumption", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "06-flag-constraints", "06-flag-constraints --zones us-east-1a --")

		assert.True(t, result.HasItem("--zones"))
		assert.True(t, result.HasItem("--port"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestElvish_RootFlagInheritanceAndPreSubcommandPlacement(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("inherited root flag completion after subcommand", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --pro")

		assert.True(t, result.HasItem("--profile"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("pre-subcommand root flag affects subcommand predictor", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl --profile production exec ")

		assert.True(t, result.HasItem("api-prod-8a"))
		assert.True(t, result.HasItem("billing-prod-3b"))
		assert.True(t, result.HasItem("auth-prod-7c"))
		assert.False(t, result.HasItem("api-stage-alpha"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestElvish_ContextualPositionalPredictor(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("default profile and namespace pods", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl exec ")

		assert.True(t, result.HasItem("api-stage-alpha"))
		assert.True(t, result.HasItem("auth-stage-beta"))
		assert.False(t, result.HasItem("api-prod-8a"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("database namespace pods without common prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl exec --namespace database ")

		assert.True(t, result.HasItem("pg-stage-0"))
		assert.True(t, result.HasItem("redis-stage-0"))
		assert.False(t, result.HasItem("api-stage-alpha"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestElvish_TerminatorDashArgs(t *testing.T) {
	t.Parallel()

	workspace := setupTrailingArgs(t)

	t.Run("flags suppressed after double dash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "runner", "runner test mypkg -- --")

		assert.False(t, result.HasItem("--verbose"))
		assert.False(t, result.HasItem("--help"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestElvish_HiddenFlags(t *testing.T) {
	t.Parallel()

	workspace := setupFlagPrimitives(t)

	result := workspace.RunElvishCompletion(t, "02-flag-primitives", "02-flag-primitives --")

	assert.True(t, result.HasItem("--verbosity"))
	assert.True(t, result.HasItem("--debug"))
	assert.True(t, result.HasItem("--port"))
	assert.False(t, result.HasItem("--secret-key"), "hidden flag must not appear in completion candidates")
	assert.Equal(t, "", result.StdoutLeak)
}

func TestElvish_DomainFilesystemTypes(t *testing.T) {
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

		result := workspace.RunElvishCompletion(t, "05-domain-types", "05-domain-types --input-file ")

		assert.True(t, result.HasItem("sample.txt"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("output dir completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "05-domain-types", "05-domain-types --output-dir ")

		assert.True(t, result.HasItem("output_dir/") || result.HasItem("output_dir"))
		assert.False(t, result.HasItem("sample.txt"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestElvish_SidebandDescriptionLength(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl ", "CONGA_DESCRIPTION_LENGTH=10")

	assert.Equal(t, "", result.StdoutLeak)
	desc, ok := result.ItemDescription("deploy")
	assert.True(t, ok)
	assert.Equal(t, "Deploy ...", desc)
}

func TestElvish_ColonMultiParts(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("image repository prefixes", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --image ")

		assert.True(t, result.HasItem("api-gateway:"))
		assert.True(t, result.HasItem("auth-service:"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("image tags after repository prefix", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --image api-gateway:")

		assert.True(t, result.HasItem("v3.1.0") || result.HasItem("api-gateway:v3.1.0"))
		assert.True(t, result.HasItem("latest") || result.HasItem("api-gateway:latest"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestElvish_DomainType_Region(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl deploy --region ")

	assert.True(t, result.HasItem("us-east-1"))
	assert.True(t, result.HasItem("eu-west-1"))
	assert.True(t, result.HasItem("ap-southeast-1"))
	assert.Equal(t, "", result.StdoutLeak)
}

func TestElvish_ContainerExclusion(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl exec --container ")

	assert.True(t, result.HasItem("app"))
	assert.True(t, result.HasItem("cache"))
	assert.False(t, result.HasItem("istio-proxy"), "excluded container must not be suggested")
	assert.Equal(t, "", result.StdoutLeak)
}

func TestElvish_PredictorSDK_Combinators(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("keeporder severity values", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --severity ")

		assert.True(t, result.HasItem("debug"))
		assert.True(t, result.HasItem("info"))
		assert.True(t, result.HasItem("warn"))
		assert.True(t, result.HasItem("error"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("rich items sink completion", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --sink ")

		assert.True(t, result.HasItem("stdout"))
		assert.True(t, result.HasItem("file"))
		assert.True(t, result.HasItem("syslog"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("prefix and suffix combinators", func(t *testing.T) {
		t.Parallel()

		bucketResult := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --bucket ")
		assert.True(t, bucketResult.HasItem("s3://logs-primary"))
		assert.True(t, bucketResult.HasItem("s3://logs-replica"))
		assert.Equal(t, "", bucketResult.StdoutLeak)

		collectorResult := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --collector ")
		assert.True(t, collectorResult.HasItem("collector-a:5514"))
		assert.True(t, collectorResult.HasItem("collector-b:5514"))
		assert.Equal(t, "", collectorResult.StdoutLeak)
	})

	t.Run("flagvalues deduplication on mute slice", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "cloudctl", "cloudctl logs --mute api,")

		assert.False(t, result.HasItem("api,api"))
		assert.True(t, result.HasItem("api,auth") || result.HasItem("auth"))
		assert.True(t, result.HasItem("api,billing") || result.HasItem("billing"))
		assert.True(t, result.HasItem("api,frontend") || result.HasItem("frontend"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestElvish_ModularPredictors(t *testing.T) {
	t.Parallel()

	workspace := setupNodectl(t)

	t.Run("subcommands discovery", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "nodectl", "nodectl ")

		assert.True(t, result.HasItem("system"))
		assert.True(t, result.HasItem("network"))
		assert.True(t, result.HasItem("completion"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("system kill signals", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "nodectl", "nodectl system kill --signal ")

		assert.True(t, result.HasItem("SIGTERM") || result.HasItem("15"))
		assert.True(t, result.HasItem("SIGKILL") || result.HasItem("9"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("network subcommands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "nodectl", "nodectl network ")

		assert.True(t, result.HasItem("probe"))
		assert.True(t, result.HasItem("scan"))
		assert.True(t, result.HasItem("serve"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}
