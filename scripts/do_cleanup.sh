#!/bin/bash
set -euo pipefail

function log() {
	echo "[$(date +%H:%M:%S)] $@" >&2
}

syncdir="${REPO_DIR:-$(pwd)}"

log "Cleaning up old package versions..."
BASE_PATH="${syncdir}" python scripts/remove_old_versions.py

# Ensure removal of previous repo files
log "Cleaning up old database files..."
find "${syncdir}" -regextype egrep -regex '^.*\.(db|files)(|\.tar|\.tar\.xz|\.tar\.zst)$' -delete

log "Adding remaining packages to database..."
packages=($(find "${syncdir}" -regextype egrep -regex '^.*\.pkg\.tar(\.xz|\.zst)$' | sort))

if [ "${#packages[@]}" -eq 0 ]; then
	log "No packages found to add to repo, this looks like an error!"
	exit 1
fi

log "Adding packages..."
repo-add --new --prevent-downgrade "${syncdir}/luzifer.db.tar.xz" "${packages[@]}"

log "All packages added, removing *.old copies..."
find "${syncdir}" -name '*.old' -delete
