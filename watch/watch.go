// Package watch provides live filesystem monitoring and automatic
// configuration reloading for CONGA applications.
//
// [Config] watches the execution's candidate configuration files and delivers
// a freshly reloaded, validated configuration to its callback; [File] and
// [Paths] watch arbitrary paths and report change notifications. Each call runs
// its watch loop until the context is cancelled, debouncing rapid filesystem
// events, and panics on a nil context. [Config] returns [ErrNoCandidates] when
// the candidate set is empty and an error when the session is nil.
package watch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/conga-sh/conga"
)

// ErrNoCandidates indicates no candidate configuration files were found to watch.
var ErrNoCandidates = fmt.Errorf("%w: no candidate configuration files to watch", conga.ErrConfig)

// errNilSession indicates a watch function was invoked without an execution session.
var errNilSession = errors.New("watch: session cannot be nil")

// Config watches all candidate configuration files and their parent directories for changes,
// calling onChange with a freshly reloaded *T when a file is created, modified, or deleted.
// Panics if ctx is nil; returns an error if session is nil.
func Config[T any](ctx context.Context, session *conga.Session, onChange func(newCfg *T, err error)) error {
	if ctx == nil {
		panic("watch: nil Context")
	}

	if session == nil {
		return errNilSession
	}

	candidates := session.CandidateConfigFiles()
	if len(candidates) == 0 {
		return ErrNoCandidates
	}

	if err := Paths(ctx, candidates, func(watchErr error) {
		if watchErr != nil {
			if onChange != nil {
				onChange(nil, watchErr)
			}

			return
		}

		reloaded, err := session.Reload[T](ctx)
		if onChange != nil {
			onChange(reloaded, err)
		}
	}); err != nil {
		return fmt.Errorf("watch candidate paths: %w", err)
	}

	return nil
}

// File monitors a single configuration file for filesystem changes.
// When the file is created, written, removed, or renamed, it debounces rapid events and invokes onChange.
// File runs until ctx is cancelled or an unrecoverable watcher error occurs.
// Panics if ctx is nil.
func File(ctx context.Context, filePath string, onChange func(err error)) error {
	return Paths(ctx, []string{filePath}, onChange)
}

// Paths monitors multiple configuration files/paths and their parent directories for filesystem changes.
// When any monitored file is created, written, removed, or renamed, it debounces rapid events and invokes onChange.
// Paths runs until ctx is cancelled or an unrecoverable watcher error occurs.
// Panics if ctx is nil.
func Paths(ctx context.Context, filePaths []string, onChange func(err error)) (retErr error) {
	if ctx == nil {
		panic("watch: nil Context")
	}

	if len(filePaths) == 0 {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create file watcher: %w", err)
	}

	defer func() {
		if closeErr := watcher.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("close watcher: %w", closeErr)
		}
	}()

	monitoredFiles, targetDirs, watchedDirs, err := registerInitialTargets(watcher, filePaths)
	if err != nil {
		return err
	}

	return runWatchLoop(ctx, watcher, monitoredFiles, targetDirs, watchedDirs, onChange)
}

type watchDebouncer struct {
	mu       sync.Mutex
	timer    *time.Timer
	duration time.Duration
	action   func(error)
}

func newWatchDebouncer(duration time.Duration, action func(error)) *watchDebouncer {
	return &watchDebouncer{
		duration: duration,
		action:   action,
	}
}

func (d *watchDebouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}
}

func (d *watchDebouncer) trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.duration, func() {
		if d.action != nil {
			d.action(nil)
		}
	})
}

func isRelevantEvent(event fsnotify.Event) bool {
	return event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove)
}

func runWatchLoop(ctx context.Context, watcher *fsnotify.Watcher, monitoredFiles, targetDirs, watchedDirs map[string]bool, onChange func(err error)) error {
	debouncer := newWatchDebouncer(50*time.Millisecond, onChange)
	defer debouncer.stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Has(fsnotify.Create) {
				refreshTargetDirs(watcher, targetDirs, watchedDirs)
			}

			eventClean := filepath.Clean(event.Name)
			if monitoredFiles[eventClean] && isRelevantEvent(event) {
				debouncer.trigger()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}

			if onChange != nil {
				onChange(err)
			}
		}
	}
}

func watchTargetDir(watcher *fsnotify.Watcher, dir string, watchedDirs map[string]bool) error {
	if watchedDirs[dir] {
		return nil
	}

	if fileInfo, err := os.Stat(dir); err == nil && fileInfo.IsDir() {
		if err := watcher.Add(dir); err != nil {
			return fmt.Errorf("watch directory %q: %w", dir, err)
		}

		watchedDirs[dir] = true

		return nil
	}

	nearest := findNearestExistingDir(dir)
	if nearest == "" || watchedDirs[nearest] {
		return nil
	}

	if err := watcher.Add(nearest); err != nil {
		return fmt.Errorf("watch ancestor directory %q: %w", nearest, err)
	}

	watchedDirs[nearest] = true

	return nil
}

func registerInitialTargets(watcher *fsnotify.Watcher, filePaths []string) (map[string]bool, map[string]bool, map[string]bool, error) {
	watchedDirs := make(map[string]bool)
	monitoredFiles := make(map[string]bool)
	targetDirs := make(map[string]bool)

	for _, p := range filePaths {
		clean := filepath.Clean(p)
		dir := filepath.Dir(clean)
		monitoredFiles[clean] = true
		targetDirs[dir] = true

		if err := watchTargetDir(watcher, dir, watchedDirs); err != nil {
			return nil, nil, nil, err
		}
	}

	return monitoredFiles, targetDirs, watchedDirs, nil
}

func refreshTargetDirs(watcher *fsnotify.Watcher, targetDirs, watchedDirs map[string]bool) {
	for tDir := range targetDirs {
		_ = watchTargetDir(watcher, tDir, watchedDirs) //nolint:errcheck // best-effort directory watch registration
	}
}

func findNearestExistingDir(dir string) string {
	curr := filepath.Clean(dir)
	for {
		if fileInfo, err := os.Stat(curr); err == nil && fileInfo.IsDir() {
			return curr
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			return ""
		}

		curr = parent
	}
}
