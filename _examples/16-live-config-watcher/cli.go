package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/watch"
)

// CLI demonstrates live configuration file watching, path expansion, and daemon hot-reloading.
type CLI struct {
	ConfigFile conga.ConfigFile               `help:"Config file path" multi:"true"                                   short:"c"`
	Port       conga.OptionalFlag[int]        `default:"8080"          help:"Server port"                             short:"p"`
	RateLimit  conga.OptionalFlag[int]        `default:"100"           help:"Rate limit per second"`
	Debug      conga.OptionalFlag[bool]       `default:"false"         help:"Enable debug logging"`
	Blocklist  conga.OptionalFlag[conga.File] `existing:"true"         help:"Blocklist file watched for live updates"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	state := &daemonState{}
	state.currentConfig.Store(c)

	blocklistFlag := c.Blocklist.Get()

	blocklistPath := blocklistFlag.String()
	if blocklistPath != "" {
		entries := countFileLines(blocklistPath)
		state.blockEntries.Store(entries)
		fmt.Fprintf(session.Stdout(), "[WATCHER] Blocklist loaded: %d entries\n", entries)
	}

	server := newHTTPServer(ctx, c, state)

	watchErrChan := make(chan error, 1)

	printWatcherStatus(session)

	startConfigWatcher(ctx, session, state, watchErrChan)

	// The blocklist is not part of the configuration cascade, so it gets its
	// own single-file watch instead of the config watcher. It only reports
	// watcher errors; clean shutdown sends nothing on watchErrChan.
	if blocklistPath != "" {
		startBlocklistWatcher(ctx, session, blocklistPath, state, watchErrChan)
	}

	startSighupReloader(ctx, session, state)

	serverErrChan := make(chan error, 1)

	go func() {
		fmt.Fprintf(session.Stdout(), "[SERVER] HTTP server listening on http://localhost:%d\n", c.Port.Get())

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrChan <- fmt.Errorf("http server: %w", err)

			return
		}

		serverErrChan <- nil
	}()

	select {
	case <-ctx.Done():
		fmt.Fprintln(session.Stdout(), "[WATCHER] Shutdown signal received, shutting down gracefully...")
	case err := <-serverErrChan:
		if err != nil {
			return err
		}
	case err := <-watchErrChan:
		if err != nil {
			return err
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http server shutdown: %w", err)
	}

	<-watchErrChan

	fmt.Fprintln(session.Stdout(), "[WATCHER] Daemon shutting down gracefully")

	return nil
}

type daemonState struct {
	currentConfig atomic.Pointer[CLI]
	reloadCount   atomic.Int64
	blockEntries  atomic.Int64
}

// reloadOnSighup re-reads the active configuration and swaps it in.
func (state *daemonState) reloadOnSighup(ctx context.Context, session *conga.Session) {
	reloaded, err := session.Reload[CLI](ctx)
	if err != nil {
		fmt.Fprintf(session.Stdout(), "[WATCHER] SIGHUP reload error: %v\n", err)

		return
	}

	previousConfig := state.currentConfig.Swap(reloaded)
	state.reloadCount.Add(1)
	fmt.Fprintf(session.Stdout(), "[WATCHER] SIGHUP reload successful! RateLimit: %d -> %d\n",
		previousConfig.RateLimit.Get(), reloaded.RateLimit.Get())
}

// newHTTPServer builds the demo HTTP server serving the live configuration.
func newHTTPServer(ctx context.Context, application *CLI, state *daemonState) *http.Server {
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/config", func(writer http.ResponseWriter, request *http.Request) {
		config := state.currentConfig.Load()

		writer.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(writer).Encode(ConfigResponse{
			Port:             config.Port.Get(),
			RateLimit:        config.RateLimit.Get(),
			Debug:            config.Debug.Get(),
			BlocklistEntries: state.blockEntries.Load(),
			Reloads:          state.reloadCount.Load(),
		}); err != nil {
			http.Error(writer, fmt.Sprintf("encode config response: %v", err), http.StatusInternalServerError)
		}
	})

	return &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", application.Port.Get()),
		Handler:           serveMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// startConfigWatcher hot-reloads the configuration on filesystem changes via
// watch.Config (which internally drives session.Reload).
func startConfigWatcher(ctx context.Context, session *conga.Session, state *daemonState, watchErrChan chan<- error) {
	go func() {
		err := watch.Config[CLI](ctx, session, func(reloaded *CLI, err error) {
			if err != nil {
				fmt.Fprintf(session.Stdout(), "[WATCHER] Config reload error: %v\n", err)

				return
			}

			previousConfig := state.currentConfig.Swap(reloaded)
			state.reloadCount.Add(1)
			fmt.Fprintf(session.Stdout(), "[WATCHER] Live hot-reload successful! RateLimit: %d -> %d | Debug: %t -> %t\n",
				previousConfig.RateLimit.Get(), reloaded.RateLimit.Get(),
				previousConfig.Debug.Get(), reloaded.Debug.Get())
		})

		if err != nil && !errors.Is(err, context.Canceled) {
			watchErrChan <- fmt.Errorf("file watcher: %w", err)

			return
		}

		watchErrChan <- nil
	}()
}

// startBlocklistWatcher monitors the auxiliary blocklist file via watch.File.
func startBlocklistWatcher(ctx context.Context, session *conga.Session, blocklistPath string, state *daemonState, watchErrChan chan<- error) {
	go func() {
		err := watch.File(ctx, blocklistPath, func(watchErr error) {
			if watchErr != nil {
				fmt.Fprintf(session.Stdout(), "[WATCHER] Blocklist watch error: %v\n", watchErr)

				return
			}

			entries := countFileLines(blocklistPath)
			state.blockEntries.Store(entries)
			fmt.Fprintf(session.Stdout(), "[WATCHER] Blocklist reloaded: %d entries\n", entries)
		})

		if err != nil && !errors.Is(err, context.Canceled) {
			watchErrChan <- fmt.Errorf("blocklist watcher: %w", err)
		}
	}()
}

// startSighupReloader triggers a synchronous session.Reload on SIGHUP: the
// classic manual daemon re-read, independent of the filesystem watcher.
func startSighupReloader(ctx context.Context, session *conga.Session, state *daemonState) {
	sighupChan := make(chan os.Signal, 1)
	signal.Notify(sighupChan, syscall.SIGHUP)

	go func() {
		defer signal.Stop(sighupChan)

		for {
			select {
			case <-ctx.Done():
				return
			case <-sighupChan:
				state.reloadOnSighup(ctx, session)
			}
		}
	}()
}
