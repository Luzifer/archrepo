package git

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchHeadCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	commit, err := GetHeadCommit(ctx, config.Package{
		Repo: "https://github.com/Luzifer/archrepo.git",
	})

	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{40}$`), commit)
}

//nolint:funlen // Just a long list of testcases
func TestGetHeadCommit(t *testing.T) {
	const (
		repo             = "https://example.com/repo.git"
		headCommit       = "1111111111111111111111111111111111111111"
		masterCommit     = "2222222222222222222222222222222222222222"
		mainCommit       = "3333333333333333333333333333333333333333"
		defaultBranchRef = "refs/heads/trunk"
	)

	type commandResponse struct {
		out []byte
		err error
	}

	testCases := []struct {
		name         string
		responses    map[string]commandResponse
		wantCommit   string
		wantErr      string
		wantCommands []string
	}{
		{
			name: "resolves through HEAD symref",
			responses: map[string]commandResponse{
				"ls-remote --symref " + repo + " HEAD": {
					out: []byte("ref: " + defaultBranchRef + "\tHEAD\n" + headCommit + "\tHEAD\n"),
				},
				"ls-remote " + repo + " " + defaultBranchRef: {
					out: []byte(headCommit + "\t" + defaultBranchRef + "\n"),
				},
			},
			wantCommit: headCommit,
			wantCommands: []string{
				"ls-remote --symref " + repo + " HEAD",
				"ls-remote " + repo + " " + defaultBranchRef,
			},
		},
		{
			name: "falls back to master when HEAD has no symref",
			responses: map[string]commandResponse{
				"ls-remote --symref " + repo + " HEAD": {
					out: []byte(masterCommit + "\tHEAD\n"),
				},
				"ls-remote " + repo + " refs/heads/master": {
					out: []byte(masterCommit + "\trefs/heads/master\n"),
				},
			},
			wantCommit: masterCommit,
			wantCommands: []string{
				"ls-remote --symref " + repo + " HEAD",
				"ls-remote " + repo + " refs/heads/master",
			},
		},
		{
			name: "falls back to main after empty master",
			responses: map[string]commandResponse{
				"ls-remote --symref " + repo + " HEAD": {
					out: []byte(""),
				},
				"ls-remote " + repo + " refs/heads/master": {
					out: []byte(""),
				},
				"ls-remote " + repo + " refs/heads/main": {
					out: []byte(mainCommit + "\trefs/heads/main\n"),
				},
			},
			wantCommit: mainCommit,
			wantCommands: []string{
				"ls-remote --symref " + repo + " HEAD",
				"ls-remote " + repo + " refs/heads/master",
				"ls-remote " + repo + " refs/heads/main",
			},
		},
		{
			name: "returns explicit error when no fallback ref resolves",
			responses: map[string]commandResponse{
				"ls-remote --symref " + repo + " HEAD": {
					out: []byte(""),
				},
				"ls-remote " + repo + " refs/heads/master": {
					out: []byte(""),
				},
				"ls-remote " + repo + " refs/heads/main": {
					out: []byte(""),
				},
			},
			wantErr: `resolving default branch for "https://example.com/repo.git": no HEAD symref returned and fallback refs ["refs/heads/master" "refs/heads/main"] did not resolve to a commit`,
			wantCommands: []string{
				"ls-remote --symref " + repo + " HEAD",
				"ls-remote " + repo + " refs/heads/master",
				"ls-remote " + repo + " refs/heads/main",
			},
		},
		{
			name: "propagates HEAD lookup failure",
			responses: map[string]commandResponse{
				"ls-remote --symref " + repo + " HEAD": {
					err: errors.New("fatal: repository not found"),
				},
			},
			wantErr: `resolving default branch for "https://example.com/repo.git": fatal: repository not found`,
			wantCommands: []string{
				"ls-remote --symref " + repo + " HEAD",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			gitOutputFn = func(_ context.Context, args ...string) ([]byte, error) {
				call := strings.Join(args, " ")
				calls = append(calls, call)

				resp, ok := tc.responses[call]
				if !ok {
					t.Fatalf("unexpected git invocation: %q", call)
				}

				return resp.out, resp.err
			}
			t.Cleanup(func() { gitOutputFn = gitOutput })

			commit, err := GetHeadCommit(context.Background(), config.Package{Repo: repo})

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tc.wantErr)
				assert.Empty(t, commit)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantCommit, commit)
			}

			assert.Equal(t, tc.wantCommands, calls)
		})
	}
}

func TestNonExistingRepo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	commit, err := GetHeadCommit(ctx, config.Package{
		Repo: "https://github.com/Luzifer/I-will-never-exist.git",
	})

	require.Error(t, err)
	assert.Empty(t, commit)
	assert.ErrorContains(t, err, "resolving default branch")
}
