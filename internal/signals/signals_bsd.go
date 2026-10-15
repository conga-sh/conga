//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package signals

import (
	"syscall"

	"github.com/conga-sh/conga/internal/style"
)

//nolint:gochecknoglobals // static platform signal table compiled into .rodata
var platformSignals = []Entry{
	{Name: "SIGHUP", ShortName: "HUP", Signal: syscall.SIGHUP, Description: "Hangup detected on controlling terminal", Style: style.Yellow},
	{Name: "SIGINT", ShortName: "INT", Signal: syscall.SIGINT, Description: "Interrupt from keyboard (Ctrl+C)", Style: style.Yellow},
	{Name: "SIGQUIT", ShortName: "QUIT", Signal: syscall.SIGQUIT, Description: "Quit from keyboard (Ctrl+\\)", Style: style.Red},
	{Name: "SIGILL", ShortName: "ILL", Signal: syscall.SIGILL, Description: "Illegal instruction", Style: style.Red},
	{Name: "SIGTRAP", ShortName: "TRAP", Signal: syscall.SIGTRAP, Description: "Trace/breakpoint trap", Style: style.Yellow},
	{Name: "SIGABRT", ShortName: "ABRT", Signal: syscall.SIGABRT, Description: "Abort signal from abort()", Style: style.Red},
	{Name: "SIGBUS", ShortName: "BUS", Signal: syscall.SIGBUS, Description: "Bus error (bad memory access)", Style: style.Red},
	{Name: "SIGFPE", ShortName: "FPE", Signal: syscall.SIGFPE, Description: "Floating-point exception", Style: style.Red},
	{Name: "SIGKILL", ShortName: "KILL", Signal: syscall.SIGKILL, Description: "Kill signal (cannot be caught or ignored)", Style: style.Red},
	{Name: "SIGUSR1", ShortName: "USR1", Signal: syscall.SIGUSR1, Description: "User-defined signal 1", Style: style.Blue},
	{Name: "SIGSEGV", ShortName: "SEGV", Signal: syscall.SIGSEGV, Description: "Invalid memory reference (segmentation fault)", Style: style.Red},
	{Name: "SIGUSR2", ShortName: "USR2", Signal: syscall.SIGUSR2, Description: "User-defined signal 2", Style: style.Blue},
	{Name: "SIGPIPE", ShortName: "PIPE", Signal: syscall.SIGPIPE, Description: "Broken pipe: write to pipe with no readers", Style: style.Yellow},
	{Name: "SIGALRM", ShortName: "ALRM", Signal: syscall.SIGALRM, Description: "Timer signal from alarm()", Style: style.Yellow},
	{Name: "SIGTERM", ShortName: "TERM", Signal: syscall.SIGTERM, Description: "Termination signal (graceful shutdown request)", Style: style.Yellow},
	{Name: "SIGCHLD", ShortName: "CHLD", Signal: syscall.SIGCHLD, Description: "Child stopped or terminated", Style: style.Default},
	{Name: "SIGCONT", ShortName: "CONT", Signal: syscall.SIGCONT, Description: "Continue if stopped", Style: style.Green},
	{Name: "SIGSTOP", ShortName: "STOP", Signal: syscall.SIGSTOP, Description: "Stop process execution (unblockable)", Style: style.Magenta},
	{Name: "SIGTSTP", ShortName: "TSTP", Signal: syscall.SIGTSTP, Description: "Stop typed at terminal (Ctrl+Z)", Style: style.Magenta},
	{Name: "SIGTTIN", ShortName: "TTIN", Signal: syscall.SIGTTIN, Description: "Terminal input for background process", Style: style.Magenta},
	{Name: "SIGTTOU", ShortName: "TTOU", Signal: syscall.SIGTTOU, Description: "Terminal output for background process", Style: style.Magenta},
	{Name: "SIGURG", ShortName: "URG", Signal: syscall.SIGURG, Description: "Urgent condition on socket", Style: style.Blue},
	{Name: "SIGXCPU", ShortName: "XCPU", Signal: syscall.SIGXCPU, Description: "CPU time limit exceeded", Style: style.Red},
	{Name: "SIGXFSZ", ShortName: "XFSZ", Signal: syscall.SIGXFSZ, Description: "File size limit exceeded", Style: style.Red},
	{Name: "SIGVTALRM", ShortName: "VTALRM", Signal: syscall.SIGVTALRM, Description: "Virtual timer expired", Style: style.Yellow},
	{Name: "SIGPROF", ShortName: "PROF", Signal: syscall.SIGPROF, Description: "Profiling timer expired", Style: style.Yellow},
	{Name: "SIGWINCH", ShortName: "WINCH", Signal: syscall.SIGWINCH, Description: "Window resize signal", Style: style.Blue},
	{Name: "SIGIO", ShortName: "IO", Signal: syscall.SIGIO, Description: "I/O now possible", Style: style.Blue},
	{Name: "SIGINFO", ShortName: "INFO", Signal: syscall.SIGINFO, Description: "Information request", Style: style.Blue},
	{Name: "SIGSYS", ShortName: "SYS", Signal: syscall.SIGSYS, Description: "Bad system call", Style: style.Red},
}
