package conga_test

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/predict"
)

var (
	// Expected completion script output, one golden per supported shell.
	//go:embed testdata/completion/bash.golden
	goldenBash []byte

	//go:embed testdata/completion/elvish.golden
	goldenElvish []byte

	//go:embed testdata/completion/fish.golden
	goldenFish []byte

	//go:embed testdata/completion/nushell.golden
	goldenNushell []byte

	//go:embed testdata/completion/powershell.golden
	goldenPowershell []byte

	//go:embed testdata/completion/zsh.golden
	goldenZsh []byte
)

type StructCompletionCLI struct {
	Completion conga.CompletionCommand  `summary:"Generate shell completion scripts"`
	Verbose    conga.OptionalFlag[bool] `help:"Verbose output"                       short:"v"`
}

type CustomNamedCompletionCLI struct {
	Completion conga.CompletionCommand `aliases:"completion" name:"comp" summary:"Custom completion helper"`
}

type PlainAppCLI struct {
	Name conga.OptionalFlag[string] `help:"App name" short:"n"`
}

type DomainDemoCLI struct {
	ConfigFile conga.OptionalFlag[conga.File]     `existing:"true"                    help:"Path to existing config file" short:"c"`
	OutputDir  conga.OptionalFlag[conga.Dir]      `existing:"true"                    help:"Path to existing directory"   short:"o"`
	MaxMemory  conga.OptionalFlag[conga.ByteSize] `default:"512MB"                    help:"Maximum memory limit"         short:"m"`
	Ports      conga.OptionalFlag[string]         `help:"Port range (e.g. 8000-8080)" short:"p"`
}

type RequiredFlagsCLI struct {
	APIKey   conga.RequiredFlag[string]   `help:"API key for authentication" short:"k"`
	Username conga.OptionalFlag[string]   `and:"auth"                        help:"Username"`
	Password conga.OptionalFlag[string]   `and:"auth"                        help:"Password"`
	JSON     conga.OptionalFlag[bool]     `help:"JSON"                       xor:"format"`
	YAML     conga.OptionalFlag[bool]     `help:"YAML"                       xor:"format"`
	Table    conga.OptionalFlag[bool]     `help:"Table"                      xor:"format"`
	Zones    conga.OptionalFlag[[]string] `help:"Zones"                      items:"2..3"`
	Port     conga.OptionalFlag[int]      `default:"8080"                    help:"Port"             range:"1..65535"`
	Env      conga.OptionalFlag[string]   `default:"dev"                     enum:"dev,staging,prod" help:"Env"`
}

type HierarchicalAppCLI struct {
	Deploy conga.Command[PlainAppCLI] `summary:"Deploy command"`
}

type TwoPositionalArgsCLI struct {
	First  conga.RequiredArg[string] `help:"First arg"`
	Second conga.RequiredArg[string] `enum:"red,green,blue" help:"Second arg"`
}

func TestCompletionScriptGeneration_AllShells(t *testing.T) {
	t.Parallel()

	opts := completion.DriverOptions{
		CommandName: "myapp",
		Aliases:     []string{"ma", "app"},
	}

	t.Run("bash script generation", func(t *testing.T) {
		t.Parallel()

		out, err := completion.Generate(completion.ShellBash, opts)

		assert.NoError(t, err)
		assert.Equal(t, string(goldenBash), out)
	})

	t.Run("zsh script generation", func(t *testing.T) {
		t.Parallel()

		out, err := completion.Generate(completion.ShellZsh, opts)

		assert.NoError(t, err)
		assert.Equal(t, string(goldenZsh), out)
	})

	t.Run("fish script generation", func(t *testing.T) {
		t.Parallel()

		out, err := completion.Generate(completion.ShellFish, opts)

		assert.NoError(t, err)
		assert.Equal(t, string(goldenFish), out)
	})

	t.Run("powershell script generation", func(t *testing.T) {
		t.Parallel()

		out, err := completion.Generate(completion.ShellPowerShell, opts)

		assert.NoError(t, err)
		assert.Equal(t, string(goldenPowershell), out)
	})

	t.Run("elvish script generation", func(t *testing.T) {
		t.Parallel()

		out, err := completion.Generate(completion.ShellElvish, opts)

		assert.NoError(t, err)
		assert.Equal(t, string(goldenElvish), out)
	})

	t.Run("nushell script generation", func(t *testing.T) {
		t.Parallel()

		out, err := completion.Generate(completion.ShellNushell, opts)

		assert.NoError(t, err)
		assert.Equal(t, string(goldenNushell), out)
	})
}

