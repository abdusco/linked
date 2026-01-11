#!/bin/sh
# Local development build script

set -e

cd "$(dirname "$0")/.."

VERSION="dev"
BUILD_TIME="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

echo "Building Linked (local)..."
./scripts/build.sh "$VERSION" "$BUILD_TIME"

echo ""
echo "✓ You can now run: ./linked"

