//go:build windows

package signals

import (
	"syscall"

	"github.com/conga-sh/conga/internal/style"
)

func known() []Entry {
	return []Entry{
		{Name: "SIGHUP", ShortName: "HUP", Signal: syscall.SIGHUP, Description: "Hangup detected on controlling terminal", Style: style.Yellow},
		{Name: "SIGINT", ShortName: "INT", Signal: syscall.SIGINT, Description: "Interrupt from keyboard (Ctrl+C)", Style: style.Yellow},
		{Name: "SIGQUIT", ShortName: "QUIT", Signal: syscall.SIGQUIT, Description: "Quit from keyboard (Ctrl+\\)", Style: style.Red},
		{Name: "SIGILL", ShortName: "ILL", Signal: syscall.SIGILL, Description: "Illegal instruction", Style: style.Red},
		{Name: "SIGTRAP", ShortName: "TRAP", Signal: syscall.SIGTRAP, Description: "Trace/breakpoint trap", Style: style.Yellow},
		{Name: "SIGABRT", ShortName: "ABRT", Signal: syscall.SIGABRT, Description: "Abort signal from abort()", Style: style.Red},
		{Name: "SIGBUS", ShortName: "BUS", Signal: syscall.SIGBUS, Description: "Bus error (bad memory access)", Style: style.Red},
		{Name: "SIGFPE", ShortName: "FPE", Signal: syscall.SIGFPE, Description: "Floating-point exception", Style: style.Red},
		{Name: "SIGKILL", ShortName: "KILL", Signal: syscall.SIGKILL, Description: "Kill signal (cannot be caught or ignored)", Style: style.Red},
		{Name: "SIGSEGV", ShortName: "SEGV", Signal: syscall.SIGSEGV, Description: "Invalid memory reference (segmentation fault)", Style: style.Red},
		{Name: "SIGPIPE", ShortName: "PIPE", Signal: syscall.SIGPIPE, Description: "Broken pipe: write to pipe with no readers", Style: style.Yellow},
		{Name: "SIGALRM", ShortName: "ALRM", Signal: syscall.SIGALRM, Description: "Timer signal from alarm()", Style: style.Yellow},
		{Name: "SIGTERM", ShortName: "TERM", Signal: syscall.SIGTERM, Description: "Termination signal (graceful shutdown request)", Style: style.Yellow},
	}
}
