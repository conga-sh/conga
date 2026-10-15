package completion_test

import (
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/sentinel"
	"github.com/conga-sh/conga/predict"
)

// fallbackFormatShells lists the shell identifiers that render in the export
// framing: the registry-narrowed legacy shells and unknown or empty identifiers
// (docs/completion-wire-protocol.md).
func fallbackFormatShells() []string {
	return []string{
		"oil",
		"xonsh",
		"unknown-shell",
		"",
	}
}

func plainResult() predict.Result {
	return predict.Result{
		Items: []predict.Item{
			{Value: "deploy", Description: "Deploy service", Style: "blue", Tag: "commands"},
			{Value: "logs", Display: "logs"},
		},
		Directive: predict.DirectiveNoFileCompletion,
	}
}

func errorResult() predict.Result {
	return predict.Result{
		Directive: predict.DirectiveError,
		Usage:     "usage: app <command>",
	}
}

func TestFormatFallbackShellsRenderExportFraming(t *testing.T) {
	t.Parallel()

	for _, shell := range fallbackFormatShells() {
		t.Run("plain result/"+shell, func(t *testing.T) {
			t.Parallel()

			formatted := completion.NewSession(shell).Format("de", plainResult())
			expected := completion.NewSession("export").Format("de", plainResult())

			assert.Equal(t, expected, formatted)
		})
	}
}

func TestFormatFallbackShellsRenderErrorResultsInExportFraming(t *testing.T) {
	t.Parallel()

	for _, shell := range fallbackFormatShells() {
		t.Run("error result/"+shell, func(t *testing.T) {
			t.Parallel()

			formatted := completion.NewSession(shell).Format("", errorResult())
			expected := completion.NewSession("export").Format("", errorResult())

			assert.Equal(t, expected, formatted)
		})
	}
}

func TestFormatNativeFish(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("fish").Format("de", plainResult())

	assert.Equal(t, "deploy\tDeploy service\nlogs", formatted)
}

func TestFormatNativeBash(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("bash").Format("de", plainResult())

	assert.Equal(t, "D\t\nC\tdeploy\nC\tlogs", formatted)
}

func TestFormatNativeZsh(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	formatted := completion.NewSession("zsh").Format("de", plainResult())

	assert.Equal(t,
		"D\x1f\x1f\n"+
			"C\x1fcommands\x1fdeploy\x1fDeploy service\x1fdeploy \x1f\n"+
			"C\x1f\x1flogs\x1f\x1flogs \x1f",
		formatted)
}

func TestFormatNativeZshErrorResult(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	formatted := completion.NewSession("zsh").Format("", errorResult())

	assert.Equal(t, "D\x1f\x1fusage: app <command>\nM\x1fusage: app <command>", formatted)
}

func TestFormatNativeBashErrorResult(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("bash").Format("", errorResult())

	assert.Equal(t, "D\t\nM\tusage: app <command>", formatted)
}

func TestFormatNativeFishErrorResult(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("fish").Format("", errorResult())

	assert.Equal(t, "", formatted)
}

func TestFormatNativeExport(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("export").Format("", plainResult())

	assert.Equal(t,
		`{"candidates":[{"value":"deploy","display":"deploy","description":"Deploy service","style":"blue","tag":"commands"},{"value":"logs","display":"logs"}],`+
			`"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":""}`,
		formatted)
}

func TestFormatNativeExportErrorResult(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("export").Format("", errorResult())

	assert.Equal(t,
		`{"candidates":[],"directives":{"keeporder":false,"messages":["usage: app <command>"],"noprefix":false,"nospace":false},"usage":"usage: app <command>"}`,
		formatted)
}

func TestPatchPassesThroughNonBashShells(t *testing.T) {
	t.Parallel()

	args := []string{"cloudctl", "deploy", "--env=pr", ">", "/tmp/out.log"}

	for _, shell := range fallbackFormatShells() {
		t.Run(shell, func(t *testing.T) {
			t.Parallel()

			patched, err := completion.NewSession(shell).Patch(args)

			assert.NoError(t, err)
			assert.Equal(t, args, patched)
		})
	}
}

func bashSessionForLine(line string) *completion.Session {
	return completion.NewSessionWithEnv("bash", func(key string) (string, bool) {
		switch key {
		case "COMP_LINE":
			return line, true
		case "COMP_POINT":
			return strconv.Itoa(len(line)), true
		default:
			return "", false
		}
	})
}

