#!/bin/bash
set -euo pipefail

[[ $(find . -name '*.tar.zstd' | wc -l) -gt 0 ]] || {
	echo "No ZStd archives found, nothing to worry"
	exit 0
}

[[ $(find . -name '*.tar.xz' | wc -l) -gt 0 ]] && {
	echo "Both XZ and ZStd archives found, pay attention!"
}

[[ $(find . -name '*.db.tar.xz' | wc -l) -gt 0 ]] && [[ $(find . -name '*.db.tar.zstd' | wc -l) -gt 0 ]] && {
	echo "Found XZ and ZStd databases! Check this!"
	exit 1
}
