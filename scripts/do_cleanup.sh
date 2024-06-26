#!/bin/bash
set -euo pipefail

function log() {
  echo "[$(date +%H:%M:%S)] $@" >&2
}

add_opts=()
[[ -z ${REPOKEY:-} ]] || add_opts+=(-s --key ${REPOKEY})

syncdir="${REPO_DIR:-$(pwd)}"

log "Cleaning up old package versions..."
BASE_PATH="${syncdir}" python scripts/remove_old_versions.py

log "Adding remaining packages to database..."
packages=($(find "${syncdir}" -regextype egrep -regex '^.*\.pkg\.tar(\.xz|\.zst)$' | sort))

if [ "${#packages[@]}" -eq 0 ]; then
  log "No packages found to add to repo, this looks like an error!"
  exit 1
fi

log "Adding packages..."
repo-add ${add_opts[@]} --new --prevent-downgrade "${DATABASE}" "${packages[@]}"

log "All packages added, removing *.old copies..."
find "${syncdir}" -name '*.old' -delete