func TestCompletionCommand_Shell(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	cli := new(StructCompletionCLI)
	app := conga.New(cli).
		EnableCompletion().
		Name("myapp").
		Stdout(&buffer).
		Args([]string{"completion", "bash"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "bash", cli.Completion.Shell())
}

func TestCompletionCommand_ShellNilReceiver(t *testing.T) {
	t.Parallel()

	var command *conga.CompletionCommand

	assert.Zero(t, command.Shell())
}

func TestCompletionCommand_StructBased(t *testing.T) {
	t.Parallel()

	t.Run("execute completion bash", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"completion", "bash"})

		err := app.Run()

		assert.NoError(t, err)
		assertGolden(t, "testdata/completion/command-bash.golden", goldenCompletionCommandBash, buffer.String())
	})

	t.Run("execute completion zsh", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"completion", "zsh"})

		err := app.Run()

		assert.NoError(t, err)
		assertGolden(t, "testdata/completion/command-zsh.golden", goldenCompletionCommandZsh, buffer.String())
	})

	t.Run("execute completion powershell", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"completion", "powershell"})

		err := app.Run()

		assert.NoError(t, err)
		assertGolden(t, "testdata/completion/command-powershell.golden", goldenCompletionCommandPowershell, buffer.String())
	})
}

//nolint:paralleltest // Mutates global os.Args for invocation name testing.
func TestCompletionCommand_Argv0(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"/usr/local/bin/foobar", "completion", "zsh"}

	var buffer bytes.Buffer

	app := conga.New[StructCompletionCLI]().
		EnableCompletion().
		Name("myapp").
		Stdout(&buffer).
		Args([]string{"completion", "zsh"})

	err := app.Run()

	assert.NoError(t, err)

	expected, err := completion.Generate(completion.ShellZsh, completion.DriverOptions{CommandName: "foobar"})
	assert.NoError(t, err)
	assert.Equal(t, expected, buffer.String())
}

func TestCompletionCommand_StructTags(t *testing.T) {
	t.Parallel()

	t.Run("execute customized comp subcommand name", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[CustomNamedCompletionCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"comp", "fish"})

		err := app.Run()

		assert.NoError(t, err)

		expected, err := completion.Generate(completion.ShellFish, completion.DriverOptions{CommandName: "myapp"})
		assert.NoError(t, err)
		assert.Equal(t, expected, buffer.String())
	})

	t.Run("execute customized comp subcommand via alias", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[CustomNamedCompletionCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"completion", "fish"})

		err := app.Run()

		assert.NoError(t, err)

		expected, err := completion.Generate(completion.ShellFish, completion.DriverOptions{CommandName: "myapp"})
		assert.NoError(t, err)
		assert.Equal(t, expected, buffer.String())
	})
}

