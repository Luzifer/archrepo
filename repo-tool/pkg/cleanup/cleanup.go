package cleanup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/config"
)

const packageSuffix = ".pkg.tar.zst"

// RemoveUnmanaged removes package artifacts from repoDir that are no longer
// represented in the configuration package list.
func RemoveUnmanaged(repoDir string, conf config.File) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return fmt.Errorf("reading repo dir %q: %w", repoDir, err)
	}

	managedNames := make([]string, 0, len(conf.Packages))
	for name := range conf.Packages {
		managedNames = append(managedNames, name)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, packageSuffix) {
			continue
		}

		if isManagedPackage(name, managedNames) {
			continue
		}

		pkgPath := filepath.Join(repoDir, name)
		if err := os.Remove(pkgPath); err != nil {
			return fmt.Errorf("removing unmanaged package %q: %w", pkgPath, err)
		}

		sigPath := pkgPath + ".sig"
		if err := os.Remove(sigPath); err != nil && !isNotExist(err) {
			return fmt.Errorf("removing unmanaged package signature %q: %w", sigPath, err)
		}
	}

	return nil
}

func isManagedPackage(filename string, managedNames []string) bool {
	for _, name := range managedNames {
		if strings.HasPrefix(filename, name+"-") {
			return true
		}
	}

	return false
}

func isNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || strings.Contains(err.Error(), fs.ErrNotExist.Error()))
}
