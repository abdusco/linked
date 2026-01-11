#!/bin/sh
# Build script for the Linked project

set -e

VERSION="${1:-dev}"
BUILD_TIME="${2:-unknown}"

echo "Building Linked..."
echo "Version: $VERSION"
echo "Build Time: $BUILD_TIME"

go build \
    -ldflags="-X main.version=$VERSION -X main.buildTime=$BUILD_TIME" \
    -o linked .

echo "✓ Build completed successfully"

