package main

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/conga-sh/conga/_examples/internal/testutil"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"negmap": main,
	})
	os.Exit(m.Run())
}

func TestScripts(t *testing.T) {
	t.Parallel()
	testscript.Run(t, testscript.Params{
		Dir:  "testdata",
		Cmds: testutil.Commands(),
	})
}
