package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/builder"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/cache"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/cleanup"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/config"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/status"
	"git.luzifer.io/luzifer/archrepo/repo-tool/pkg/vault"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/Luzifer/rconfig/v2"
)

var (
	cfg = struct {
		BuildImage              string `flag:"build-image" vardefault:"buildimg" description:"Image to use for Arch-RepoBuilder"`
		Cache                   string `flag:"cache" default:".repo_cache.yaml" description:"Cache file to hold the previous builds"`
		Config                  string `flag:"config,c" default:"repo-urls.yaml" description:"Configuration holding the packages to build"`
		ErrorOnFailedBuilds     bool   `flag:"error-on-failed-builds,e" default:"false" description:"Exit non-zero when any build failed"`
		LogLevel                string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		PacmanConf              string `flag:"pacman-config" default:"scripts/pacman.conf" description:"Which pacman config to mount into the build container"`
		RemoveUnmanagedPackages bool   `flag:"remove-unmanaged-packages" default:"true" description:"Remove packages not mentioned in the config"`
		RepoDir                 string `flag:"repo-dir" default:"repo" description:"Where to put the repository"`
		ShowAlreadyBuilt        bool   `flag:"show-already-built" default:"false" description:"Show output for packages previously built without updates"`
		SigningVaultKey         string `flag:"signing-vault-key" vardefault:"signkey" description:"Where to find the signing key"`
		VersionAndExit          bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"
)

func initApp() error {
	rconfig.SetVariableDefaults(map[string]string{
		"buildimg": "ghcr.io/luzifer-docker/arch-repo-builder:latest",
		"signkey":  "secret/jenkins/arch-signing",
	})

	rconfig.AutoEnv(true)
	if err := rconfig.ParseAndValidate(&cfg); err != nil {
		return errors.Wrap(err, "parsing cli options")
	}

	l, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return errors.Wrap(err, "parsing log-level")
	}
	logrus.SetLevel(l)

	return nil
}

func main() {
	if err := initApp(); err != nil {
		logrus.WithError(err).Fatal("initializing app")
	}

	if cfg.VersionAndExit {
		logrus.WithField("version", version).Info("repo-tool")
		os.Exit(0)
	}

	exitCode, err := run(context.Background())
	if err != nil {
		logrus.WithError(err).Error("running repo-tool")
	}

	os.Exit(exitCode)
}

func run(ctx context.Context) (exitCode int, err error) {
	conf, err := config.Load(cfg.Config)
	if err != nil {
		return 1, fmt.Errorf("loading config: %w", err)
	}

	buildCache, err := cache.Load(cfg.Cache)
	if err != nil {
		return 1, fmt.Errorf("loading build-cache: %w", err)
	}

	var buildResult builder.BuildResult
	if err = vault.WithSigningKey(cfg.SigningVaultKey, func(keyFilePath string) error {
		buildResult, err = builder.RunBuild(ctx, builder.BuildOpts{
			Config:             conf,
			BuildImage:         cfg.BuildImage,
			Cache:              buildCache,
			Display:            status.NewDisplay(),
			DiplayAlreadyBuilt: cfg.ShowAlreadyBuilt,
			PacmanConf:         cfg.PacmanConf,
			RepoDir:            cfg.RepoDir,
			SigningKeyPath:     keyFilePath,
		})
		if err != nil {
			return fmt.Errorf("running builds: %w", err)
		}

		return nil
	}); err != nil {
		return 1, fmt.Errorf("running builders with signing key: %w", err)
	}

	if err = buildCache.Save(); err != nil {
		return 1, fmt.Errorf("saving build-cache: %w", err)
	}

	if cfg.RemoveUnmanagedPackages {
		if err = cleanup.RemoveUnmanaged(cfg.RepoDir, conf); err != nil {
			return 1, fmt.Errorf("removing unmanaged packages: %w", err)
		}
	}

	if !buildResult.HasFailures() {
		return 0, nil
	}

	failedPackages := make([]string, 0, len(buildResult.Failures))
	for _, failure := range buildResult.Failures {
		failedPackages = append(failedPackages, failure.Name)
	}

	logrus.WithFields(logrus.Fields{
		"count":    len(buildResult.Failures),
		"packages": strings.Join(failedPackages, ","),
	}).Error("builds completed with failures")

	if cfg.ErrorOnFailedBuilds {
		// Flags asked to exit with error status for failed builds
		return 1, nil
	}

	return 0, nil
}
