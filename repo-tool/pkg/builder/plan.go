package builder

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/cache"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/config"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/git"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/status"
)

const planJobKey = "creating plan"

func createExecutionPlan(
	ctx context.Context,
	opts BuildOpts,
) (
	plan []string,
	commits map[string]string,
	err error,
) {
	opts.Display.AddJob(planJobKey, status.JobStatusRunning, "")

	names := orderedPackageNames(opts.Config)

	commits, err = packageHeadCommits(ctx, opts, names)
	if err != nil {
		opts.Display.UpdateJob(planJobKey, status.JobStatusFailure, "")
		return nil, nil, err
	}

	queued := make(map[string]struct{}, len(names))
	plan = make([]string, 0, len(names))

	for _, name := range names {
		pkg := opts.Config.Packages[name]

		if packageAlreadyBuilt(opts.Cache, name, commits[name]) && !hasQueuedDependency(pkg, queued) {
			if opts.DiplayAlreadyBuilt {
				opts.Display.AddJob(name, status.JobStatusSkipped, "already built")
			}
			continue
		}

		queued[name] = struct{}{}
		plan = append(plan, name)
	}

	opts.Display.UpdateJob(planJobKey, status.JobStatusSuccess, "")
	return plan, commits, nil
}

func hasQueuedDependency(pkg config.Package, queued map[string]struct{}) bool {
	for _, dep := range pkg.DependsOn {
		if _, ok := queued[dep]; ok {
			return true
		}
	}

	return false
}

func orderedPackageNames(conf config.File) []string {
	names := make([]string, 0, len(conf.Packages))
	for name := range conf.Packages {
		names = append(names, name)
	}

	depths := make(map[string]int, len(names))
	var packageDepth func(string) int
	packageDepth = func(name string) int {
		if depth, ok := depths[name]; ok {
			return depth
		}

		depth := 0
		for _, dep := range conf.Packages[name].DependsOn {
			depth = max(depth, packageDepth(dep)+1)
		}

		depths[name] = depth
		return depth
	}

	for _, name := range names {
		packageDepth(name)
	}

	slices.SortStableFunc(names, func(a, b string) int {
		return cmp.Or(
			cmp.Compare(depths[a], depths[b]),
			cmp.Compare(a, b),
		)
	})

	return names
}

func packageAlreadyBuilt(buildCache *cache.File, name, commit string) bool {
	if buildCache == nil {
		return false
	}

	cacheEntry, ok := buildCache.Packages[name]
	if !ok {
		return false
	}

	return cacheEntry.LastBuiltStatus == cache.BuildStatusSuccess && cacheEntry.LastBuiltCommit == commit
}

func packageHeadCommits(ctx context.Context, opts BuildOpts, names []string) (commits map[string]string, err error) {
	commits = make(map[string]string, len(names))

	for _, name := range names {
		opts.Display.UpdateJob(planJobKey, status.JobStatusRunning, fmt.Sprintf("scanning head-commit of %q", name))
		commit, err := git.GetHeadCommit(ctx, opts.Config.Packages[name])
		if err != nil {
			return nil, fmt.Errorf("resolving head commit for %q: %w", name, err)
		}

		commits[name] = commit
	}

	return commits, nil
}
