package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/config"
)

var gitOutputFn = gitOutput

func gitOutput(ctx context.Context, args ...string) (output []byte, err error) {
	//#nosec:G204 // Arguments come from trusted config and internal constants
	output, err = exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return output, fmt.Errorf("running command: %w", err)
	}
	return output, nil
}

// GetHeadCommit fetches the latest commit of the default branch in
// the repository given inside the Package configuration
func GetHeadCommit(ctx context.Context, pkg config.Package) (commitHash string, err error) {
	symrefOut, err := gitOutputFn(ctx, "ls-remote", "--symref", pkg.Repo, "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolving default branch for %q: %w", pkg.Repo, err)
	}

	var defaultBranchRef string
	for line := range strings.SplitSeq(strings.TrimSpace(string(symrefOut)), "\n") {
		if !strings.HasPrefix(line, "ref: ") || !strings.HasSuffix(line, "\tHEAD") {
			continue
		}

		defaultBranchRef = strings.TrimSuffix(strings.TrimPrefix(line, "ref: "), "\tHEAD")
		break
	}

	if defaultBranchRef == "" {
		for _, fallbackRef := range []string{"refs/heads/master", "refs/heads/main"} {
			commitHash, err = resolveCommitHash(ctx, pkg.Repo, fallbackRef)
			if err != nil {
				return "", err
			}
			if commitHash != "" {
				return commitHash, nil
			}
		}

		return "", fmt.Errorf(
			"resolving default branch for %q: no HEAD symref returned and fallback refs %q did not resolve to a commit",
			pkg.Repo,
			[]string{"refs/heads/master", "refs/heads/main"},
		)
	}

	commitHash, err = resolveCommitHash(ctx, pkg.Repo, defaultBranchRef)
	if err != nil {
		return "", err
	}
	if commitHash == "" {
		return "", fmt.Errorf("resolving head commit for %q on %q: empty response", pkg.Repo, defaultBranchRef)
	}

	return commitHash, nil
}

func resolveCommitHash(ctx context.Context, repo, ref string) (string, error) {
	commitOut, err := gitOutputFn(ctx, "ls-remote", repo, ref)
	if err != nil {
		return "", fmt.Errorf("resolving head commit for %q on %q: %w", repo, ref, err)
	}

	commitHash, _, _ := strings.Cut(string(commitOut), "\t")
	return strings.TrimSpace(commitHash), nil
}
