#!/bin/bash
set -euo pipefail

source ./scripts/script_framework.sh

REPO_DIR=${REPO_DIR:-$(pwd)}

REPO=${1:-}
[ -z "${REPO}" ] && fail "No repo given as CLI argument"

step "Checking for changes from last build"
last_remote_hash=$(git ls-remote ${REPO} master | awk '{print $1}')
grep "${REPO}#${last_remote_hash}" .repo_cache && {
  warn "Remote has no changes from last build, skipping..."
  exit 0
} || true

# Create working dir
TMPDIR="/tmp/aur2repo_$(basename ${REPO})"
mkdir -p "${TMPDIR}/cfg"

# Ensure cleanup on script exit
function cleanup() {
  rm -rf "${TMPDIR}"
}
trap cleanup EXIT

step "Fetching signing key"
vault read --field=key secret/jenkins/arch-signing >"${TMPDIR}/cfg/signing.asc"

step "Re-fetching Docker image"
docker pull gcr.io/luzifer-registry/arch-repo-builder

step "Building package $(basename ${REPO})"
docker run --rm -ti \
  -v "${TMPDIR}/src:/src" \
  -v "${TMPDIR}/cfg:/config" \
  -v "${REPO_DIR}:/repo" \
  -v "$(pwd)/scripts/pacman.conf:/etc/pacman.conf:ro" \
  --ulimit nofile=262144:262144 \
  gcr.io/luzifer-registry/arch-repo-builder \
  "${REPO}"

step "Updating cache entry"
grep -v "^${REPO}#" .repo_cache >.repo_cache.tmp || true
echo "${REPO}#${last_remote_hash}" >>.repo_cache.tmp
mv .repo_cache.tmp .repo_cache
