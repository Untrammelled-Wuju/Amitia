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
PACKAGE_ID=$(jq -r '.packageId' "$LOCK_FILE")
PACKAGE_FILE_NAME=$(jq -r '.packageFileName' "$LOCK_FILE")

echo "Runtime Version: $RUNTIME_VERSION"
echo "Package ID:      $PACKAGE_ID"
echo "Package Format:  $PACKAGE_FORMAT_VERSION"
echo "============================================"

REQUIRED_COMPONENTS=("node" "rootfs" "backend" "qdrant" "plugin-host" "task-host")
for component in "${REQUIRED_COMPONENTS[@]}"; do
    RECORD_PATH="$RUNTIME_ROOT/build/out/$component/linux-arm64/$component-build-record.json"
    if [[ ! -f "$RECORD_PATH" ]]; then
        echo "[FATAL] Missing build record for $component : $RECORD_PATH" >&2
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

ROOTFS_TAR="$RUNTIME_ROOT/build/out/rootfs/linux-arm64/ubuntu-rootfs-arm64.tar.xz"
if [[ ! -f "$ROOTFS_TAR" ]]; then
    echo "[FATAL] Rootfs frozen archive not found. Run prepare-ubuntu-rootfs-arm64.sh first." >&2
    exit 1
fi
ROOTFS_SHA=$(sha256sum "$ROOTFS_TAR" | awk '{print $1}')
echo "[INPUT] Rootfs archive: $ROOTFS_TAR"

BACKEND_TAR="$RUNTIME_ROOT/build/out/backend/linux-arm64/amitia-backend-linux-arm64.tar.xz"
if [[ ! -f "$BACKEND_TAR" ]]; then
    echo "[FATAL] Backend frozen archive not found." >&2
    exit 1
fi

QDRANT_TAR="$RUNTIME_ROOT/build/out/qdrant/linux-arm64/qdrant-linux-arm64.tar.xz"
if [[ ! -f "$QDRANT_TAR" ]]; then
    echo "[FATAL] Qdrant frozen archive not found." >&2
    exit 1
fi

PLUGIN_HOST_DIR="$RUNTIME_ROOT/build/out/plugin-host/linux-arm64"
if [[ ! -d "$PLUGIN_HOST_DIR" ]]; then
    echo "[FATAL] Plugin-host artifacts not found." >&2
    exit 1
fi

TASK_HOST_DIR="$RUNTIME_ROOT/build/out/task-host/linux-arm64"
if [[ ! -d "$TASK_HOST_DIR" ]]; then
    echo "[FATAL] Task-host artifacts not found." >&2
    exit 1
fi

mkdir -p "$OUTPUT_DIR"

BUILD_ID="$(date +%Y%m%d%H%M%S)-$$"
STAGING_PATH="$STAGING_DIR/$BUILD_ID"
rm -rf "$STAGING_PATH"
mkdir -p "$STAGING_PATH"

trap 'rm -rf "$STAGING_PATH"' EXIT

PAYLOAD_DIR="$STAGING_PATH/payload"
METADATA_DIR="$STAGING_PATH/metadata"
RUNTIME_STAGING="$STAGING_PATH/runtime-staging"
mkdir -p "$PAYLOAD_DIR/runtime" "$PAYLOAD_DIR/rootfs" "$METADATA_DIR/component-build-records"
mkdir -p "$RUNTIME_STAGING/backend" "$RUNTIME_STAGING/node" "$RUNTIME_STAGING/qdrant"
mkdir -p "$RUNTIME_STAGING/plugin-host" "$RUNTIME_STAGING/task-host" "$RUNTIME_STAGING/scripts"

echo "[ASSEMBLE] Building runtime program tree..."
cp -a "$NODE_ARTIFACTS/." "$RUNTIME_STAGING/node/"
cp "$BACKEND_TAR" "$RUNTIME_STAGING/backend/backend.tar.xz"
cp "$QDRANT_TAR" "$RUNTIME_STAGING/qdrant/qdrant.tar.xz"
cp -a "$PLUGIN_HOST_DIR/." "$RUNTIME_STAGING/plugin-host/"
cp -a "$TASK_HOST_DIR/." "$RUNTIME_STAGING/task-host/"

echo "[FREEZE] Creating deterministic runtime.tar.xz..."
RUNTIME_TAR_XZ="$PAYLOAD_DIR/runtime/runtime.tar.xz"
(
    cd "$RUNTIME_STAGING"
    tar --sort=name \
        --mtime='UTC 1970-01-01' \
        --owner=0 \
        --group=0 \
        --numeric-owner \
        --format=posix \
        -cJf "$RUNTIME_TAR_XZ" \
        .
)
RUNTIME_SHA=$(sha256sum "$RUNTIME_TAR_XZ" | awk '{print $1}')
echo "[FREEZE] runtime.tar.xz created: $RUNTIME_SHA"

