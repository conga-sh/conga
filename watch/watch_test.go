package watch_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/watch"
)

func TestWatch_LiveReload(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(cfgPath, []byte("port = 8080\n"), 0o600)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var (
		stateMu    sync.Mutex
		callCount  int
		changeDone = make(chan struct{}, 10)
	)

	go func() {
		_ = watch.File(ctx, cfgPath, func(err error) {
			if err == nil {
				stateMu.Lock()
				callCount++
				stateMu.Unlock()

				select {
				case changeDone <- struct{}{}:
				default:
				}
			}
		})
	}()

	// Give watcher a moment to attach
	time.Sleep(50 * time.Millisecond)

	err = os.WriteFile(cfgPath, []byte("port = 9090\n"), 0o600)
	assert.NoError(t, err)

	select {
	case <-changeDone:
		stateMu.Lock()
		count := callCount
		stateMu.Unlock()
		assert.True(t, count >= 1)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for file watch change notification")
	}
}

func TestWatchPaths_Create(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	candidatePath := filepath.Join(tmpDir, "local.yaml")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	changeDone := make(chan struct{}, 10)

	go func() {
		_ = watch.Paths(ctx, []string{candidatePath}, func(err error) {
			if err == nil {
				select {
				case changeDone <- struct{}{}:
				default:
				}
			}
		})
	}()

	// Give watcher a moment to attach to parent directory
	time.Sleep(50 * time.Millisecond)

	err := os.WriteFile(candidatePath, []byte("port: 7070\n"), 0o600)
	assert.NoError(t, err)

	select {
	case <-changeDone:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for file creation watch notification")
	}
}

func TestWatchPaths_Delete(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	candidatePath := filepath.Join(tmpDir, "override.yaml")
	err := os.WriteFile(candidatePath, []byte("port: 8888\n"), 0o600)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	changeDone := make(chan struct{}, 10)

	go func() {
		_ = watch.Paths(ctx, []string{candidatePath}, func(err error) {
			if err == nil {
				select {
				case changeDone <- struct{}{}:
				default:
				}
			}
		})
	}()

	// Give watcher a moment to attach
	time.Sleep(50 * time.Millisecond)

	err = os.Remove(candidatePath)
	assert.NoError(t, err)

	select {
	case <-changeDone:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for file deletion watch notification")
	}
}

func TestWatchPaths_DirectoryIsolation(t *testing.T) {
	t.Parallel()

	dirA := filepath.Join(t.TempDir(), "dirA")
	dirB := filepath.Join(t.TempDir(), "dirB")
	assert.NoError(t, os.MkdirAll(dirA, 0o750))
	assert.NoError(t, os.MkdirAll(dirB, 0o750))

	monitoredFileA := filepath.Join(dirA, "config.yaml")
	monitoredFileB := filepath.Join(dirB, "other.json")
	unmonitoredFileB := filepath.Join(dirB, "config.yaml") // Same base name as fileA, but in dirB

	assert.NoError(t, os.WriteFile(monitoredFileA, []byte("port: 1000\n"), 0o600))
	assert.NoError(t, os.WriteFile(monitoredFileB, []byte("port: 2000\n"), 0o600))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	changeDone := make(chan struct{}, 10)

	go func() {
		_ = watch.Paths(ctx, []string{monitoredFileA, monitoredFileB}, func(err error) {
			if err == nil {
				select {
				case changeDone <- struct{}{}:
				default:
				}
			}
		})
	}()

	// Give watcher a moment to attach
	time.Sleep(50 * time.Millisecond)

	err := os.WriteFile(unmonitoredFileB, []byte("port: 9999\n"), 0o600)
	assert.NoError(t, err)

	select {
	case <-changeDone:
		t.Fatal("unmonitored file modification in watched directory triggered unexpected watch callback")
	case <-time.After(150 * time.Millisecond):
		// Expected: no event triggered
	}

	// 4. Act: Modify actual monitored file in dirA
	err = os.WriteFile(monitoredFileA, []byte("port: 1111\n"), 0o600)
	assert.NoError(t, err)

	// 5. Assert: Must trigger callback
	select {
	case <-changeDone:
		// Success: triggered only for monitored file
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for monitored file change notification")
	}
}

