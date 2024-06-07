# Luzifer / archrepo

This repository contains the setup and tools to maintain my "private" Archlinux repository with AUR packages and packages I rely on which does not have a place in AUR. This setup is intended for my own use and uses a clean build environment for each package.

> **Warning:** As I'm not a [Trusted User](https://wiki.archlinux.org/index.php/Trusted_Users) and this is not an official Archlinux repository you probably don't want to use this repository directly - at last you don't know whether those packages were tempered with. If you do please keep it civilized: if you plan to use the repository in build causing high amounts of traffic or many machines please don't or at least create a mirror.

## Setup

This repository contains four essential parts:

- The `scripts` folder containing bash scripts to control all actions
- The `Makefile` to orchestrate everything. The main functionality is the `maintenance` target (or just `make`)
- The package lists in `repo-urls`
- The `repo/luzifer.asc` public key

It currently relies on my [`luzifer/arch-repo-builder`](https://github.com/luzifer-docker/arch-repo-builder) docker image which does all of the building within a clean environment for each package.

For the initial setup you need to do some steps:

- Adjust the `Makefile` as you need different `download` and `upload` targets
- Create an empty database `tar -cJf repo/luzifer.db.tar.xz -T /dev/null` (adjust the filename)
- Put the public key for your repo into `repo/luzifer.asc` (filename should match the database, makes it easier to find)
- Set up your `repo-urls` package list: it contains one git repository URL per line (comments allowed)
- Provide a docker daemon and all tools listed in the `check_tools` target of the `Makefile`

Afterwards you should be good to just `make` your first build. Depending on the number of packages you selected to be in your repo you might go and fetch dinner while it builds.

## Maintenance

The repo should be updated on a regular base by just executing `make` on it. This will check for updates of the AUR packages specified in the `aur-packages` list and update them if their version is newer than the local one. Also it will check for new commits in the repos listed in `repo-urls` and build them if there are newer commits than those in the local cache.

## Flaws / Remarks / TODOs

- The whole build already strongly relies on Archlinux tools so will not run on any other distro
- For packages having dynamic `pkgver` calculation the update check will not work properly until the `PKGBUILD` in the repo is updated. You can force a rebuild by removing the corresponding line from the `.repo_cache`
