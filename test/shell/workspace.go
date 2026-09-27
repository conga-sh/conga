//go:build shelltest

package shell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	binaryCacheMu sync.Mutex                //nolint:gochecknoglobals // Process-wide cache for compiled test binaries.
	binaryCache   = make(map[string]string) //nolint:gochecknoglobals // Process-wide cache for compiled test binaries.
)

// Workspace represents an isolated, CI-safe test environment for shell completion testing.
type Workspace struct {
	Root    string
	BinDir  string
	HomeDir string
	WorkDir string
}

// NewWorkspace initializes an isolated filesystem workspace for shell completion testing.
// It checks CONGA_TEST_ROOT, then TEST_TMPDIR, and falls back to t.TempDir().
func NewWorkspace(t *testing.T) *Workspace {
	t.Helper()

	var baseDir string
	if envRoot := os.Getenv("CONGA_TEST_ROOT"); envRoot != "" {
		baseDir = filepath.Join(envRoot, sanitizeName(t.Name()))
		if err := os.MkdirAll(baseDir, 0o700); err != nil { //nolint:gosec // Isolated test workspace.
			t.Fatalf("failed to create CONGA_TEST_ROOT directory: %v", err)
		}
	} else if bazelTmp := os.Getenv("TEST_TMPDIR"); bazelTmp != "" {
		baseDir = filepath.Join(bazelTmp, sanitizeName(t.Name()))
		if err := os.MkdirAll(baseDir, 0o700); err != nil { //nolint:gosec // Isolated test workspace.
			t.Fatalf("failed to create TEST_TMPDIR directory: %v", err)
		}
	} else {
		baseDir = t.TempDir()
	}

	workspace := &Workspace{
		Root:    baseDir,
		BinDir:  filepath.Join(baseDir, "bin"),
		HomeDir: filepath.Join(baseDir, "home"),
		WorkDir: filepath.Join(baseDir, "work"),
	}

	if err := os.MkdirAll(workspace.BinDir, 0o700); err != nil { //nolint:gosec // Isolated test workspace.
		t.Fatalf("failed to create bin dir: %v", err)
	}

	if err := os.MkdirAll(workspace.HomeDir, 0o700); err != nil { //nolint:gosec // Isolated test workspace.
		t.Fatalf("failed to create home dir: %v", err)
	}

	if err := os.MkdirAll(workspace.WorkDir, 0o700); err != nil { //nolint:gosec // Isolated test workspace.
		t.Fatalf("failed to create work dir: %v", err)
	}

	return workspace
}

// BuildBinary compiles a Go CLI package into the workspace bin directory.
func (ws *Workspace) BuildBinary(t *testing.T, pkgPath, binaryName string) string {
	t.Helper()

	targetPath := filepath.Join(ws.BinDir, binaryName)

	binaryCacheMu.Lock()

	cachedPath, exists := binaryCache[pkgPath]
	if !exists {
		cacheDir, err := os.MkdirTemp("", "conga_shelltest_bin_*") //nolint:usetesting // Cache must persist beyond a single test.
		if err != nil {
			binaryCacheMu.Unlock()
			t.Fatalf("failed to create bin cache dir: %v", err)
		}

		cachedPath = filepath.Join(cacheDir, binaryName)
		cmd := exec.CommandContext(t.Context(), "go", "build", "-o", cachedPath, pkgPath) //nolint:gosec // Test harness building test binary.

		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

		if out, err := cmd.CombinedOutput(); err != nil {
			binaryCacheMu.Unlock()
			t.Fatalf("failed to build binary %q (%s): %v\nOutput: %s", binaryName, pkgPath, err, string(out))
		}

		binaryCache[pkgPath] = cachedPath
	}
	binaryCacheMu.Unlock()

	if err := os.Symlink(cachedPath, targetPath); err != nil {
		if err := copyFile(cachedPath, targetPath); err != nil {
			t.Fatalf("failed to link or copy cached binary: %v", err)
		}
	}

	return targetPath
}

// WriteFile creates a file relative to the workspace work directory.
func (ws *Workspace) WriteFile(t *testing.T, relPath string, content []byte) string {
	t.Helper()

	fullPath := filepath.Join(ws.WorkDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		t.Fatalf("failed to create dir for %s: %v", relPath, err)
	}

	if err := os.WriteFile(fullPath, content, 0o600); err != nil {
		t.Fatalf("failed to write file %s: %v", relPath, err)
	}

	return fullPath
}

// Mkdir creates a directory relative to the workspace work directory.
func (ws *Workspace) Mkdir(t *testing.T, relPath string) string {
	t.Helper()

	fullPath := filepath.Join(ws.WorkDir, relPath)
	if err := os.MkdirAll(fullPath, 0o700); err != nil {
		t.Fatalf("failed to create dir %s: %v", relPath, err)
	}

	return fullPath
}

func sanitizeName(name string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")

	return r.Replace(name)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(filepath.Clean(src))
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer func() {
		_ = srcFile.Close() //nolint:errcheck // Best-effort cleanup.
	}()

	dstFile, err := os.OpenFile(filepath.Clean(dst), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // G304: path is controlled by the test harness.
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}
	defer func() {
		_ = dstFile.Close() //nolint:errcheck // Best-effort cleanup.
	}()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy file contents: %w", err)
	}

	return nil
}
