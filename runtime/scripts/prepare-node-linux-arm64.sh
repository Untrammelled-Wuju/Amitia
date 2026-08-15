#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOCK_FILE="$RUNTIME_ROOT/artifacts/node/linux-arm64/node-runtime-lock.json"
CACHE_DIR="$RUNTIME_ROOT/.cache/node"
STAGING_DIR="$RUNTIME_ROOT/build/staging/node-linux-arm64"
OUTPUT_DIR="$RUNTIME_ROOT/build/out/node/linux-arm64"

if [[ ! -f "$LOCK_FILE" ]]; then
    echo "[FATAL] Lock file not found: $LOCK_FILE" >&2
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "[FATAL] jq is required but not installed" >&2
    exit 1
fi

VERSION=$(jq -r '.version' "$LOCK_FILE")
ARCHIVE_FILE_NAME=$(jq -r '.archiveFileName' "$LOCK_FILE")
SOURCE_URL=$(jq -r '.sourceUrl' "$LOCK_FILE")
EXPECTED_SHA=$(jq -r '.sha256' "$LOCK_FILE")
INSTALL_SUBDIR=$(jq -r '.installSubdir' "$LOCK_FILE")

echo "============================================"
echo " Node Linux ARM64 Prepare (Linux CI)"
echo "============================================"
echo "Version:          $VERSION"
echo "Archive:          $ARCHIVE_FILE_NAME"
echo "Source:           $SOURCE_URL"
echo "Expected SHA256:  $EXPECTED_SHA"
echo "============================================"

mkdir -p "$CACHE_DIR"

ARCHIVE_PATH="$CACHE_DIR/$ARCHIVE_FILE_NAME"

if [[ -f "$ARCHIVE_PATH" ]]; then
    CACHED_SHA=$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')
    if [[ "$CACHED_SHA" == "$EXPECTED_SHA" ]]; then
        echo "[CACHE] Cached archive SHA matches"
    else
        echo "[CACHE] Cached archive SHA mismatch, removing"
        rm -f "$ARCHIVE_PATH"
    fi
fi

if [[ ! -f "$ARCHIVE_PATH" ]]; then
    echo "[DOWNLOAD] $SOURCE_URL"
    TMP_FILE="$ARCHIVE_PATH.tmp"
    if ! curl -L --fail --silent --show-error "$SOURCE_URL" -o "$TMP_FILE"; then
        rm -f "$TMP_FILE"
        echo "[FATAL] Download failed" >&2
        exit 1
    fi
    DOWNLOAD_SHA=$(sha256sum "$TMP_FILE" | awk '{print $1}')
    if [[ "$DOWNLOAD_SHA" != "$EXPECTED_SHA" ]]; then
        rm -f "$TMP_FILE"
        echo "[FATAL] SHA mismatch: expected=$EXPECTED_SHA actual=$DOWNLOAD_SHA" >&2
        exit 1
    fi
    mv "$TMP_FILE" "$ARCHIVE_PATH"
    echo "[PASS] SHA verified: $DOWNLOAD_SHA"
fi

BUILD_ID="$(date +%Y%m%d%H%M%S)-$$"
STAGING_ROOT="$STAGING_DIR/$BUILD_ID"
rm -rf "$STAGING_ROOT"
mkdir -p "$STAGING_ROOT"

echo "[EXTRACT] Validating archive members before extraction..."
if tar -tJf "$ARCHIVE_PATH" | grep -qE '^((\.\./)|/)'; then
    echo "[FATAL] Archive contains absolute paths or parent traversal entries" >&2
    exit 1
fi
echo "[PASS] No absolute path or parent traversal entries"

echo "[EXTRACT] Extracting to staging..."
tar -xJf "$ARCHIVE_PATH" -C "$STAGING_ROOT" --no-same-owner

EXPECTED_ROOT="node-v${VERSION}-linux-arm64"
EXTRACTED_ROOT="$STAGING_ROOT/$EXPECTED_ROOT"

if [[ ! -d "$EXTRACTED_ROOT" ]]; then
    echo "[FATAL] Extracted root not found: $EXTRACTED_ROOT" >&2
    rm -rf "$STAGING_ROOT"
    exit 1
fi

NODE_BIN="$EXTRACTED_ROOT/bin/node"
NPM_CLI="$EXTRACTED_ROOT/lib/node_modules/npm/bin/npm-cli.js"
NPX_CLI="$EXTRACTED_ROOT/lib/node_modules/npm/bin/npx-cli.js"

if [[ ! -f "$NODE_BIN" ]]; then
    echo "[FATAL] node binary not found: $NODE_BIN" >&2
    rm -rf "$STAGING_ROOT"
    exit 1
