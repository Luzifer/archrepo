package builder

import (
	"context"
	"fmt"
	"strings"
	"time"

	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/cache"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/config"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/status"
)

type (
	// BuildOpts holds the information required for running the build
	BuildOpts struct {
		Config config.File

		BuildImage         string      // Docker Image to run for building
		Cache              *cache.File // Cache storage
		Display            *status.Display
		DiplayAlreadyBuilt bool
		PacmanConf         string // Path to pacman.conf to use in the container
		RepoDir            string // Path to mount as repo-dir into the container
		SigningKeyPath     string // Path to the secret for signing packages
	}

	// BuildFailure stores a package build failure that happened after the
	// container started successfully.
	BuildFailure struct {
		Name    string
		Message string
	}

	// BuildResult summarizes the outcome of a build run.
	BuildResult struct {
		SuccessCount int
		BlockedCount int
		Failures     []BuildFailure
	}
)

var (
	createExecutionPlanFn      = createExecutionPlan
	executeAndWatchContainerFn = executeAndWatchContainer
)

// RunBuild creates the build plan, checks the repos whether builds
// are required and executes the builds if so
func RunBuild(ctx context.Context, opts BuildOpts) (BuildResult, error) {
	if err := prepareBuild(&opts); err != nil {
		return BuildResult{}, err
	}

	plan, commits, err := createExecutionPlanFn(ctx, opts)
	if err != nil {
		return BuildResult{}, fmt.Errorf("creating execution plan: %w", err)
	}

	if len(plan) == 0 {
		return BuildResult{}, nil
	}

	if _, err = dockerOutputFn(ctx, "pull", opts.BuildImage); err != nil {
		return BuildResult{}, fmt.Errorf("pulling builder image %q: %w", opts.BuildImage, err)
	}

	var (
		blocked = make(map[string]struct{})
		result  = BuildResult{}
	)

	for _, name := range plan {
		if err = runPlannedBuild(ctx, opts, name, commits[name], blocked, &result); err != nil {
			return result, err
		}
	}

	return result, nil
}

func blockedDependencies(pkg config.Package, blocked map[string]struct{}) []string {
	var deps []string

	for _, dep := range pkg.DependsOn {
		if _, ok := blocked[dep]; ok {
			deps = append(deps, dep)
		}
	}

	return deps
}

func prepareBuild(opts *BuildOpts) error {
	if opts.Display == nil {
		return fmt.Errorf("display is required")
	}

	if opts.Cache == nil {
		return fmt.Errorf("cache is required")
	}

	if opts.Cache.Packages == nil {
		opts.Cache.Packages = make(map[string]cache.Package)
	}

	return nil
}

func runPlannedBuild(ctx context.Context, opts BuildOpts, name, commit string, blocked map[string]struct{}, result *BuildResult) error {
	pkg := opts.Config.Packages[name]
	entry := opts.Cache.Packages[name]
	entry.LastBuiltAt = time.Now()
	entry.LastSeenCommit = commit
	entry.Repo = pkg.Repo

	if isBlockedBy := blockedDependencies(pkg, blocked); len(isBlockedBy) > 0 {
		return recordBlockedBuild(opts, name, entry, isBlockedBy, blocked, result)
	}

	opts.Display.AddJob(name, status.JobStatusRunning, pkg.Repo)

	buildState, err := executeAndWatchContainerFn(ctx, opts, name, pkg.Repo)
	if err != nil {
		return fmt.Errorf("building %q: %w", name, err)
	}
	if buildState.Failed {
		return recordFailedBuild(opts, name, entry, buildState, blocked, result)
	}

	return recordSuccessfulBuild(opts, name, commit, entry, result)
}

func recordBlockedBuild(opts BuildOpts, name string, entry cache.Package, isBlockedBy []string, blocked map[string]struct{}, result *BuildResult) error {
	entry.LastBuiltStatus = cache.BuildStatusBlocked
	blocked[name] = struct{}{}
	result.BlockedCount++
	opts.Display.AddJob(name, status.JobStatusSkipped, fmt.Sprintf("blocked by failed dependencies: %s", strings.Join(isBlockedBy, ", ")))

	if err := updateCache(opts.Cache, name, entry); err != nil {
		return fmt.Errorf("updating build-cache: %w", err)
	}

	return nil
}

func recordFailedBuild(opts BuildOpts, name string, entry cache.Package, buildState containerRunResult, blocked map[string]struct{}, result *BuildResult) error {
	entry.LastBuiltStatus = cache.BuildStatusFailed
	blocked[name] = struct{}{}
	opts.Display.UpdateJob(name, status.JobStatusFailure, buildState.Message)
	result.Failures = append(result.Failures, BuildFailure{
		Name:    name,
		Message: buildState.Message,
	})

	if err := updateCache(opts.Cache, name, entry); err != nil {
		return fmt.Errorf("updating build-cache: %w", err)
	}

	return nil
}

func recordSuccessfulBuild(opts BuildOpts, name, commit string, entry cache.Package, result *BuildResult) error {
	entry.LastBuiltCommit = commit
	entry.LastBuiltStatus = cache.BuildStatusSuccess
	result.SuccessCount++
	opts.Display.UpdateJob(name, status.JobStatusSuccess, "")

	if err := updateCache(opts.Cache, name, entry); err != nil {
		return fmt.Errorf("updating build-cache: %w", err)
	}

	return nil
}

func updateCache(buildCache *cache.File, name string, entry cache.Package) (err error) {
	buildCache.Packages[name] = entry

	if err = buildCache.Save(); err != nil {
		return fmt.Errorf("saving cache: %w", err)
	}

	return nil
}

// HasFailures reports whether any package build failed after starting.
func (r BuildResult) HasFailures() bool {
	return len(r.Failures) > 0
}
