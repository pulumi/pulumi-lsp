#!/usr/bin/env bash

# Get the next minor release based on existing tags.

set -euo pipefail

git fetch --tags
VERSION=$(git tag --list 'v*.*.*' | tail -1)
MAJOR=$(echo $VERSION | sed 's/\..*//')
MAJOR=${MAJOR#v}
MINOR=$(echo ${VERSION#v$MAJOR.} | sed 's/\..*//')

echo $MAJOR.$(($MINOR + 1)).0
