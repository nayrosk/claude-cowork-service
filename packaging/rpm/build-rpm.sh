#!/bin/bash
#
# Build RPM package for claude-cowork-service
#
# Usage: ./build-rpm.sh <binary_path> <version>
#
# Creates claude-cowork-service-<version>-1.x86_64.rpm in the current directory.
#
set -euo pipefail

BINARY="$1"
VERSION="$2"

if [ -z "$BINARY" ] || [ -z "$VERSION" ]; then
    echo "Usage: $0 <binary_path> <version>"
    exit 1
fi

if [ ! -f "$BINARY" ]; then
    echo "ERROR: Binary not found: $BINARY"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Create rpmbuild directory structure
WORK_DIR=$(mktemp -d)
RPM_BUILD="$WORK_DIR/rpmbuild"
mkdir -p "$RPM_BUILD"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

trap 'rm -rf "$WORK_DIR"' EXIT

echo "=== Building claude-cowork-service RPM ==="

# Copy binary and service file to SOURCES
cp "$BINARY" "$RPM_BUILD/SOURCES/cowork-svc-linux"
cp "$REPO_ROOT/dist/claude-cowork.service" "$RPM_BUILD/SOURCES/"

# Copy spec file
cp "$SCRIPT_DIR/claude-cowork-service.spec" "$RPM_BUILD/SPECS/"

# Build RPM
rpmbuild -bb \
    --define "_topdir $RPM_BUILD" \
    --define "pkg_version $VERSION" \
    "$RPM_BUILD/SPECS/claude-cowork-service.spec"

# Copy RPM to current directory
RPM_FILE=$(find "$RPM_BUILD/RPMS" -name "*.rpm" -type f | head -1)
if [ -z "$RPM_FILE" ]; then
    echo "ERROR: No RPM file found after build!"
    exit 1
fi

RPM_BASENAME=$(basename "$RPM_FILE")
cp "$RPM_FILE" "$RPM_BASENAME"

SHA256=$(sha256sum "$RPM_BASENAME" | cut -d' ' -f1)

echo "=== Built ${RPM_BASENAME} ($(du -h "$RPM_BASENAME" | cut -f1)) ==="
echo "  SHA256: $SHA256"

# Write build info
cat > "rpm-info.txt" << EOF
VERSION=$VERSION
RPM=$RPM_BASENAME
SHA256=$SHA256
EOF