func TestApp_Completion_Fluent(t *testing.T) {
	t.Parallel()

	t.Run("mounts completion subcommand when enabled on multi-command app", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[HierarchicalAppCLI]().
			EnableCompletion().
			Name("cloudapp").
			Stdout(&buffer).
			Args([]string{"completion", "bash"})

		err := app.Run()

		assert.NoError(t, err)

		expected, err := completion.Generate(completion.ShellBash, completion.DriverOptions{CommandName: "cloudapp"})
		assert.NoError(t, err)
		assert.Equal(t, expected, buffer.String())
	})

	t.Run("flat app uses completion flag when enabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Name("plainapp").
			Stdout(&buffer).
			Args([]string{"--completion", "bash"})

		err := app.Run()

		assert.NoError(t, err)

		expected, err := completion.Generate(completion.ShellBash, completion.DriverOptions{CommandName: "plainapp"})
		assert.NoError(t, err)
		assert.Equal(t, expected, buffer.String())
	})

	t.Run("flat app rejects completion subcommand when not enabled", func(t *testing.T) {
		t.Parallel()

		app := conga.New[PlainAppCLI]().
			Name("plainapp").
			Args([]string{"completion", "bash"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("unknown flag when completion is not enabled", func(t *testing.T) {
		t.Parallel()

		app := conga.New[PlainAppCLI]().
			Name("plainapp").
			Args([]string{"--completion", "bash"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})
}

func TestCompletionCommand_Errors(t *testing.T) {
	t.Parallel()

	t.Run("unknown shell argument", func(t *testing.T) {
		t.Parallel()

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp").
			Args([]string{"completion", "unknownshell"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestCompletionCommand_NoShellShowsHelp(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[StructCompletionCLI]().
		EnableCompletion().
		Name("myapp").
		Stdout(&buffer).
		Args([]string{"completion"})

	err := app.Run()

	assert.NoError(t, err)

	// Bare `completion` renders its help screen (supported shells plus
	// current-shell enable hints) instead of failing. The enable hint is
	// derived from the live process chain, so only the static screen is pinned.
	output := buffer.String()
	assert.Contains(t, output, "Generate shell completion scripts")
	assert.Contains(t, output, "Usage:\n  myapp completion <shell>")
	assert.Contains(t, output, "Supported shells:\n  bash        Bourne Again SHell\n  elvish      Elvish shell\n  fish        Friendly interactive shell\n  nushell     Nushell shell\n  powershell  PowerShell / pwsh\n  zsh         Z shell")
}

func TestCompletionCommand_InteractiveTAB(t *testing.T) {
	t.Parallel()

	t.Run("complete shells after completion subcommand", func(t *testing.T) {
		t.Parallel()

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp")

		invoked, formatted := app.CompleteForShell("fish", []string{"completion", ""})

		assert.Equal(t, 6, len(invoked.Items))
		assert.Equal(t, "bash\tBourne Again SHell\nelvish\tElvish shell\nfish\tFriendly interactive shell\nnushell\tNushell shell\npowershell\tPowerShell / pwsh\nzsh\tZ shell", formatted)
	})

	t.Run("complete shell prefix", func(t *testing.T) {
		t.Parallel()

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp")

		invoked, formatted := app.CompleteForShell("fish", []string{"completion", "ba"})

		assert.Equal(t, 1, len(invoked.Items))
		assert.Equal(t, "bash", invoked.Items[0].Value)
		assert.Equal(t, "bash\tBourne Again SHell", formatted)
	})

	t.Run("simulate TAB via CompleteCompLine with mismatched executable name", func(t *testing.T) {
		t.Parallel()

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp")

		// Binary invoked as ./bin/custom-cli or /usr/local/bin/custom-app instead of "myapp"
		compLine := "./bin/custom-cli completion ba"
		invoked, formatted := app.CompleteCompLineForShell("fish", compLine, len(compLine))

		assert.Equal(t, 1, len(invoked.Items))
		assert.Equal(t, "bash", invoked.Items[0].Value)
		assert.Equal(t, "bash\tBourne Again SHell", formatted)
	})

	t.Run("simulate TAB for flags including built-in help", func(t *testing.T) {
		t.Parallel()

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp")

		compLine := "./different-binary --"
		invoked, formatted := app.CompleteCompLineForShell("fish", compLine, len(compLine))

		assert.True(t, len(invoked.Items) >= 2)
		assert.True(t, strings.Contains(formatted, "--help\tShow help for command"))
		assert.True(t, strings.Contains(formatted, "--verbose\tVerbose output"))
	})

	t.Run("simulate TAB prefix filter on flags", func(t *testing.T) {
		t.Parallel()

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp")

		compLine := "./different-binary --he"
		invoked, formatted := app.CompleteCompLineForShell("fish", compLine, len(compLine))

		assert.Equal(t, 1, len(invoked.Items))
		assert.Equal(t, "--help", invoked.Items[0].Value)
		assert.Equal(t, "--help\tShow help for command", formatted)
	})
}

func TestSingleCommand_InteractiveTAB(t *testing.T) {
	t.Parallel()

	t.Run("root TAB on single command expands flags and hides completion subcommand", func(t *testing.T) {
		t.Parallel()

		app := conga.New[DomainDemoCLI]().
			EnableCompletion().
			Name("domain-demo")

		invoked, formatted := app.CompleteForShell("fish", []string{""})

		assert.True(t, len(invoked.Items) >= 5)
		assert.False(t, strings.Contains(formatted, "completion\tGenerate shell completion scripts"))
		assert.True(t, strings.Contains(formatted, "--config-file\tPath to existing config file"))
		assert.True(t, strings.Contains(formatted, "--output-dir\tPath to existing directory"))
		assert.True(t, strings.Contains(formatted, "--max-memory\tMaximum memory limit"))
		assert.True(t, strings.Contains(formatted, "--ports\tPort range"))
		assert.True(t, strings.Contains(formatted, "--help\tShow help for command"))

		for _, item := range invoked.Items {
			assert.NotEqual(t, "completion", item.Value)
			assert.NotEqual(t, "commands", item.Tag)
		}
	})

	t.Run("root TAB on Zsh never contains completion subcommand or command tag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[DomainDemoCLI]().
			EnableCompletion().
			Name("domain-demo")

		invoked, formatted := app.CompleteForShell("zsh", []string{""})

		hasCompletion := slices.ContainsFunc(invoked.Items, func(item predict.Item) bool {
			return item.Value == "completion"
		})
		assert.False(t, hasCompletion)

		assert.False(t, strings.Contains(formatted, "\x1fcommands\x1fcompletion"))
	})

	t.Run("typing 'c' on single command does not complete completion", func(t *testing.T) {
		t.Parallel()

		app := conga.New[DomainDemoCLI]().
			EnableCompletion().
			Name("domain-demo")

		invoked, _ := app.CompleteForShell("fish", []string{"c"})

		assert.Zero(t, invoked.Items)
	})

	t.Run("flag prefix filter on single command matches config-file and completion", func(t *testing.T) {
		t.Parallel()

		app := conga.New[DomainDemoCLI]().
			EnableCompletion().
			Name("domain-demo")

		invoked, _ := app.CompleteForShell("fish", []string{"--co"})

		assert.Equal(t, 2, len(invoked.Items))
		assert.Equal(t, "--config-file", invoked.Items[0].Value)
		assert.Equal(t, "--completion", invoked.Items[1].Value)
	})

	t.Run("explicit completion command on single command only completes shells and no parent flags", func(t *testing.T) {
		t.Parallel()

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("struct-demo")

		invoked, formatted := app.CompleteForShell("fish", []string{"completion", ""})

		assert.Equal(t, 6, len(invoked.Items))
		assert.True(t, strings.Contains(formatted, "bash\tBourne Again SHell"))
		assert.True(t, strings.Contains(formatted, "zsh\tZ shell"))
		assert.True(t, strings.Contains(formatted, "fish\tFriendly interactive shell"))

		// Negative invariant: MUST NOT contain any of struct-demo's parent flags
		for _, item := range invoked.Items {
			assert.NotEqual(t, "--verbose", item.Value)
		}
	})

	t.Run("explicit completion command on Zsh has pure shell arguments and zero parent flags", func(t *testing.T) {
		t.Parallel()

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("struct-demo")

		invoked, formatted := app.CompleteForShell("zsh", []string{"completion", ""})

		// Negative invariant: Zsh formatted output must not leak parent flags
		assert.False(t, strings.Contains(formatted, "--verbose"))

		for _, item := range invoked.Items {
			assert.NotEqual(t, "--verbose", item.Value)
		}
	})
}

func TestCompletion_PureFlagsCommandWithRequiredFlag(t *testing.T) {
	t.Parallel()

	shells := []string{"bash", "zsh", "fish", "powershell", "elvish", "nushell"}

	for _, shell := range shells {
		t.Run("generate completion script for "+shell, func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer

			app := conga.New[RequiredFlagsCLI]().
				EnableCompletion().
				Name("constraints-demo").
				Stdout(&buffer).
				Args([]string{"--completion", shell})

			err := app.Run()

			assert.NoError(t, err)

			expected, err := completion.Generate(completion.Shell(shell), completion.DriverOptions{CommandName: "constraints-demo"})
			assert.NoError(t, err)
			assert.Equal(t, expected, buffer.String())
		})
	}

	t.Run("--help displays help screen without requiring required flag", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[RequiredFlagsCLI]().
			EnableCompletion().
			Name("constraints-demo").
			Stdout(&buffer).
			Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Usage:\n  constraints-demo --api-key=STRING")
	})

	t.Run("--help displays completion flag alongside declared flags", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[RequiredFlagsCLI]().
			EnableCompletion().
			Name("constraints-demo").
			Stdout(&buffer).
			Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "--completion=SHELL")
		assert.Contains(t, output, "--help")

		// Negative invariant: the flat command must not have a Commands section.
		assert.NotContains(t, output, "Commands:")
		assert.NotContains(t, output, "Global Flags")
	})

	t.Run("interactive completion discovers all flags without required flag error", func(t *testing.T) {
		t.Parallel()

		app := conga.New[RequiredFlagsCLI]().
			EnableCompletion().
			Name("constraints-demo")

		invoked, _ := app.CompleteForShell("zsh", []string{"--"})

		assert.True(t, len(invoked.Items) >= 9)

		hasFlag := func(name string) bool {
			for _, item := range invoked.Items {
				if item.Value == name {
					return true
				}
			}

			return false
		}

		assert.True(t, hasFlag("--api-key"))
		assert.True(t, hasFlag("--json"))
		assert.True(t, hasFlag("--yaml"))
		assert.True(t, hasFlag("--table"))
		assert.True(t, hasFlag("--username"))
		assert.True(t, hasFlag("--password"))
		assert.True(t, hasFlag("--port"))
		assert.True(t, hasFlag("--env"))
		assert.True(t, hasFlag("--zones"))
		assert.True(t, hasFlag("--help"))
	})

	t.Run("missing required flag is still strictly enforced during standard execution", func(t *testing.T) {
		t.Parallel()

		app := conga.New[RequiredFlagsCLI]().
			EnableCompletion().
			Name("constraints-demo").
			Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestCompletionHelp_PerShell(t *testing.T) {
	t.Parallel()

	for _, shell := range []string{"bash", "zsh", "fish", "powershell", "elvish", "nushell"} {
		t.Run(shell+" enable instructions", func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer

			app := conga.New[StructCompletionCLI]().
				EnableCompletion().
				Name("myapp").
				Stdout(&buffer).
				Args([]string{"completion", shell, "--help"})

			err := app.Run()

			assert.NoError(t, err)

			output := buffer.String()
			assert.Contains(t, output, "Generate the shell completion script for the "+shell+" shell.")
			assert.Contains(t, output, "To load completions in your current ")
			assert.Contains(t, output, "myapp completion "+shell)

			// The supplied shell is pinned in the usage synopsis and the
			// Arguments section is dropped as noise.
			assert.Contains(t, output, "Usage:\n  myapp completion "+shell)
			assert.NotContains(t, output, "myapp completion <shell>")
			assert.NotContains(t, output, "Arguments:")

			// Negative invariant: the per-shell screen focuses on one shell.
			assert.NotContains(t, output, "Supported shells:")
		})
	}

	t.Run("zsh persistence variants", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"completion", "zsh", "--help"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "echo 'source <(myapp completion zsh)' >> ~/.zshrc")
		assert.Contains(t, output, "myapp completion zsh > ~/.zfunc/_myapp")
		assert.Contains(t, output, "echo 'fpath=(~/.zfunc $fpath)' >> ~/.zshrc")
	})

	t.Run("zsh quick persist avoids autoload", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"completion", "zsh", "--help"})

		err := app.Run()

		assert.NoError(t, err)

		// The quick persistence line must not recommend autoload; compinit is
		// only referenced in the conditional note.
		for line := range strings.SplitSeq(buffer.String(), "\n") {
			if strings.HasPrefix(line, "\techo 'source") {
				assert.NotContains(t, line, "autoload")
			}
		}
	})

	t.Run("bash note absent when no old bash detected", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[StructCompletionCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"completion", "bash", "--help"})

		err := app.Run()

		assert.NoError(t, err)

		assert.NotContains(t, buffer.String(), "Bash completion requires bash 4.4+")
	})
}

