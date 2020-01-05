#!/bin/bash
set -euo pipefail

source scripts/script_framework.sh

join_by() {
	local d=$1
	shift
	echo -n "$1"
	shift
	printf "%s" "${@/#/$d}"
}

declare -A local_versions
declare -A aur_versions

database=$(find . -maxdepth 1 -mindepth 1 -name '*.db.tar.xz' -or '*.db.tar.zstd')

aur_query=("https://aur.archlinux.org/rpc/?v=5&type=info")

step "Collecting local package versions..."
known_packages=$(tar -tf ${database} | grep -v /desc | sed -E 's@^(.*)-([^-]+-[0-9]+)/$@\1 \2@')

IFS=$'\n'

for package in ${known_packages}; do
	name=$(echo "${package}" | cut -d ' ' -f 1)
	version=$(echo "${package}" | cut -d ' ' -f 2)

	local_versions[${name}]=${version}
	aur_query+=("arg[]=${name}")
done

step "Fetching AUR package versions..."
aur_packages=$(curl -sSfL "$(join_by "&" "${aur_query[@]}")" | jq -r '.results | .[] | .Name + " " + .Version')

step "Collecting AUR package versions..."
for package in ${aur_packages}; do
	name=$(echo "${package}" | cut -d ' ' -f 1)
	version=$(echo "${package}" | cut -d ' ' -f 2)

	aur_versions[${name}]=${version}
done

updates=()

step "Checking for updates..."
for package in "${!local_versions[@]}"; do
	local_version="${local_versions[${package}]}"
	aur_version="${aur_versions[${package}]:-}"

	[[ -n ${aur_version} ]] || {
		error "Package ${package} did not yield a version from AUR (local=${local_version})"
		continue
	}

	[[ ${local_version} == ${aur_version} ]] || {
		warn "Package ${package} needs update (${local_version} => ${aur_version})"
		updates+=("${package}")
		continue
	}

	success "Package ${package} is up-to-date (${local_version})"
done

echo "${updates[@]}"
