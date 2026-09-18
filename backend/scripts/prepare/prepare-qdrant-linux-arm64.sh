#!/bin/bash
# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
# Qdrant Linux ARM64 Artifact Preparation Script
# 用法:
#  在线模式: bash scripts/prepare/prepare-qdrant-linux-arm64.sh
#  离线模式: bash scripts/prepare/prepare-qdrant-linux-arm64.sh --archive /path/to/qdrant-aarch64-unknown-linux-musl.tar.gz

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LOCK_FILE="$PROJECT_ROOT/runtime-artifacts/qdrant-linux-arm64.lock.json"
STAGING_DIR="$PROJECT_ROOT/backend/build/qdrant-linux-arm64-staging"
OUTPUT_DIR="$PROJECT_ROOT/backend/dist/runtime-assets/qdrant-linux-arm64"
CACHE_DIR="$PROJECT_ROOT/backend/build/qdrant-cache"

OFFLINE=false=""
ARCHIVE_PATH=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --offline)
            OFFLINE=true
            shift
            ;;
        --archive)
            ARCHIVE_PATH="$2"
            shift 2
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
done

if [[ ! -f "$LOCK_FILE" ]]; then
    echo "[FATAL] Lock file not found: $LOCK_FILE"
    exit 1
fi

VERSION=$(jq -r '.version' "$LOCK_FILE")
ASSET_NAME=$(jq -r '.releaseAsset' "$LOCK_FILE")
EXPECTED_SHA=$(jq -r '.sha256' "$LOCK_FILE")
SOURCE=$(jq -r '.source' "$LOCK_FILE")

echo "============================================"
echo " Qdrant Linux ARM64 Artifact Preparation"
echo "============================================"
echo "Version:          $VERSION"
echo "Asset:            $ASSET_NAME"
echo "Expected SHA256:  $EXPECTED_SHA"
echo "Source:           $SOURCE"
echo "============================================"

rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR"
mkdir -p "$CACHE_DIR"
mkdir -p "$OUTPUT_DIR"

if [[ -n "$ARCHIVE_PATH" ]]; then
    if [[ ! -f "$ARCHIVE_PATH" ]]; then
        echo "[FATAL] Specified archive not found: $ARCHIVE_PATH"
        exit 1
    fi
    ARCHIVE_FILE="$ARCHIVE_PATH"
    echo "[INPUT] Using local archive: $ARCHIVE_FILE"
else
    ARCHIVE_FILE="$CACHE_DIR/$ASSET_NAME"
    if [[ -f "$ARCHIVE_FILE" ]]; then
        CACHED_SHA=$(sha256sum "$ARCHIVE_FILE" | awk '{print $1}')
        if [[ "$CACHED_SHA" == "$EXPECTED_SHA" ]]; then
            echo "[CACHE] Cached archive SHA matches, skipping download"
        else
            echo "[CACHE] Cached archive SHA mismatch, re-downloading"
            rm -f "$ARCHIVE_FILE"
        fi
    fi

    if [[ ! -f "$ARCHIVE_FILE" ]]; then
        if [[ "$OFFLINE" == "true" ]]; then
            echo "[FATAL] Offline mode and no cached archive available"
            exit 1
        fi
        DOWNLOAD_URL="https://github.com/qdrant/qdrant/releases/download/v${VERSION}/${ASSET_NAME}"
        echo "[DOWNLOAD] $DOWNLOAD_URL"
        curl -L --fail --silent --show-error "$DOWNLOAD_URL" -o "$ARCHIVE_FILE.tmp"
        mv "$ARCHIVE_FILE.tmp" "$ARCHIVE_FILE"
    fi
fi

echo "[VERIFY] Verifying archive SHA256..."
ACTUAL_SHA=$(sha256sum "$ARCHIVE_FILE" | awk '{print $1}')
if [[ "$ACTUAL_SHA" != "$EXPECTED_SHA" ]]; then
    echo "[FATAL] SHA256 mismatch!"
    echo "  Expected: $EXPECTED_SHA"
    echo "  Actual:   $ACTUAL_SHA"
    exit 1
fi
echo "[PASS] SHA256 verified: $ACTUAL_SHA"

echo "[EXTRACT] Extracting to staging..."
tar xzf "$ARCHIVE_FILE" -C "$STAGING_DIR"

BINARY_CANDIDATE=""
if [[ -f "$STAGING_DIR/qdrant" ]]; then
    BINARY_CANDIDATE="$STAGING_DIR/qdrant"
elif [[ -d "$STAGING_DIR/qdrant" && -f "$STAGING_DIR/qdrant/qdrant" ]]; then
    BINARY_CANDIDATE="$STAGING_DIR/qdrant/qdrant"
else
    BINARY_CANDIDATE=$(find "$STAGING_DIR" -name "qdrant" -type f | head -1)
fi

