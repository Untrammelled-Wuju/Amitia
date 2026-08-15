#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOCK_FILE="$RUNTIME_ROOT/build/runtime-package/linux-arm64/runtime-package-lock.json"
OUTPUT_DIR="$RUNTIME_ROOT/build/out/runtime-package/linux-arm64"
STAGING_DIR="$RUNTIME_ROOT/build/staging/runtime-package-linux-arm64"

echo "============================================"
echo " Runtime Package Build - Linux ARM64"
echo "============================================"

if [[ ! -f "$LOCK_FILE" ]]; then
    echo "[FATAL] Lock file not found: $LOCK_FILE" >&2
    exit 1
fi

RUNTIME_VERSION=$(jq -r '.runtimeVersion' "$LOCK_FILE")
PACKAGE_FORMAT_VERSION=$(jq -r '.packageFormatVersion' "$LOCK_FILE")

echo "Runtime Version: $RUNTIME_VERSION"
echo "Package Format:  $PACKAGE_FORMAT_VERSION"
echo "============================================"

REQUIRED_RECORDS=("node" "rootfs")
for component in "${REQUIRED_RECORDS[@]}"; do
    RECORD_PATH="$Runtime_ROOT/build/out/$component/linux-arm64/$component-build-record.json"
    if [[ ! -f "$RECORD_PATH" ]]; then
        echo "[FATAL] Missing build record for $component : $RECORD_PATH" >&2
        echo "[FATAL] All frozen input build records are required. Cannot auto-build missing components." >&2
        exit 1
    fi
    echo "[INPUT] $component build record loaded: $RECORD_PATH"
done

NODE_VERSION=$(jq -r '.components.node.version' "$LOCK_FILE")
NODE_EXPECTED_TREE_SHA=$(jq -r '.components.node.treeSha256' "$LOCK_FILE")

NODE_ARTIFACTS="$RUNTIME_ROOT/build/out/node/linux-arm64/node"
if [[ ! -d "$NODE_ARTIFACTS" ]]; then
    echo "[FATAL] Node frozen artifacts not found: $NODE_ARTIFACTS" >&2
    exit 1
fi

NODE_FILES_SHA="$RUNTIME_ROOT/build/out/node/linux-arm64/node-files.sha256"
if [[ ! -f "$NODE_FILES_SHA" ]]; then
    echo "[FATAL] Node tree manifest not found: $NODE_FILES_SHA" >&2
    exit 1
fi

NODE_ACTUAL_TREE_SHA=$(sha256sum "$NODE_FILES_SHA" | awk '{print $1}')
if [[ "$NODE_ACTUAL_TREE_SHA" != "$NODE_EXPECTED_TREE_SHA" ]]; then
    echo "[FATAL] Node tree SHA mismatch: lock=$NODE_EXPECTED_TREE_SHA actual=$NODE_ACTUAL_TREE_SHA" >&2
    exit 1
fi
echo "[VERIFY] Node tree SHA verified: $NODE_ACTUAL_TREE_SHA"

ROOTFS_TAR="$RUNTIME_ROOT/build/out/rootfs/linux-arm64/ubuntu-rootfs-arm64.tar"
if [[ ! -f "$ROOTFS_TAR" ]]; then
    echo "[FATAL] Rootfs frozen archive not found. Run prepare-ubuntu-rootfs-arm64.sh first." >&2
    exit 1
fi
echo "[INPUT] Rootfs archive: $ROOTFS_TAR"

mkdir -p "$OUTPUT_DIR"

BUILD_ID="$(date +%Y%m%d%H%M%S)-$$"
STAGING_PATH="$STAGING_DIR/$BUILD_ID"
rm -rf "$STAGING_PATH"
mkdir -p "$STAGING_PATH"

trap 'rm -rf "$STAGING_PATH"' EXIT

PAYLOAD_DIR="$STAGING_PATH/payload"
METADATA_DIR="$STAGING_PATH/metadata"
mkdir -p "$PAYLOAD_DIR/program" "$PAYLOAD_DIR/rootfs" "$METADATA_DIR/component-build-records"

