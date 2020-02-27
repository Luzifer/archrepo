#!/bin/bash
set -euo pipefail

source ./scripts/script_framework.sh

REPO_DIR=${REPO_DIR:-$(pwd)}

PACKAGE=${1:-}
[ -z "${PACKAGE}" ] && fail "No package given as CLI argument"

# Create working dir
TMPDIR="/tmp/aur2repo_${PACKAGE}"
mkdir -p "${TMPDIR}/cfg"

# Ensure cleanup on script exit
function cleanup() {
	rm -rf "${TMPDIR}"
}
trap cleanup EXIT

step "Fetching signing key"
vault read --field=key secret/jenkins/arch-signing >"${TMPDIR}/cfg/signing.asc"

step "Re-fetching Docker image"
docker pull luzifer/arch-repo-builder

step "Building AUR package ${PACKAGE}"
docker run --rm -ti \
	-v "${TMPDIR}/src:/src" \
	-v "${TMPDIR}/cfg:/config" \
	-v "${REPO_DIR}:/repo" \
	-v "$(pwd)/scripts/pacman.conf:/etc/pacman.conf:ro" \
	luzifer/arch-repo-builder \
	"https://aur.archlinux.org/${PACKAGE}.git"