func TestPatchNativeBash(t *testing.T) {
	t.Parallel()

	args := []string{"cloudctl", "deploy", ">", "/tmp/out.log", "--env", ""}

	t.Run("redirect target and trailing word preserved", func(t *testing.T) {
		t.Parallel()

		line := "cloudctl deploy > /tmp/out.log --env "
		session := bashSessionForLine(line)

		patched, err := session.Patch(args)

		assert.NoError(t, err)
		assert.Equal(t, []string{"deploy", "--env", ""}, patched)
	})

	t.Run("pipeline isolation", func(t *testing.T) {
		t.Parallel()

		line := "echo foo | cloudctl dep"
		session := bashSessionForLine(line)

		patched, err := session.Patch([]string{"foo", "|", "cloudctl", "dep"})

		assert.NoError(t, err)
		assert.Equal(t, []string{"dep"}, patched)
	})

	t.Run("redirect position rejected", func(t *testing.T) {
		t.Parallel()

		line := "cloudctl deploy > "
		session := bashSessionForLine(line)

		patched, err := session.Patch([]string{"deploy", ">", ""})

		assert.Error(t, err)
		assert.IsError(t, err, sentinel.ErrRedirectTarget)
		assert.Zero(t, patched)
	})

	t.Run("without environment passes through", func(t *testing.T) {
		t.Parallel()

		patched, err := completion.NewSession("bash").Patch(args)

		assert.NoError(t, err)
		assert.Equal(t, args, patched)
	})
}

func TestSnippetRejectsUnsupportedShells(t *testing.T) {
	t.Parallel()

	opts := completion.DriverOptions{CommandName: "myapp", Aliases: []string{"ma", "app"}}

	for _, shell := range fallbackFormatShells() {
		t.Run(shell, func(t *testing.T) {
			t.Parallel()

			snippet, err := completion.Snippet(opts, shell)

			assert.Error(t, err)
			assert.Equal(t, "", snippet)
			assert.Contains(t, err.Error(), "unknown shell")
			assert.Contains(t, err.Error(), "supported shells: bash, elvish, fish, nushell, powershell, zsh")
		})
	}
}

func TestSnippetNativeFish(t *testing.T) {
	t.Parallel()

	opts := completion.DriverOptions{CommandName: "myapp", Aliases: []string{"ma", "app"}}

	snippet, err := completion.Snippet(opts, "fish")

	assert.NoError(t, err)
	assert.Equal(t,
		`function _myapp_completion`+"\n"+
			`  set --local words (commandline -opc)`+"\n"+
			`  if test (count $words) -gt 0`+"\n"+
			`    set --local current (commandline -ct)`+"\n"+
			`    $words[1] __complete fish $words[2..-1] $current 2>/dev/null`+"\n"+
			`  end`+"\n"+
			`end`+"\n"+
			"\n"+
			`complete -e 'myapp'`+"\n"+
			`complete -c 'myapp' -f -a '(_myapp_completion)' -r`+"\n"+
			"\n"+
			`complete -e 'ma'`+"\n"+
			`complete -c 'ma' -f -a '(_myapp_completion)' -r`+"\n"+
			"\n"+
			`complete -e 'app'`+"\n"+
			`complete -c 'app' -f -a '(_myapp_completion)' -r`+"\n",
		snippet)
}

func TestSnippetNativeBash(t *testing.T) {
	t.Parallel()

	opts := completion.DriverOptions{CommandName: "myapp", Aliases: []string{"ma", "app"}}

	snippet, err := completion.Snippet(opts, "bash")

	assert.NoError(t, err)
	assert.Contains(t, snippet, "_myapp_completion() {")
	assert.Contains(t, snippet, `xargs -- "$cmd" __complete bash 2>/dev/null`)
	assert.Contains(t, snippet, "compopt -o nospace")
	assert.Contains(t, snippet, "complete -o noquote -F _myapp_completion myapp ma app")
}

func TestFormatNativeElvish(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("elvish").Format("de", plainResult())

	assert.Equal(t,
		`{"usage":"","messages":[],"descriptionStyle":"dim",`+
			`"candidates":[{"value":"deploy","display":"deploy","description":"Deploy service","codeSuffix":" ","style":"blue"},{"value":"logs","display":"logs","description":"","codeSuffix":" ","style":"default"}]}`,
		formatted)
}

func TestFormatNativeElvishErrorResult(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("elvish").Format("", errorResult())

	assert.Equal(t,
		`{"usage":"usage: app <command>","messages":["usage: app <command>"],"descriptionStyle":"dim","candidates":[]}`,
		formatted)
}

func TestSnippetNativeElvish(t *testing.T) {
	t.Parallel()

	opts := completion.DriverOptions{CommandName: "myapp", Aliases: []string{"ma", "app"}}

	snippet, err := completion.Snippet(opts, "elvish")

	assert.NoError(t, err)
	assert.Contains(t, snippet, "set cmd = (search-external $args[0])")
	assert.Contains(t, snippet, "var completion = ($cmd __complete elvish (all $args[1..]) 2>/dev/null | from-json)")
	assert.Contains(t, snippet, "set edit:completion:arg-completer[myapp] = $completer~")
	assert.Contains(t, snippet, "set edit:completion:arg-completer[ma] = $completer~")
}

func TestFormatNativeNushell(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("nushell").Format("", plainResult())

	assert.Equal(t,
		`[{"value":"deploy ","display":"deploy","description":"Deploy service","style":{"fg":"blue"}},{"value":"logs ","display":"logs"}]`,
		formatted)
}