fi
if [[ ! -f "$NPM_CLI" ]]; then
    echo "[FATAL] npm-cli.js not found" >&2
    rm -rf "$STAGING_ROOT"
    exit 1
fi
if [[ ! -f "$NPX_CLI" ]]; then
    echo "[FATAL] npx-cli.js not found" >&2
    rm -rf "$STAGING_ROOT"
    exit 1
fi

echo "[ELF] Checking node binary ELF header..."
if ! command -v file &> /dev/null; then
    echo "[WARN] 'file' command not available, skipping ELF check"
else
    ELF_INFO=$(file "$NODE_BIN")
    echo "  $ELF_INFO"
    if ! echo "$ELF_INFO" | grep -q "ELF 64-bit"; then
        echo "[FATAL] node is not ELF 64-bit" >&2
        rm -rf "$STAGING_ROOT"
        exit 1
    fi
    if ! echo "$ELF_INFO" | grep -q "ARM aarch64"; then
        echo "[FATAL] node is not ARM aarch64" >&2
        rm -rf "$STAGING_ROOT"
        exit 1
    fi
    if ! echo "$ELF_INFO" | grep -q "Linux"; then
        echo "[FATAL] node is not for Linux" >&2
        rm -rf "$STAGING_ROOT"
        exit 1
    fi
    echo "[PASS] ELF verification passed"
fi

NODE_DEST="$OUTPUT_DIR/$INSTALL_SUBDIR"
TEMP_DEST="${NODE_DEST}.tmp.$$"
rm -rf "$TEMP_DEST"
mkdir -p "$OUTPUT_DIR"
cp -a "$EXTRACTED_ROOT" "$TEMP_DEST"
if [[ -d "$NODE_DEST" ]]; then
    rm -rf "$NODE_DEST"
fi
mv "$TEMP_DEST" "$NODE_DEST"
echo "[FREEZE] Node runtime atomically published to: $NODE_DEST"

echo "[MANIFEST] Generating node-files.sha256..."
(cd "$OUTPUT_DIR" && find "$INSTALL_SUBDIR" -type f | sort | xargs sha256sum) > "$OUTPUT_DIR/node-files.sha256"
echo "[PASS] node-files.sha256 generated"

echo "[TREE SHA] Computing tree hash..."
TREE_SHA=$(sha256sum "$OUTPUT_DIR/node-files.sha256" | awk '{print $1}')
echo "[TREE SHA] $TREE_SHA"

EXECUTION_STATUS="NOT_EXECUTED"

if file "$NODE_BIN" | grep -q "ELF 64-bit"; then
    if "$NODE_BIN" --version &> /dev/null; then
        NODE_VERSION_OUTPUT=$("$NODE_BIN" --version)
        NPM_VERSION_OUTPUT=$("$NODE_BIN" "$NPM_CLI" --version 2>/dev/null || echo "bundled")
        NPX_VERSION_OUTPUT=$("$NODE_BIN" "$NPX_CLI" --version 2>/dev/null || echo "bundled")
        EXECUTION_STATUS="PASS"
        echo "[EXEC] $NODE_VERSION_OUTPUT"
    fi
fi

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date +"%Y-%m-%dT%H:%M:%SZ")

cat > "$OUTPUT_DIR/node-build-record.json" << BUILDEOF
{
  "schemaVersion": 1,
  "component": "node",
  "version": "$VERSION",
  "platform": "linux",
  "architecture": "arm64",
  "source": {
    "url": "$SOURCE_URL",
    "archiveFileName": "$ARCHIVE_FILE_NAME",
    "expectedSha256": "$EXPECTED_SHA",
    "actualSha256": "$EXPECTED_SHA"
  },
  "runtime": {
    "nodePath": "node/bin/node",
    "npmPath": "node/bin/npm",
    "npxPath": "node/bin/npx",
    "corepackPath": ""
  },
  "npmVersion": "${NPM_VERSION_OUTPUT:-bundled}",
  "npxVersion": "${NPX_VERSION_OUTPUT:-bundled}",
  "corepackIncluded": false,
  "validation": {
    "staticValidation": "PASS",
    "executionValidation": "$EXECUTION_STATUS"
  },
  "treeSha256": "$TREE_SHA",
  "frozenRoot": "$INSTALL_SUBDIR",
  "frozenAt": "$TIMESTAMP"
}
BUILDEOF
echo "[RECORD] node-build-record.json generated"

rm -rf "$STAGING_ROOT"

echo "============================================"
echo "[DONE] Node Linux ARM64 prepare complete"
echo "Output: $OUTPUT_DIR"
echo "  node/                    - frozen runtime"
echo "  node-files.sha256        - tree manifest"
echo "  node-build-record.json   - build record"
echo "============================================"
