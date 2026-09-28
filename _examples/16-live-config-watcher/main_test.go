package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"watcher-app": main,
	})
	os.Exit(m.Run())
}

func TestScripts(t *testing.T) {
	t.Parallel()
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"http-wait": httpWaitCmd,
			"interrupt": interruptCmd,
			"hup":       hupCmd,
		},
	})
}

func sendSignal(process *os.Process, signal os.Signal) error {
	if err := process.Signal(signal); err != nil {
		return fmt.Errorf("signal %v: %w", signal, err)
	}

	return nil
}

// httpWaitCmd polls a URL until its body matches a regular expression. This
// removes the timing race between a config file change, the watcher debounce,
// and the HTTP handler serving the reloaded value.
//
// Usage: http-wait <url> <pattern>.
func httpWaitCmd(testScript *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		testScript.Fatalf("usage: http-wait <url> <pattern>")
	}

	pattern, err := regexp.Compile(args[1])
	if err != nil {
		testScript.Fatalf("invalid pattern %q: %v", args[1], err)
	}

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		body, _, err := fetchWithRetry(args[0])
		if err == nil && pattern.Match(body) {
			if neg {
				testScript.Fatalf("http-wait %s unexpectedly matched %q", args[0], args[1])
			}

			testScript.Stdout().Write(body)
			testScript.Stdout().Write([]byte("\n"))

			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	if neg {
		return
	}

	testScript.Fatalf("http-wait %s never matched %q within 5s", args[0], args[1])
}

func fetchWithRetry(url string) ([]byte, int, error) {
	client := &http.Client{Timeout: 2 * time.Second}

	var (
		response *http.Response
		err      error
	)

	for range 20 {
		var request *http.Request

		request, err = http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
		if err != nil {
			return nil, 0, fmt.Errorf("build request for %s: %w", url, err)
		}

		response, err = client.Do(request)
		if err == nil {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s: %w", url, err)
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("read response body from %s: %w", url, err)
	}

	return body, response.StatusCode, nil
}

// interruptCmd sends SIGINT to all background commands to trigger graceful shutdown.
func interruptCmd(testScript *testscript.TestScript, _ bool, _ []string) {
	commands := testScript.BackgroundCmds()
	if len(commands) == 0 {
		testScript.Fatalf("no background commands to interrupt")
	}

	for _, command := range commands {
		if command.Process != nil {
			if err := sendSignal(command.Process, os.Interrupt); err != nil {
				testScript.Fatalf("failed to interrupt pid %d: %v", command.Process.Pid, err)
			}
		}
	}
}

// hupCmd sends SIGHUP to all background commands to trigger a manual session.Reload.
func hupCmd(testScript *testscript.TestScript, _ bool, _ []string) {
	commands := testScript.BackgroundCmds()
	if len(commands) == 0 {
		testScript.Fatalf("no background commands to signal")
	}

	for _, command := range commands {
		if command.Process != nil {
			if err := sendSignal(command.Process, syscall.SIGHUP); err != nil {
				testScript.Fatalf("failed to signal pid %d: %v", command.Process.Pid, err)
			}
		}
	}
}
