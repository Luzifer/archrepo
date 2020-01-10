#!/bin/bash
set -euo pipefail

cat -s <<EOF >scripts/repoctl.toml
repo = "$(find $(pwd) -mindepth 1 -maxdepth 1 -name '*.db.tar.xz' -or -name '*.db.tar.xstd')"
backup = false
interactive = false
columnate = false
color = "auto"
quiet = false
EOF
