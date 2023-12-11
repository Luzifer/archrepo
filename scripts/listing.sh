#!/usr/bin/env bash
set -euo pipefail

source ./scripts/script_framework.sh

[[ -n ${DATABASE:-} ]] || fatal "Missing DATABASE envvar"

cat scripts/packages.hdr.txt
{
  for pkg in $(tar -tf ${DATABASE} | grep -v '/desc'); do
    buildtime=$(date -u -d @$(tar -xf ${DATABASE} --to-stdout "${pkg}desc" | grep -A1 '%BUILDDATE%' | tail -n1))
    echo -e "$(sed -E 's/(.*)-([^-]+-[0-9]+)\//\1\t\2/' <<<"${pkg}")\t${buildtime}"
  done
} | sort | column -t -s "$(printf "\t")" -N 'Package,Version,Build-Time'
