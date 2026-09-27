//nolint:testpackage // white-box: exercises the unexported shell-from-path classifier
package completion

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestShellFromExecPath_WindowsNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		execPath string
		want     Shell
	}{
		{name: "bare powershell.exe", execPath: "powershell.exe", want: ShellPowerShell},
		{name: "bare pwsh.exe", execPath: "pwsh.exe", want: ShellPowerShell},
		{name: "windows powershell system path", execPath: `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, want: ShellPowerShell},
		{name: "powershell core program files path", execPath: `C:\Program Files\PowerShell\7\pwsh.exe`, want: ShellPowerShell},
		{name: "bare nu.exe", execPath: "nu.exe", want: ShellNushell},
		{name: "bare nushell.exe", execPath: "nushell.exe", want: ShellNushell},
		{name: "nushell user bin path", execPath: `C:\Users\username\bin\nu.exe`, want: ShellNushell},
		{name: "git bash full path", execPath: `C:\Program Files\Git\bin\bash.exe`, want: ShellBash},
		{name: "bare bash.exe", execPath: "bash.exe", want: ShellBash},
		{name: "bare nu name", execPath: "nu", want: ShellNushell},
		{name: "bare pwsh name", execPath: "pwsh", want: ShellPowerShell},
		{name: "cmd.exe unsupported", execPath: "cmd.exe", want: ""},
		{name: "cmd system path unsupported", execPath: `C:\Windows\System32\cmd.exe`, want: ""},
		{name: "explorer.exe unsupported", execPath: `C:\Windows\explorer.exe`, want: ""},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := shellFromExecPath(test.execPath)

			assert.Equal(t, test.want, got)
		})
	}
}
