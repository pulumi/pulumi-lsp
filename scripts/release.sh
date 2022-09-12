#!/usr/bin/env bash

set -euo pipefail

usage()
{
    echo "Usage: ./scripts/release.sh ACTION VERSION"
    echo ""
    echo "Automates release-related chores"
    echo ""
    echo "Example releasing v3.35.1:"
    echo ""
    echo "./scripts/release.sh start v3.35.1"
    echo "./scripts/release.sh tag v3.35.1"
}

if [ "$#" -ne 2 ]; then
    usage
    exit 1
fi

VERSION="$2"

if [[ "$VERSION" != v* ]]; then
    echo "VERSION must start with a v"
    exit 1
fi

merge_changelogs()
{
    PREV=$(cat CHANGELOG.md)
    echo "# CHANGELOG" > CHANGELOG.md
    echo "" >> CHANGELOG.md
    echo "## $VERSION ($(date "+%Y-%m-%d"))" >> CHANGELOG.md
    echo "" >> CHANGELOG.md
    echo -n "$(cat CHANGELOG_PENDING.md)" >> CHANGELOG.md
    echo "${PREV#\# CHANGELOG}" >> CHANGELOG.md
}

restore_changelog_pending()
{
    echo "### Improvements" >  CHANGELOG_PENDING.md
    echo ""                 >> CHANGELOG_PENDING.md
    echo "### Bug Fixes"    >> CHANGELOG_PENDING.md
    echo ""                 >> CHANGELOG_PENDING.md
}

case "$1" in
    start)
        git fetch origin main
        git checkout main -b release/$VERSION

        merge_changelogs
        git add CHANGELOG.md
        git commit -m "Release ${VERSION}"

        restore_changelog_pending
        git add CHANGELOG_PENDING.md
        git commit -m "Cleanup for ${VERSION} release"

        git push --set-upstream origin release/${VERSION}
        echo ""
        echo "Merge the newly created release/${VERSION} branch and run $0 tag ${VERSION}"
        echo "Make sure to use 'Rebase and merge' option to preserve the commit sequence."
        echo ""
        echo "After release/${VERSION} is merged, run"
        echo "$ $0 tag $VERSION"
        ;;

    tag)
        git fetch origin master

        git tag "$VERSION" origin/main~1
        git push origin "$VERSION"
        ;;

    *)
        echo "Invalid command: $1. Expecting one of: start, tag"
        usage
        exit 1
        ;;
esac