cp "$ROOTFS_TAR" "$PAYLOAD_DIR/rootfs/rootfs.tar.xz"
echo "[INPUT] Rootfs copied to payload"

for component in "${REQUIRED_COMPONENTS[@]}"; do
    RECORD_PATH="$RUNTIME_ROOT/build/out/$component/linux-arm64/$component-build-record.json"
    if [[ -f "$RECORD_PATH" ]]; then
        cp "$RECORD_PATH" "$METADATA_DIR/component-build-records/$component-build-record.json"
    fi
done

echo "[MANIFEST] Generating package-index.json..."
cat > "$METADATA_DIR/package-index.json" << MANIFESTEOF
{
  "schemaVersion": 1,
  "runtimeVersion": "$RUNTIME_VERSION",
  "packageId": "$PACKAGE_ID",
  "packageFormatVersion": $PACKAGE_FORMAT_VERSION,
  "target": {
    "hostPlatform": "android",
    "hostAbi": "arm64-v8a",
    "runtimeKind": "proot",
    "guestPlatform": "linux",
    "guestArchitecture": "arm64"
  },
  "payloads": [
    {
      "role": "rootfs",
      "path": "payload/rootfs/rootfs.tar.xz",
      "sha256": "$ROOTFS_SHA",
      "size": $(stat -c %s "$ROOTFS_TAR" 2>/dev/null || stat -f %z "$ROOTFS_TAR" 2>/dev/null || wc -c < "$ROOTFS_TAR")
    },
    {
      "role": "runtime",
      "path": "payload/runtime/runtime.tar.xz",
      "sha256": "$RUNTIME_SHA",
      "size": $(stat -c %s "$RUNTIME_TAR_XZ" 2>/dev/null || stat -f %z "$RUNTIME_TAR_XZ" 2>/dev/null || wc -c < "$RUNTIME_TAR_XZ")
    }
  ],
  "metadata": [
    {
      "role": "guest-layout",
      "path": "metadata/guest-layout.json",
      "sha256": "placeholder",
      "size": 0
    },
    {
      "role": "mount-contract",
      "path": "metadata/mount-contract.json",
      "sha256": "placeholder",
      "size": 0
    },
    {
      "role": "sha256sums",
      "path": "metadata/SHA256SUMS",
      "sha256": "placeholder",
      "size": 0
    }
  ]
}
MANIFESTEOF

echo "[MANIFEST] Generating component-lock.json..."
cat > "$METADATA_DIR/component-lock.json" << LOCKEOF
{
  "schemaVersion": 1,
  "runtimeVersion": "$RUNTIME_VERSION",
  "packageId": "$PACKAGE_ID",
  "components": [
    {
      "id": "node",
      "version": "$NODE_VERSION",
      "architecture": "arm64",
      "path": "payload/runtime/runtime.tar.xz",
      "sha256": "$RUNTIME_SHA"
    },
    {
      "id": "rootfs",
      "version": "$RUNTIME_VERSION",
      "architecture": "arm64",
      "path": "payload/rootfs/rootfs.tar.xz",
      "sha256": "$ROOTFS_SHA"
    }
  ]
}
LOCKEOF

echo "[VERIFY] Generating SHA256SUMS..."
(cd "$STAGING_PATH" && find payload -type f | sort | xargs sha256sum) > "$METADATA_DIR/SHA256SUMS"
echo "[PASS] SHA256SUMS generated"

PAYLOAD_SHA=$( (cd "$PAYLOAD_DIR" && find . -type f | sort | xargs sha256sum) | sha256sum | awk '{print $1}' )
echo "[VERIFY] Payload tree SHA: $PAYLOAD_SHA"

PACKAGE_PATH="$OUTPUT_DIR/$PACKAGE_FILE_NAME"
TEMP_PACKAGE_PATH="$PACKAGE_PATH.tmp.$$"

echo "[PACK] creating package archive..."
(cd "$STAGING_PATH" && find . -type f | sort | zip -q -@ "$TEMP_PACKAGE_PATH")

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
  "packageId": "$PACKAGE_ID",
  "packageFormatVersion": $PACKAGE_FORMAT_VERSION,
  "guestOs": "linux",
  "guestArchitecture": "arm64",
  "buildMode": "release",
  "package": {
    "file": "$PACKAGE_FILE_NAME",
    "sha256": "$PACKAGE_SHA",
    "size": $PACKAGE_SIZE
  },
  "rootfs": {
    "sha256": "$ROOTFS_SHA"
  },
  "runtimePayload": {
    "sha256": "$RUNTIME_SHA"
  }
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