func TestCompletionHelp_Generic(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[StructCompletionCLI]().
		EnableCompletion().
		Name("myapp").
		Stdout(&buffer).
		Args([]string{"completion", "--help"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()
	assert.Contains(t, output, "Generate shell completion scripts")
	assert.Contains(t, output, "Target shell name")
	assert.Contains(t, output, "Supported shells:")
	assert.Contains(t, output, "bash")
	assert.Contains(t, output, "zsh")

	// Negative invariant: the generic completion screen inherits no root flags.
	assert.NotContains(t, output, "Global Flags")
	assert.NotContains(t, output, "--verbose")
}

func TestCompletionFlag_FlatCLI(t *testing.T) {
	t.Parallel()

	t.Run("bare completion flag renders generic completion help", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "Generate shell completion script")
		assert.Contains(t, output, "Usage:\n  myapp --completion [shell]")
		assert.Contains(t, output, "Supported shells:")
		assert.Contains(t, output, "bash")
		assert.Contains(t, output, "zsh")
		assert.NotContains(t, output, "flag requires an argument")
	})

	t.Run("bare completion flag with help renders generic completion help", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion", "--help"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "Usage:\n  myapp --completion [shell]")
		assert.Contains(t, output, "Supported shells:")
	})

	t.Run("completion flag with shell and help renders per-shell help", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion", "zsh", "--help"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "Generate the shell completion script for the zsh shell.")
		assert.Contains(t, output, "Usage:\n  myapp --completion zsh")
		assert.Contains(t, output, "source <(myapp --completion zsh)")
		assert.Contains(t, output, "echo 'source <(myapp --completion zsh)' >> ~/.zshrc")
		assert.Contains(t, output, "myapp --completion zsh > ~/.zfunc/_myapp")
		assert.Contains(t, output, "--help")
		assert.NotContains(t, output, "--why")
		assert.NotContains(t, output, "Supported shells:")
	})

	t.Run("attached completion flag with help renders per-shell help", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion=zsh", "--help"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "Generate the shell completion script for the zsh shell.")
		assert.Contains(t, output, "Usage:\n  myapp --completion zsh")
		assert.Contains(t, output, "source <(myapp --completion zsh)")
	})

	t.Run("completion flag with valid shell outputs completion script", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion", "zsh"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "#compdef myapp")
		assert.Contains(t, output, "_myapp_completion")
	})

	t.Run("attached completion flag with valid shell outputs completion script", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion=zsh"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "#compdef myapp")
	})

	t.Run("repeated completion flag selects last value", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion=bash", "--completion=zsh"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "#compdef myapp")
	})

	t.Run("separated completion flag predicts supported shells", func(t *testing.T) {
		t.Parallel()

		app := conga.New[PlainAppCLI]().
			EnableCompletion()

		invoked, formatted := app.CompleteForShell("fish", []string{"--completion", ""})

		assert.Equal(t, len(completion.SupportedShells()), len(invoked.Items))

		for _, shell := range completion.SupportedShells() {
			assert.True(t, slices.ContainsFunc(invoked.Items, func(item predict.Item) bool { return item.Value == shell }), "missing shell candidate: "+shell)
		}

		assert.True(t, strings.Contains(formatted, "bash\tBourne Again SHell"))
		assert.True(t, strings.Contains(formatted, "zsh\tZ shell"))
	})

	t.Run("separated completion flag value filters shells by prefix", func(t *testing.T) {
		t.Parallel()

		app := conga.New[PlainAppCLI]().
			EnableCompletion()

		invoked, formatted := app.CompleteForShell("fish", []string{"--completion", "z"})

		assert.Equal(t, 1, len(invoked.Items))
		assert.Equal(t, "zsh", invoked.Items[0].Value)
		assert.Equal(t, "zsh\tZ shell", formatted)
	})

	t.Run("attached completion flag predicts supported shells", func(t *testing.T) {
		t.Parallel()

		app := conga.New[PlainAppCLI]().
			EnableCompletion()

		invoked, _ := app.CompleteForShell("fish", []string{"--completion=z"})

		assert.Equal(t, 1, len(invoked.Items))
		assert.Equal(t, "--completion=zsh", invoked.Items[0].Value)
	})

	t.Run("completion flag with invalid shell returns exit code 2", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			}).
			Args([]string{"--completion", "invalidshell"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Contains(t, buffer.String(), "unknown shell \"invalidshell\"")
	})

	t.Run("completion flag with invalid shell and help returns exit code 2", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[PlainAppCLI]().
			EnableCompletion().
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			}).
			Args([]string{"--completion", "invalidshell", "--help"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Contains(t, buffer.String(), "unknown shell \"invalidshell\"")
	})
}

func TestCompletion_FirstPositionalPathPreserved(t *testing.T) {
	t.Parallel()

	shells := []string{"bash", "zsh", "fish", "powershell"}
	firstArgs := []string{"/x/one", "./rel/path", "tool.exe", "myapp"}

	for _, shell := range shells {
		for _, firstArg := range firstArgs {
			t.Run(shell+"_"+firstArg, func(t *testing.T) {
				t.Parallel()

				var buffer bytes.Buffer

				app := conga.New[TwoPositionalArgsCLI]().
					EnableCompletion().
					Name("myapp").
					Stdout(&buffer).
					Args([]string{"__complete", shell, firstArg, "r"})

				err := app.Run()

				assert.NoError(t, err)
				assert.Contains(t, buffer.String(), "red")
			})
		}
	}
}
