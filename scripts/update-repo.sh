#!/bin/bash
set -euo pipefail

source ./scripts/script_framework.sh

REPO_DIR=${REPO_DIR:-$(pwd)}

REPO=${1:-}
[ -z "${REPO}" ] && fail "No repo given as CLI argument"

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
docker pull luzifer/arch-repo-builder

step "Building package $(basename ${REPO})"
docker run --rm -ti \
	-v "${TMPDIR}/src:/src" \
	-v "${TMPDIR}/cfg:/config" \
	-v "${REPO_DIR}:/repo" \
	luzifer/arch-repo-builder \
	"${REPO}"