type watchTestConfigCLI struct {
	ConfigFile conga.ConfigFile        `short:"c"`
	Port       conga.OptionalFlag[int] `name:"port"`
	reloaded   chan *watchTestConfigCLI
}

func (c *watchTestConfigCLI) Run(ctx context.Context, session *conga.Session) error {
	return watch.Config[watchTestConfigCLI](ctx, session, func(newCfg *watchTestConfigCLI, err error) {
		if err == nil && newCfg != nil && c.reloaded != nil {
			select {
			case c.reloaded <- newCfg:
			default:
			}
		}
	})
}

func TestWatchConfig_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(cfgPath, []byte(`{"port": 8080}`), 0o600)
	assert.NoError(t, err)

	reloadedChan := make(chan *watchTestConfigCLI, 10)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cli := new(watchTestConfigCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.RawDecoder).
		Args([]string{"-c", cfgPath})
	cli.reloaded = reloadedChan

	go func() {
		_ = app.RunContext(ctx)
	}()

	// Give watcher a moment to attach
	time.Sleep(50 * time.Millisecond)

	err = os.WriteFile(cfgPath, []byte(`{"port": 9090}`), 0o600)
	assert.NoError(t, err)

	select {
	case reloaded := <-reloadedChan:
		assert.NotZero(t, reloaded)
		assert.Equal(t, 9090, reloaded.Port.Get())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch.Config reload callback")
	}
}

type watchTestErrorCLI struct {
	ConfigFile conga.ConfigFile        `short:"c"`
	Port       conga.OptionalFlag[int] `name:"port"`
	errorsChan chan error
}

func (c *watchTestErrorCLI) Run(ctx context.Context, session *conga.Session) error {
	return watch.Config[watchTestErrorCLI](ctx, session, func(newCfg *watchTestErrorCLI, err error) {
		if err != nil && c.errorsChan != nil {
			select {
			case c.errorsChan <- err:
			default:
			}
		}
	})
}

func TestWatchConfig_ReloadError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(cfgPath, []byte(`{"port": 8080}`), 0o600)
	assert.NoError(t, err)

	errChan := make(chan error, 10)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cli := new(watchTestErrorCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.RawDecoder).
		Args([]string{"-c", cfgPath})
	cli.errorsChan = errChan

	go func() {
		_ = app.RunContext(ctx)
	}()

	// Give watcher a moment to attach
	time.Sleep(50 * time.Millisecond)

	err = os.WriteFile(cfgPath, []byte(`{"port": "invalid-int"}`), 0o600)
	assert.NoError(t, err)

	select {
	case err := <-errChan:
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reload error callback")
	}
}

func TestWatchConfig_NoCandidates(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	session := &conga.Session{}

	err := watch.Config[watchTestConfigCLI](ctx, session, nil)

	assert.Error(t, err)
	assert.IsError(t, err, watch.ErrNoCandidates)
}

func TestWatchPaths_Empty(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	err := watch.Paths(ctx, nil, func(err error) {})

	assert.NoError(t, err)
}

func TestWatch_DebounceCoalescing(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "debounce.txt")
	err := os.WriteFile(cfgPath, []byte("v0"), 0o600)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var (
		callCount int
		stateMu   sync.Mutex
	)

	go func() {
		_ = watch.File(ctx, cfgPath, func(err error) {
			if err == nil {
				stateMu.Lock()
				callCount++
				stateMu.Unlock()
			}
		})
	}()

	time.Sleep(50 * time.Millisecond)

	for i := 1; i <= 5; i++ {
		err = os.WriteFile(cfgPath, []byte("v"+strconv.Itoa(i)), 0o600)
		assert.NoError(t, err)

		time.Sleep(3 * time.Millisecond)
	}

	time.Sleep(150 * time.Millisecond)

	stateMu.Lock()
	count := callCount
	stateMu.Unlock()

	assert.Equal(t, 1, count)
}

