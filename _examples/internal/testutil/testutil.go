// Package testutil provides shared testscript commands used by the example
// end-to-end test suites.
package testutil

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// Main runs the example suite's test binary with COLUMNS cleared, so help
// screens render at the engine default width regardless of the invoking shell:
// an exported COLUMNS would otherwise wrap every fixture at the developer's
// terminal width. Scripts inherit the test process environment, so clearing it
// here covers every example that calls testscript.Run.
func Main(m *testing.M, commands map[string]func()) {
	os.Unsetenv("COLUMNS") //nolint:errcheck // "COLUMNS" is a valid key, so Unsetenv cannot fail.

	testscript.Main(m, commands)
}

// Commands returns the shared custom testscript commands registered by the
// example test suites.
//
//	exit-status <code> <program> [args...]
//	  Runs program and asserts the exact process exit code, leaving the
//	  command's stdout and stderr available for subsequent assertions.
//	  Use this to verify CONGA's exit-code contract: 2 for input/usage
//	  errors and 1 for runtime failures.
//
//	copy-fixture <source> <destination>
//	  Copies source (a path relative to the example package directory) into
//	  destination (a path relative to the testscript working directory).
//	  Use this to materialise fixture files without duplicating them as
//	  inline heredocs in every script.
func Commands() map[string]func(testScript *testscript.TestScript, neg bool, args []string) {
	return map[string]func(testScript *testscript.TestScript, neg bool, args []string){
		"copy-fixture": copyFixtureCmd,
		"exit-status":  exitStatusCmd,
	}
}

// copyFixtureCmd copies a file from the example package directory (the test
// binary's working directory, unaffected by script-internal `cd` commands)
// into the testscript working directory.
func copyFixtureCmd(testScript *testscript.TestScript, neg bool, args []string) {
	if neg {
		testScript.Fatalf("unsupported: ! copy-fixture")
	}

	if len(args) != 2 {
		testScript.Fatalf("usage: copy-fixture <source> <destination>")
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		testScript.Fatalf("copy-fixture %s: %v", args[0], err)
	}

	destination := testScript.MkAbs(args[1])
	if err := os.WriteFile(destination, data, 0o644); err != nil { //nolint:gosec // Fixture files are world-readable by design
		testScript.Fatalf("copy-fixture %s: %v", args[1], err)
	}
}

func exitStatusCmd(testScript *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		testScript.Fatalf("usage: exit-status <code> <program> [args...]")
	}

	expected, err := strconv.Atoi(args[0])
	if err != nil {
		testScript.Fatalf("invalid exit code %q: %v", args[0], err)
	}

	err = testScript.Exec(args[1], args[2:]...)

	actual := 0

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		actual = exitErr.ExitCode()
	} else if err != nil {
		testScript.Fatalf("exec %s: %v", args[1], err)
	}

	if neg {
		if actual == expected {
			testScript.Fatalf("unexpected exit status %d from %s", actual, args[1])
		}

		return
	}

	if actual != expected {
		testScript.Fatalf("expected exit status %d from %s, got %d", expected, args[1], actual)
	}
}
