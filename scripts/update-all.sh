#!/usr/bin/env bash
set -euo pipefail

BUILD_IMAGE="git.luzifer.io/registry/arch-repo-builder"
declare -A ICON=(
  ["CHECK"]=$(printf "\e[34m\u3f\e[0m")
  ["FAIL"]=$(printf "\e[31m\uf05e\e[0m")
  ["RUN"]=$(printf "\e[32m\uf2f1\e[0m")
  ["SKIP"]=$(printf "\e[1;30m\uf00c\e[0m")
  ["SUCCESS"]=$(printf "\e[32m\uf00c\e[0m")
  ["WAIT"]=$(printf "\e[1;30m\uf251\e[0m")
)
REPO_DIR=${REPO_DIR:-$(pwd)/repo}

cleanup=()
last_line_len=0

function cleanup() {
  rm -rf "${cleanup[@]}"
}

function main() {
  repo_list=(
    $(grep -v "^#" ./repo-urls | cut -d "#" -f 1)
  )

  docker pull -q "${BUILD_IMAGE}" >/dev/null

  for repo in $(grep -v "^#" ./repo-urls | cut -d "#" -f 1); do
    update ${repo}
    echo
  done
}

function update() {
  repo="${1}"

  write_status CHECK ${repo} "Checking build status..."
  local last_remote_hash=$(git ls-remote ${repo} master | awk '{print $1}')

  if grep -q "${repo}#${last_remote_hash}" .repo_cache; then
    write_status SKIP ${repo} "No changes from last build"
    return
  fi

  write_status RUN ${repo} "Build running..."

  # Create working dir
  local tmpdir="/tmp/aur2repo_$(basename ${repo})"
  mkdir -p "${tmpdir}/cfg"

  # Ensure cleanup on script exit
  cleanup+=("${tmpdir}")

  write_status RUN ${repo} "Fetching signing key..."
  vault read --field=key secret/jenkins/arch-signing >"${tmpdir}/cfg/signing.asc"

  write_status RUN ${repo} "Building package..."
  local extra_opts=()
  if [[ -f /etc/pacman.d/mirrorlist ]]; then
    extra_opts+=(-v "/etc/pacman.d/mirrorlist:/etc/pacman.d/mirrorlist:ro")
  fi

  local container=$(
    docker run -d \
      -v "${tmpdir}/src:/src" \
      -v "${tmpdir}/cfg:/config" \
      -v "${REPO_DIR}:/repo" \
      -v "$(pwd)/scripts/pacman.conf:/etc/pacman.conf:ro" \
      "${extra_opts[@]}" \
      --ulimit nofile=262144:262144 \
      "${BUILD_IMAGE}" \
      "${repo}"
  )

  local status="running"
  while [[ $status == "running" ]]; do
    status=$(docker inspect ${container} | jq -r '.[0].State.Status')
    local started=$(date -d $(docker inspect ${container} |
      jq -r '.[0].State.StartedAt') +%s)
    write_status RUN ${repo} "Building package in container $(printf "%.12s" ${container}) for $(($(date +%s) - started))s..."
    sleep 1
  done

  local exitcode=$(docker inspect ${container} | jq -r '.[0].State.ExitCode')
  if [ $exitcode -gt 0 ]; then
    local logfile="/tmp/arch-package-build_$(basename ${repo}).log"
    docker logs ${container} 2>&1 >${logfile}
    write_status FAIL ${repo} "Build failed (${exitcode}), see logs at ${logfile}"
  else
    write_status SUCCESS ${repo} "Updating cache entry..."

    grep -v "^${repo}#" .repo_cache >.repo_cache.tmp || true
    echo "${repo}#${last_remote_hash}" >>.repo_cache.tmp
    mv .repo_cache.tmp .repo_cache

    write_status SUCCESS ${repo} "Build succeeded"
  fi

  docker rm ${container} >/dev/null
}

function write_status() {
  local icon="${ICON[$1]}"
  local name="${2%%.git}"
  local comment="$3"

  # Wipe previous printed line
  local wipe=$(echo -en "\r$(printf "%-${last_line_len}s" "")")

  # Assemble new line
  local line=$(echo -en "\r ${icon} \e[1m${name##*/}\e[0m - ${comment}")
  # Count line length for later wipe
  last_line_len=$(wc -c <<<"${line}")
  # Print line
  echo -en "${wipe}${line}"
}

trap cleanup EXIT
main
