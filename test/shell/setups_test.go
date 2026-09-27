//go:build shelltest

package shell_test

import (
	"testing"

	"github.com/conga-sh/conga/test/shell"
)

func setupCloudctl(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/19-shell-completion", "cloudctl")

	return workspace
}

func setupFlagConstraints(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/06-flag-constraints", "06-flag-constraints")

	return workspace
}

func setupDomainTypes(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/05-domain-types", "05-domain-types")

	return workspace
}

func setupMemberDeprecated(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/23-member-deprecation", "member-deprecated")

	return workspace
}

func setupAppDeprecated(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/22-app-deprecation", "app-deprecated")

	return workspace
}

func setupTrailingArgs(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/11-trailing-args-forwarding", "runner")

	return workspace
}

func setupFlagPrimitives(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/02-flag-primitives", "02-flag-primitives")

	return workspace
}

func setupNodectl(t *testing.T) *shell.Workspace {
	t.Helper()

	workspace := shell.NewWorkspace(t)
	workspace.BuildBinary(t, "../../_examples/20-modular-predictors", "nodectl")

	return workspace
}
