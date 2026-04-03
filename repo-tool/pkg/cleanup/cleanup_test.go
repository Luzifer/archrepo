package cleanup

import (
	"os"
	"path/filepath"
	"testing"

	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestRemoveUnmanaged(t *testing.T) {
	for _, tc := range []struct {
		name       string
		conf       config.File
		files      []string
		remaining  []string
	}{
		{
			name: "managed package is retained",
			conf: config.File{Packages: map[string]config.Package{
				"foo": {},
			}},
			files: []string{
				"foo-1.2.3-any.pkg.tar.zstd",
				"foo-1.2.3-any.pkg.tar.zstd.sig",
			},
			remaining: []string{
				"foo-1.2.3-any.pkg.tar.zstd",
				"foo-1.2.3-any.pkg.tar.zstd.sig",
			},
		},
		{
			name: "unmanaged package and signature are removed",
			conf: config.File{Packages: map[string]config.Package{
				"foo": {},
			}},
			files: []string{
				"bar-1.2.3-any.pkg.tar.zstd",
				"bar-1.2.3-any.pkg.tar.zstd.sig",
			},
			remaining: nil,
		},
		{
			name: "missing signature does not fail",
			conf: config.File{Packages: map[string]config.Package{
				"foo": {},
			}},
			files: []string{
				"bar-1.2.3-any.pkg.tar.zstd",
			},
			remaining: nil,
		},
		{
			name: "shared prefix is not treated as managed",
			conf: config.File{Packages: map[string]config.Package{
				"foo": {},
			}},
			files: []string{
				"foobar-1.2.3-any.pkg.tar.zstd",
				"foobar-1.2.3-any.pkg.tar.zstd.sig",
			},
			remaining: nil,
		},
		{
			name: "non package files and standalone signatures are ignored",
			conf: config.File{Packages: map[string]config.Package{
				"foo": {},
			}},
			files: []string{
				"notes.txt",
				"orphan.pkg.tar.zstd.sig",
			},
			remaining: []string{
				"notes.txt",
				"orphan.pkg.tar.zstd.sig",
			},
		},
		{
			name: "empty repo dir succeeds",
			conf: config.File{Packages: map[string]config.Package{
				"foo": {},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := t.TempDir()

			for _, name := range tc.files {
				require.NoError(t, os.WriteFile(filepath.Join(repoDir, name), []byte("test"), 0o600))
			}

			require.NoError(t, RemoveUnmanaged(repoDir, tc.conf))

			for _, name := range tc.remaining {
				_, err := os.Stat(filepath.Join(repoDir, name))
				require.NoError(t, err)
			}

			entries, err := os.ReadDir(repoDir)
			require.NoError(t, err)
			require.Len(t, entries, len(tc.remaining))
		})
	}
}