if [[ -z "$BINARY_CANDIDATE" || ! -f "$BINARY_CANDIDATE" ]]; then
    echo "[FATAL] Could not find qdrant binary in extracted archive"
    exit 1
fi
echo "[FOUND] Binary: $BINARY_CANDIDATE"

DIST_ROOT="$STAGING_DIR/qdrant"
BIN_DIR="$DIST_ROOT/bin"
mkdir -p "$BIN_DIR"

if [[ "$BINARY_CANDIDATE" != "$BIN_DIR/qdrant" ]]; then
    cp "$BINARY_CANDIDATE" "$BIN_DIR/qdrant"
fi

echo "[PERMISSION] Setting binary permissions..."
chmod 0755 "$BIN_DIR/qdrant"

echo "[ELF VERIFY] Checking ELF header..."
if ! command -v file &> /dev/null; then
    echo "[WARN] 'file' command not available, skipping ELF check"
else
    ELF_INFO=$(file "$BIN_DIR/qdrant")
    echo "  $ELF_INFO"

    if ! echo "$ELF_INFO" | grep -q "ELF 64-bit"; then
        echo "[FATAL] Binary is not ELF 64-bit"
        exit 1
    fi
    if ! echo "$ELF_INFO" | grep -q "ARM aarch64"; then
        echo "[FATAL] Binary is not ARM aarch64"
        exit 1
    fi
    if ! echo "$ELF_INFO" | grep -q "Linux"; then
        echo "[FATAL] Binary is not for Linux"
        exit 1
    fi
    echo "[PASS] ELF verification passed"
fi

if command -v readelf &> /dev/null; then
    echo "[DEPENDENCY] Checking dynamic dependencies..."
    readelf -d "$BIN_DIR/qdrant" 2>/dev/null | grep "NEEDED" || echo "  (static binary or no dynamic dependencies)"

    INTERPRETER=$(readelf -l "$BIN_DIR/qdrant" 2>/dev/null | grep "interpreter" | sed 's/.*: \(.*\)]/\1/')
    if [[ -n "$INTERPRETER" ]]; then
        echo "  Interpreter: $INTERPRETER"
    fi
fi

BINARY_SHA=$(sha256sum "$BIN_DIR/qdrant" | awk '{print $1}')
BINARY_SIZE=$(stat -c%s "$BIN_DIR/qdrant" 2>/dev/null || stat -f%z "$BIN_DIR/qdrant" 2>/dev/null)
BINARY_MODE=$(stat -c%a "$BIN_DIR/qdrant" 2>/dev/null || stat -f%Lp "$BIN_DIR/qdrant" 2>/dev/null)

echo "============================================"
echo " Binary Information"
echo "============================================"
echo " Path: $BIN_DIR/qdrant"
echo " SHA256: $BINARY_SHA"
echo " Size: $BINARY_SIZE bytes"
echo " Mode: $BINARY_MODE"
echo "============================================"

TREE_SHA=$(cd "$DIST_ROOT" && find . -type f | sort | xargs sha256sum | sha256sum | awk '{print $1}')
echo " Distribution Tree SHA256: $TREE_SHA"

cat > "$STAGING_DIR/qdrant-artifact.json" << EOF
{
  "schemaVersion": 1,
  "component": "qdrant",
  "version": "$VERSION",
  "source": "$SOURCE",
  "os": "linux",
  "architecture": "arm64",
  "releaseAsset": "$ASSET_NAME",
  "archiveSha256": "$ACTUAL_SHA",
  "binary": "qdrant/bin/qdrant",
  "binarySha256": "$BINARY_SHA",
  "binarySize": $BINARY_SIZE,
  "mode": "0755",
  "elfClass": "ELF64",
  "elfMachine": "AArch64",
  "linkMode": "static",
  "distributionTreeSha256": "$TREE_SHA"
}
EOF

echo "$BINARY_SHA  qdrant/bin/qdrant" > "$STAGING_DIR/SHA256SUMS"

echo "============================================"
echo " Artifact Metadata"
echo "============================================"
cat "$STAGING_DIR/qdrant-artifact.json"
echo ""
cat "$STAGING_DIR/SHA256SUMS"
echo "============================================"

rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"
cp -r "$STAGING_DIR/qdrant" "$OUTPUT_DIR/"
cp "$STAGING_DIR/qdrant-artifact.json" "$OUTPUT_DIR/"
cp "$STAGING_DIR/SHA256SUMS" "$OUTPUT_DIR/"

rm -rf "$STAGING_DIR"

echo "[PUBLISH] Artifact published to: $OUTPUT_DIR"
echo ""
echo " Contents:"
ls -la "$OUTPUT_DIR/"
echo ""
echo " Binary:"
ls -la "$OUTPUT_DIR/qdrant/bin/"
echo ""
echo "[DONE] Qdrant Linux ARM64 Artifact preparation complete"
