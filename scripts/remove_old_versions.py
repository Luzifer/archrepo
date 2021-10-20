#!/usr/bin/env python3

import os
import re
import sys

# Available only on Archlinux systems through extra/pyalpm package
from pyalpm import vercmp

FILENAME_REGEX = r'^(?P<pkgname>.*)-(?P<version>(?:[0-9]+:)?(?:[^:-]+)-(?:[0-9]+))-[^-]+\.pkg\.tar\.(?:xz|zst)$'

base_path = os.environ['BASE_PATH'] if 'BASE_PATH' in os.environ else './repo'
retain_versions = int(os.environ['RETAIN']) if 'RETAIN' in os.environ else 2

class VercmpSort:
    def __init__(self, obj, *args):
        self.obj = obj

    def __lt__(self, other):
        return vercmp(self.obj['version'], other.obj['version']) < 0

    def __gt__(self, other):
        return vercmp(self.obj['version'], other.obj['version']) > 0

    def __eq__(self, other):
        return vercmp(self.obj['version'], other.obj['version']) == 0

    def __le__(self, other):
        return vercmp(self.obj['version'], other.obj['version']) <= 0

    def __ge__(self, other):
        return vercmp(self.obj['version'], other.obj['version']) >= 0

    def __ne__(self, other):
        return vercmp(self.obj['version'], other.obj['version']) != 0

def get_package_versions():
    packages = {}
    for filename in os.listdir(base_path):
        match = re.match(FILENAME_REGEX, filename)

        if match is None:
            # Not a package file
            continue

        if match.group('pkgname') not in packages:
            packages[match.group('pkgname')] = []

        packages[match.group('pkgname')].append({
            'filename': filename,
            'version': match.group('version'),
        })

    return packages

def main():
    packages = get_package_versions()
    for pkgname in sorted(packages.keys()):
        versions = packages[pkgname]

        print('')
        print('Analysing package "{}"...'.format(pkgname))

        if len(versions) <= retain_versions:
            print('  Retaining all versions: {}'.format(
                ', '.join([x['version'] for x in versions]),
            ))
            continue

        ndrop = len(versions) - retain_versions

        versions = sorted(versions, key=VercmpSort)
        drop = versions[:ndrop]
        retain = versions[ndrop:]

        print('  Dropped versions: {}'.format(
            ', '.join([x['version'] for x in drop]),
        ))
        print('  Retained versions: {}'.format(
            ', '.join([x['version'] for x in retain]),
        ))

        for filename in [x['filename'] for x in drop]:
            os.unlink(os.path.join(base_path, filename))

    return 0

if __name__ == '__main__':
    sys.exit(main())