func TestWatchPaths_AtomicReplace(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "atomic.json")
	err := os.WriteFile(cfgPath, []byte("initial"), 0o600)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	changeDone := make(chan struct{}, 10)

	go func() {
		_ = watch.File(ctx, cfgPath, func(err error) {
			if err == nil {
				select {
				case changeDone <- struct{}{}:
				default:
				}
			}
		})
	}()

	time.Sleep(50 * time.Millisecond)

	tempFile := filepath.Join(tmpDir, "atomic.json.tmp")
	err = os.WriteFile(tempFile, []byte("updated"), 0o600)
	assert.NoError(t, err)
	err = os.Rename(tempFile, cfgPath)
	assert.NoError(t, err)

	select {
	case <-changeDone:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for atomic file replacement watch notification")
	}
}

func TestWatch_ContextCancelledDuringDebounce(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cancel.txt")
	err := os.WriteFile(cfgPath, []byte("data"), 0o600)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	var (
		callCount int
		stateMu   sync.Mutex
	)

	watchErrChan := make(chan error, 1)

	go func() {
		watchErrChan <- watch.File(ctx, cfgPath, func(err error) {
			if err == nil {
				stateMu.Lock()
				callCount++
				stateMu.Unlock()
			}
		})
	}()

	time.Sleep(50 * time.Millisecond)

	err = os.WriteFile(cfgPath, []byte("data-updated"), 0o600)
	assert.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case err := <-watchErrChan:
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher to exit on context cancellation")
	}

	time.Sleep(100 * time.Millisecond)

	stateMu.Lock()
	count := callCount
	stateMu.Unlock()

	assert.Equal(t, 0, count)
}

func TestWatchPaths_NonExistentDirectory(t *testing.T) {
	t.Parallel()

	nonExistentPath := filepath.Join(t.TempDir(), "missing_dir", "config.yaml")

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	err := watch.Paths(ctx, []string{nonExistentPath}, nil)

	assert.Error(t, err)
}

func TestWatch_NilOnChange(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "nil_test.txt")
	err := os.WriteFile(cfgPath, []byte("start"), 0o600)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	watchErrChan := make(chan error, 1)

	go func() {
		watchErrChan <- watch.File(ctx, cfgPath, nil)
	}()

	time.Sleep(50 * time.Millisecond)

	err = os.WriteFile(cfgPath, []byte("updated"), 0o600)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-watchErrChan:
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher with nil OnChange to exit on context cancellation")
	}
}

func TestWatchPaths_NonExistentDirectoryCreatedLater(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "missing", "sub")
	cfgPath := filepath.Join(nestedDir, "config.yaml")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var (
		eventMu sync.Mutex
		called  bool
	)

	errChan := make(chan error, 1)
	go func() {
		errChan <- watch.Paths(ctx, []string{cfgPath}, func(err error) {
			if err == nil {
				eventMu.Lock()
				called = true
				eventMu.Unlock()
			}
		})
	}()

	time.Sleep(50 * time.Millisecond)

	assert.NoError(t, os.MkdirAll(nestedDir, 0o755))
	time.Sleep(20 * time.Millisecond)
	assert.NoError(t, os.WriteFile(cfgPath, []byte("key: value"), 0o600))

	var wasCalled bool

	for range 20 {
		time.Sleep(25 * time.Millisecond)
		eventMu.Lock()
		wasCalled = called
		eventMu.Unlock()

		if wasCalled {
			break
		}
	}

	assert.True(t, wasCalled, "expected onChange to be invoked when file in previously non-existent directory is created")

	cancel()

	err := <-errChan
	assert.IsError(t, err, context.Canceled)
}

func TestConfig_NilContext(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context

	assert.Panics(t, func() {
		_ = watch.Config[watchTestConfigCLI](nilCtx, nil, nil)
	})
}

func TestConfig_NilSession(t *testing.T) {
	t.Parallel()

	err := watch.Config[watchTestConfigCLI](t.Context(), nil, nil)

	assert.Error(t, err)
	assert.Equal(t, "watch: session cannot be nil", err.Error())
}

func TestPaths_NilContext(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context

	assert.Panics(t, func() {
		_ = watch.Paths(nilCtx, []string{"dummy.json"}, nil)
	})
}

func TestFile_NilContext(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context

	assert.Panics(t, func() {
		_ = watch.File(nilCtx, "dummy.json", nil)
	})
}