func TestFormatNativeNushellErrorResult(t *testing.T) {
	t.Parallel()

	formatted := completion.NewSession("nushell").Format("", errorResult())

	// Nushell's external-completer contract has no message/usage channel.
	assert.Equal(t, `[]`, formatted)
}

func TestFormatNativeNushellQuoting(t *testing.T) {
	t.Parallel()

	result := predict.Result{
		Items: []predict.Item{
			{Value: "my file.txt", Display: "my file.txt"},
			{Value: "~/config", Display: "~/config"},
			{Value: `a"b\c`, Display: `a"b\c`},
			{Value: "dir/", Display: "dir/"},
			{Value: "styled", Display: "styled", Style: "bright-red bold"},
			{Value: "indexed", Display: "indexed", Style: "color123"},
			{Value: "hex", Display: "hex", Style: "#ff8800"},
		},
	}

	formatted := completion.NewSession("nushell").Format("", result)

	assert.Equal(t,
		`[{"value":"\"my file.txt\" ","display":"my file.txt"}`+
			`,{"value":"~/config ","display":"~/config"}`+
			`,{"value":"\"a\\\"b\\\\c\" ","display":"a\"b\\c"}`+
			`,{"value":"dir/","display":"dir/"}`+
			`,{"value":"styled ","display":"styled","style":{"fg":"light_red","attr":"b"}}`+
			`,{"value":"indexed ","display":"indexed","style":{"fg":"#87ffff"}}`+
			`,{"value":"hex ","display":"hex","style":{"fg":"#ff8800"}}]`,
		formatted)
}

func TestPatchNativeNushell(t *testing.T) {
	t.Parallel()

	patched, err := completion.NewSession("nushell").Patch([]string{`"double quoted"`, `'single quoted'`, "`backtick`", "plain", "", `"partial"word`, `"unclosed`})

	assert.NoError(t, err)
	assert.Equal(t, []string{"double quoted", "single quoted", "backtick", "plain", "", "partialword", `"unclosed`}, patched)
}

func TestSnippetNativeNushell(t *testing.T) {
	t.Parallel()

	opts := completion.DriverOptions{CommandName: "myapp", Aliases: []string{"ma", "app"}}

	snippet, err := completion.Snippet(opts, "nushell")

	assert.NoError(t, err)
	assert.Contains(t, snippet, "let myapp_completer = {|spans|")
	assert.Contains(t, snippet, "^$spans.0 __complete nushell ...$words")
	assert.Contains(t, snippet, "load-env { CONGA_SHELL: 'nushell' }")
	assert.Contains(t, snippet, "upsert completer { if $in == null { $myapp_completer } else { $in } })")
	assert.Contains(t, snippet, "nushell #15654")
}

func TestFormatNativePowerShell(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	formatted := completion.NewSession("powershell").Format("", plainResult())

	assert.Equal(t,
		`{"candidates":[{"value":"deploy ","display":"deploy (Deploy service)","description":" ","style":"blue","tag":"commands"},{"value":"logs ","display":"logs","description":" "}],`+
			`"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":""}`,
		formatted)
}

func TestFormatNativePowerShellErrorResult(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	formatted := completion.NewSession("powershell").Format("", errorResult())

	assert.Equal(t,
		`{"candidates":[],"directives":{"keeporder":false,"messages":["usage: app <command>"],"noprefix":false,"nospace":false},"usage":"usage: app <command>"}`,
		formatted)
}

func TestSnippetNativePowerShell(t *testing.T) {
	t.Parallel()

	opts := completion.DriverOptions{CommandName: "myapp", Aliases: []string{"ma", "app"}}

	snippet, err := completion.Snippet(opts, "powershell")

	assert.NoError(t, err)
	assert.Contains(t, snippet, "$_myapp_completer = {")
	assert.Contains(t, snippet, "__complete powershell")
	assert.Contains(t, snippet, "Register-ArgumentCompleter -Native -ScriptBlock $_myapp_completer -CommandName 'ma' # 'ma.exe'")
}

func TestSnippetNativeZsh(t *testing.T) {
	t.Parallel()

	opts := completion.DriverOptions{CommandName: "myapp", Aliases: []string{"ma", "app"}}

	snippet, err := completion.Snippet(opts, "zsh")

	assert.NoError(t, err)
	assert.Contains(t, snippet, "#compdef myapp ma app")
	assert.Contains(t, snippet, "function _myapp_completer {")
	assert.Contains(t, snippet, `"$target" __complete zsh "${args[@]}" 2>/dev/null`)
	assert.Contains(t, snippet, "compdef _myapp_completer myapp ma app")
}

func TestNativeSupportedShells(t *testing.T) {
	t.Parallel()

	shells := completion.SupportedShells()

	assert.Equal(t, []string{"bash", "elvish", "fish", "nushell", "powershell", "zsh"}, shells)
}
