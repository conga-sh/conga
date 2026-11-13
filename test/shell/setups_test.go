//go:build shelltest

package shell_test

import (
	"testing"

	"github.com/conga-sh/conga/test/shell"
)

func setupShellCompletion(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/19-shell-completion", "shell-completion")

	return workspace
}

func setupFlagConstraints(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/06-flag-constraints", "flag-constraints")

	return workspace
}

func setupDomainTypes(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/05-domain-types", "domain-types")

	return workspace
}

func setupMemberDeprecation(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/23-member-deprecation", "member-deprecation")

	return workspace
}

func setupAppDeprecation(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/22-app-deprecation", "app-deprecation")

	return workspace
}

func setupTrailingArgsForwarding(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/11-trailing-args-forwarding", "trailing-args-forwarding")

	return workspace
}

func setupFlagPrimitives(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/02-flag-primitives", "flag-primitives")

	return workspace
}

func setupModularPredictors(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/20-modular-predictors", "modular-predictors")

	return workspace
}

func setupOrdertool(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "./testdata/keeporder", "ordertool")

	return workspace
}

// setupWordtool builds the word-handling fixture next to a directory and a
// file whose names contain a space.
func setupWordtool(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "./testdata/wordtool", "wordtool")
	workspace.WriteFile(t, "My Dir/inner.yaml", []byte("kind: Config"))
	workspace.WriteFile(t, "My File.yaml", []byte("kind: Config"))
	workspace.WriteFile(t, `back\slash.yaml`, []byte("kind: Config"))

	return workspace
}

func setupRawtool(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "./testdata/implicitraw", "rawtool")

	return workspace
}
