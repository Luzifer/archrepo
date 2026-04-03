package builder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/cache"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/config"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/status"
	"github.com/stretchr/testify/require"
)

func TestRunBuildRecordsStartedBuildFailuresWithoutFatalError(t *testing.T) {
	t.Cleanup(resetBuilderTestHooks)

	createExecutionPlanFn = func(context.Context, BuildOpts) ([]string, map[string]string, error) {
		return []string{"pkg-a", "pkg-b"}, map[string]string{
			"pkg-a": "commit-a",
			"pkg-b": "commit-b",
		}, nil
	}

	dockerOutputFn = func(context.Context, ...string) (string, error) {
		return "", nil
	}

	executeAndWatchContainerFn = func(_ context.Context, _ BuildOpts, _ string, repo string) (containerRunResult, error) {
		if repo == "https://example.invalid/pkg-a" {
			return containerRunResult{
				Failed:  true,
				LogFile: "/tmp/pkg-a.log",
				Message: "build failed with exit code 1, see logs at /tmp/pkg-a.log",
			}, nil
		}

		return containerRunResult{}, nil
	}

	result, err := RunBuild(context.Background(), BuildOpts{
		Config: config.File{
			Packages: map[string]config.Package{
				"pkg-a": {Repo: "https://example.invalid/pkg-a"},
				"pkg-b": {
					Repo:      "https://example.invalid/pkg-b",
					DependsOn: []string{"pkg-a"},
				},
			},
		},
		BuildImage: "buildimg",
		Cache:      testCacheFile(t),
		Display:    status.NewDisplay(),
	})
	require.NoError(t, err)

	require.Len(t, result.Failures, 1)
	require.Equal(t, "pkg-a", result.Failures[0].Name)
	require.Contains(t, result.Failures[0].Message, "/tmp/pkg-a.log")
	require.Equal(t, 0, result.SuccessCount)
	require.Equal(t, 1, result.BlockedCount)
}

func TestRunBuildReturnsFatalErrorWhenContainerStartFails(t *testing.T) {
	t.Cleanup(resetBuilderTestHooks)

	createExecutionPlanFn = func(context.Context, BuildOpts) ([]string, map[string]string, error) {
		return []string{"pkg-a"}, map[string]string{"pkg-a": "commit-a"}, nil
	}

	dockerOutputFn = func(context.Context, ...string) (string, error) {
		return "", nil
	}

	executeAndWatchContainerFn = func(context.Context, BuildOpts, string, string) (containerRunResult, error) {
		return containerRunResult{}, errors.New("starting build container: docker failed")
	}

	result, err := RunBuild(context.Background(), BuildOpts{
		Config: config.File{
			Packages: map[string]config.Package{
				"pkg-a": {Repo: "https://example.invalid/pkg-a"},
			},
		},
		BuildImage: "buildimg",
		Cache:      testCacheFile(t),
		Display:    status.NewDisplay(),
	})
	require.Error(t, err)
	require.False(t, result.HasFailures())
	require.Contains(t, err.Error(), `building "pkg-a"`)
}

func TestRunBuildRecordsBlockedDependenciesInCache(t *testing.T) {
	t.Cleanup(resetBuilderTestHooks)

	createExecutionPlanFn = func(context.Context, BuildOpts) ([]string, map[string]string, error) {
		return []string{"pkg-a", "pkg-b"}, map[string]string{
			"pkg-a": "commit-a",
			"pkg-b": "commit-b",
		}, nil
	}

	dockerOutputFn = func(context.Context, ...string) (string, error) {
		return "", nil
	}

	executeAndWatchContainerFn = func(_ context.Context, _ BuildOpts, _ string, repo string) (containerRunResult, error) {
		if repo == "https://example.invalid/pkg-a" {
			return containerRunResult{
				Failed:  true,
				Message: "pkg-a failed",
			}, nil
		}

		t.Fatalf("unexpected build for repo %s", repo)
		return containerRunResult{}, nil
	}

	buildCache := testCacheFile(t)
	result, err := RunBuild(context.Background(), BuildOpts{
		Config: config.File{
			Packages: map[string]config.Package{
				"pkg-a": {Repo: "https://example.invalid/pkg-a"},
				"pkg-b": {
					Repo:      "https://example.invalid/pkg-b",
					DependsOn: []string{"pkg-a"},
				},
			},
		},
		BuildImage: "buildimg",
		Cache:      buildCache,
		Display:    status.NewDisplay(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.BlockedCount)
	require.Equal(t, cache.BuildStatusBlocked, buildCache.Packages["pkg-b"].LastBuiltStatus)
	require.Equal(t, "commit-b", buildCache.Packages["pkg-b"].LastSeenCommit)
}

func TestRunBuildRecordsSuccessfulBuildInCache(t *testing.T) {
	t.Cleanup(resetBuilderTestHooks)

	createExecutionPlanFn = func(context.Context, BuildOpts) ([]string, map[string]string, error) {
		return []string{"pkg-a"}, map[string]string{"pkg-a": "commit-a"}, nil
	}

	dockerOutputFn = func(context.Context, ...string) (string, error) {
		return "", nil
	}

	executeAndWatchContainerFn = func(context.Context, BuildOpts, string, string) (containerRunResult, error) {
		return containerRunResult{}, nil
	}

	buildCache := testCacheFile(t)
	result, err := RunBuild(context.Background(), BuildOpts{
		Config: config.File{
			Packages: map[string]config.Package{
				"pkg-a": {Repo: "https://example.invalid/pkg-a"},
			},
		},
		BuildImage: "buildimg",
		Cache:      buildCache,
		Display:    status.NewDisplay(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, cache.BuildStatusSuccess, buildCache.Packages["pkg-a"].LastBuiltStatus)
	require.Equal(t, "commit-a", buildCache.Packages["pkg-a"].LastBuiltCommit)
	require.Equal(t, "commit-a", buildCache.Packages["pkg-a"].LastSeenCommit)
}

func TestExecuteAndWatchContainerReturnsRecordedFailure(t *testing.T) {
	t.Cleanup(resetBuilderTestHooks)

	sleepFn = func(time.Duration) {}

	var writes []string
	writeFileFn = func(name string, data []byte, perm os.FileMode) error {
		writes = append(writes, name)
		require.EqualValues(t, logFilePerms, perm)
		require.Equal(t, "container logs", string(data))
		return nil
	}

	dockerOutputFn = func(_ context.Context, args ...string) (string, error) {
		switch args[0] {
		case "run":
			return "container-123\n", nil
		case "inspect":
			return `[{"State":{"Status":"exited","ExitCode":23}}]`, nil
		case "logs":
			return "container logs", nil
		case "rm":
			return "", nil
		default:
			return "", errors.New("unexpected docker command")
		}
	}

	result, err := executeAndWatchContainer(context.Background(), BuildOpts{}, "pkg-a", "https://example.invalid/pkg-a")
	require.NoError(t, err)
	require.True(t, result.Failed)
	require.Contains(t, result.Message, "see logs at")
	require.Contains(t, result.Message, result.LogFile)
	require.Len(t, writes, 1)
	require.Equal(t, result.LogFile, writes[0])
}

func resetBuilderTestHooks() {
	createExecutionPlanFn = createExecutionPlan
	executeAndWatchContainerFn = executeAndWatchContainer
	dockerOutputFn = dockerOutput
	sleepFn = time.Sleep
	writeFileFn = os.WriteFile
}

func testCacheFile(t *testing.T) *cache.File {
	t.Helper()

	filename := filepath.Join(t.TempDir(), "build-cache.yaml")
	f, err := cache.Load(filename)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(filename))
	})

	return f
}