cp -a "$NODE_ARTIFACTS" "$PAYLOAD_DIR/program/node"
cp "$NODE_FILES_SHA" "$METADATA_DIR/component-build-records/node-files.sha256"

for component in "${REQUIRED_RECORDS[@]}"; do
    RECORD_PATH="$RUNTIME_ROOT/build/out/$component/linux-arm64/$component-build-record.json"
    if [[ -f "$RECORD_PATH" ]]; then
        cp "$RECORD_PATH" "$METADATA_DIR/component-build-records/$component-build-record.json"
    fi
done

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

cat > "$METADATA_DIR/package-manifest.json" << MANIFESTEOF
{
  "schemaVersion": 1,
  "runtimeVersion": "$RUNTIME_VERSION",
  "packageFormatVersion": $PACKAGE_FORMAT_VERSION,
  "guestOs": "linux",
  "guestArchitecture": "arm64",
  "buildMode": "release",
  "createdAt": "$TIMESTAMP",
  "components": {
    "node": {
      "version": "$NODE_VERSION",
      "treeSha256": "$NODE_EXPECTED_TREE_SHA"
    }
  }
}
MANIFESTEOF

PACKAGE_FILE_NAME="amitia-runtime-${RUNTIME_VERSION}-linux-arm64.zip"
PACKAGE_PATH="$OUTPUT_DIR/$PACKAGE_FILE_NAME"
TEMP_PACKAGE_PATH="$PACKAGE_PATH.tmp.$$"

echo "[PACK] creating package archive..."
(cd "$STAGING_PATH" && zip -rq "$TEMP_PACKAGE_PATH" .)

PACKAGE_SHA=$(sha256sum "$TEMP_PACKAGE_PATH" | awk '{print $1}')
PACKAGE_SIZE=$(stat -c %s "$TEMP_PACKAGE_PATH" 2>/dev/null || stat -f %z "$TEMP_PACKAGE_PATH" 2>/dev/null || wc -c < "$TEMP_PACKAGE_PATH")

[[ -f "$PACKAGE_PATH" ]] && rm -f "$PACKAGE_PATH"
mv "$TEMP_PACKAGE_PATH" "$PACKAGE_PATH"

echo "[PACK] Package created: $PACKAGE_PATH"
echo "[PACK] SHA256: $PACKAGE_SHA"
echo "[PACK] Size: $PACKAGE_SIZE bytes"

cat > "$OUTPUT_DIR/runtime-package-build-record.json" << RECORDEOF
{
  "schemaVersion": 1,
  "runtimeVersion": "$RUNTIME_VERSION",
  "packageFormatVersion": $PACKAGE_FORMAT_VERSION,
  "guestOs": "linux",
  "guestArchitecture": "arm64",
  "buildMode": "release",
  "package": {
    "file": "$PACKAGE_FILE_NAME",
    "sha256": "$PACKAGE_SHA",
    "size": $PACKAGE_SIZE
  },
  "node": {
    "version": "$NODE_VERSION",
    "treeSha256": "$NODE_EXPECTED_TREE_SHA",
    "verifiedSha": "$NODE_ACTUAL_TREE_SHA"
  },
  "createdAt": "$TIMESTAMP"
}
RECORDEOF
echo "[RECORD] runtime-package-build-record.json written"

echo "$PACKAGE_SHA  $PACKAGE_FILE_NAME" > "$OUTPUT_DIR/$PACKAGE_FILE_NAME.sha256"
echo "[DONE] $PACKAGE_FILE_NAME.sha256 written"

echo "============================================"
echo "[DONE] Runtime Package Build Complete"
echo "============================================"
echo " Output: $OUTPUT_DIR"
echo "  $PACKAGE_FILE_NAME"
echo "  $PACKAGE_FILE_NAME.sha256"
echo "  runtime-package-build-record.json"
echo "============================================"
