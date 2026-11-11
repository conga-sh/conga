package why

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestEnrichJSONReport_IgnoresInvalidPWD(t *testing.T) {
	t.Setenv("PWD", "/definitely/not/the/cwd")

	report := Report{}

	enrichJSONReport(&report)

	workingDirectory, err := os.Getwd()
	assert.NoError(t, err)
	assert.Equal(t, workingDirectory, report.WorkingDirectory)
}

func TestBuildConfigPaths_FirstHitSkipsLaterTiers(t *testing.T) {
	t.Parallel()

	localDir := t.TempDir()
	userDir := t.TempDir()
	systemDir := t.TempDir()
	loadedFile := filepath.Join(localDir, "config.json")
	configProbed := map[string][]string{
		localDir:  nil,
		userDir:   {filepath.Join(userDir, "config.json")},
		systemDir: {filepath.Join(systemDir, "config.json")},
	}

	paths := BuildConfigPaths([]string{localDir, userDir, systemDir}, nil, []string{loadedFile}, configProbed, true)

	assert.Equal(t, []ConfigPath{
		{Path: localDir, Status: statusLoaded, Loaded: loadedFile},
		{Path: userDir, Status: statusSkipped},
		{Path: systemDir, Status: statusSkipped},
	}, paths)
}

func TestBuildConfigPaths_CascadingReportsAbsentTiers(t *testing.T) {
	t.Parallel()

	localDir := t.TempDir()
	userDir := t.TempDir()
	loadedFile := filepath.Join(localDir, "config.json")
	userProbes := []string{filepath.Join(userDir, "config.json")}
	configProbed := map[string][]string{localDir: nil, userDir: userProbes}

	paths := BuildConfigPaths([]string{localDir, userDir}, nil, []string{loadedFile}, configProbed, false)

	assert.Equal(t, []ConfigPath{
		{Path: localDir, Status: statusLoaded, Loaded: loadedFile},
		{Path: userDir, Status: statusNoConfigFile, ProbedFiles: userProbes},
	}, paths)
}

func TestBuildConfigPaths_BootstrapOriginDoesNotSkipTiers(t *testing.T) {
	t.Parallel()

	localDir := t.TempDir()
	localProbes := []string{filepath.Join(localDir, "config.json")}
	configProbed := map[string][]string{localDir: localProbes}

	paths := BuildConfigPaths([]string{"bootstrap document", localDir}, nil, []string{"bootstrap document"}, configProbed, true)

	assert.Equal(t, []ConfigPath{
		{Path: "bootstrap document", Status: statusLoaded},
		{Path: localDir, Status: statusNoConfigFile, ProbedFiles: localProbes},
	}, paths)
}

func TestNormalizeStatusEnum_Skipped(t *testing.T) {
	t.Parallel()

	status := normalizeStatusEnum(statusSkipped)

	assert.Equal(t, "skipped", status)
}

func TestBuildConfigPaths_SkippedNamedTargetsFollowCandidates(t *testing.T) {
	t.Parallel()

	paths := BuildConfigPaths([]string{"/work/ci.json"}, []string{"/work/team.json", "/etc/app.json"}, []string{"/work/ci.json"}, nil, true)

	assert.Equal(t, []ConfigPath{
		{Path: "/work/ci.json", Status: statusLoaded},
		{Path: "/work/team.json", Status: statusSkippedNamed},
		{Path: "/etc/app.json", Status: statusSkippedNamed},
	}, paths)
}

func TestBuildConfigPaths_SkippedTargetAlsoLoadedListedOnce(t *testing.T) {
	t.Parallel()

	paths := BuildConfigPaths([]string{"/etc/app.json"}, []string{"/etc/app.json"}, []string{"/etc/app.json"}, nil, true)

	assert.Equal(t, []ConfigPath{
		{Path: "/etc/app.json", Status: statusLoaded},
	}, paths)
}

func TestNormalizeStatusEnum_SkippedNamedTargetIsSkipped(t *testing.T) {
	t.Parallel()

	status := normalizeStatusEnum(statusSkippedNamed)

	assert.Equal(t, "skipped", status)
}
