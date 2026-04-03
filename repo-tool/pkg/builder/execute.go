package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/status"
	"github.com/sirupsen/logrus"
)

const logFilePerms = 0o600

type (
	inspectState struct {
		State struct {
			Status   string
			ExitCode int
		}
	}

	containerRunResult struct {
		Failed  bool
		LogFile string
		Message string
	}
)

var (
	dockerOutputFn = dockerOutput
	sleepFn        = time.Sleep
	writeFileFn    = os.WriteFile
)

func dockerOutput(ctx context.Context, args ...string) (string, error) {
	//#nosec:G204 // Running internally built commands
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}

	return string(out), nil
}

func executeAndWatchContainer(ctx context.Context, opts BuildOpts, identifier, repo string) (result containerRunResult, err error) {
	repoBase := filepath.Base(repo)
	startedAt := time.Now()

	signingKeyPath, err := filepath.Abs(opts.SigningKeyPath)
	if err != nil {
		return containerRunResult{}, fmt.Errorf("resolving signing key path: %w", err)
	}

	pacmanConfPath, err := filepath.Abs(opts.PacmanConf)
	if err != nil {
		return containerRunResult{}, fmt.Errorf("resolving pacman.conf path: %w", err)
	}

	repoDirPath, err := filepath.Abs(opts.RepoDir)
	if err != nil {
		return containerRunResult{}, fmt.Errorf("resolving repo dir path: %w", err)
	}

	containerID, err := dockerOutputFn(
		ctx,
		"run", "-d",
		"--tmpfs", "/src:rw,exec",
		"--volume", signingKeyPath+":/config/signing.asc:ro",
		"--volume", pacmanConfPath+":/config/pacman.conf:ro",
		"--volume", repoDirPath+":/repo",
		"--ulimit", "nofile=262144:262144",
		opts.BuildImage,
		repo,
	)
	if err != nil {
		return containerRunResult{}, fmt.Errorf("starting build container: %w", err)
	}

	containerID = strings.TrimSpace(containerID)
	defer func() {
		if containerID == "" {
			return
		}

		if _, err = dockerOutputFn(ctx, "rm", "-f", containerID); err != nil {
			logrus.WithError(err).WithField("container", containerID).Error("removing container")
		}
	}()

	for {
		rawState, err := dockerOutputFn(ctx, "inspect", containerID)
		if err != nil {
			return containerRunResult{}, fmt.Errorf("inspecting container %s: %w", containerID, err)
		}

		var state []inspectState
		if err = json.Unmarshal([]byte(rawState), &state); err != nil {
			return containerRunResult{}, fmt.Errorf("decoding container state: %w", err)
		}

		if len(state) != 1 {
			return containerRunResult{}, fmt.Errorf("unexpected inspect response for container %s", containerID)
		}

		if state[0].State.Status == "running" {
			opts.Display.UpdateJob(
				identifier,
				status.JobStatusRunning,
				fmt.Sprintf("Running for %s in container %s", time.Since(startedAt).Round(time.Second), containerID[:8]),
			)
			sleepFn(time.Second)
			continue
		}

		if state[0].State.ExitCode == 0 {
			return containerRunResult{}, nil
		}

		logFile := filepath.Join(os.TempDir(), "arch-package-build_"+repoBase+".log")
		logs, err := dockerOutputFn(ctx, "logs", containerID)
		if err != nil {
			return containerRunResult{}, fmt.Errorf("build failed with exit code %d and logs could not be read: %w", state[0].State.ExitCode, err)
		}

		if err = writeFileFn(logFile, []byte(logs), logFilePerms); err != nil {
			return containerRunResult{}, fmt.Errorf("build failed with exit code %d and logs could not be written: %w", state[0].State.ExitCode, err)
		}

		return containerRunResult{
			Failed:  true,
			LogFile: logFile,
			Message: fmt.Sprintf("build failed with exit code %d, see logs at %s", state[0].State.ExitCode, logFile),
		}, nil
	}
}
