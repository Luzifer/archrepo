#!/bin/bash
set -euo pipefail

[[ $(find . -name '*.tar.zst' | wc -l) -gt 0 ]] || {
	echo "No zst archives found, nothing to worry"
	exit 0
}

[[ $(find . -name '*.tar.xz' | wc -l) -gt 0 ]] && {
	echo "Both XZ and zst archives found, pay attention!"
} || true

[[ $(find . -name '*.db.tar.xz' | wc -l) -gt 0 ]] && [[ $(find . -name '*.db.tar.zst' | wc -l) -gt 0 ]] && {
	echo "Found XZ and zst databases! Check this!"
	exit 1
} || true
