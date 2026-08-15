#!/bin/bash
# Ubuntu ARM64 Rootfs Prepare Script
# Usage:
#   Online mode:  bash runtime/scripts/prepare/prepare-ubuntu-rootfs-arm64.sh
#   Offline mode: bash runtime/scripts/prepare/prepare-ubuntu-rootfs-arm64.sh --offline

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"
LOCK_FILE="$PROJECT_ROOT/runtime/artifacts/ubuntu-rootfs/linux-arm64/ubuntu-rootfs-lock.json"
POLICY_FILE="$PROJECT_ROOT/runtime/artifacts/ubuntu-rootfs/linux-arm64/rootfs-policy.json"

RELEASE_MODE=true
OFFLINE=false
CACHE_DIR=""
STAGING_DIR=""
OUTPUT_DIR=""
SKIP_VERIFY=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --offline)
            OFFLINE=true
            shift
            ;;
        --cache-dir)
            CACHE_DIR="$2"
            shift 2
            ;;
        --staging-dir)
            STAGING_DIR="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --skip-verify)
            if [[ "$RELEASE_MODE" == "true" ]]; then
                echo "[FATAL] --skip-verify is not allowed in release mode" >&2
                exit 1
            fi
            SKIP_VERIFY=true
            shift
            ;;
        --dev-mode)
            RELEASE_MODE=false
            shift
            ;;
        *)
            echo "Unknown parameter: $1"
            exit 1
            ;;
    esac
done

if [[ ! -f "$LOCK_FILE" ]]; then
    echo "[FATAL] Lock file not found: $LOCK_FILE"
    exit 1
fi

RELEASE=$(jq -r '.release' "$LOCK_FILE")
ARCHIVE_FILE_NAME=$(jq -r '.archiveFileName' "$LOCK_FILE")
EXPECTED_SHA=$(jq -r '.sha256' "$LOCK_FILE")
SOURCE_URL=$(jq -r '.sourceUrl' "$LOCK_FILE")

echo "============================================"
echo " Ubuntu ARM64 Rootfs Prepare"
echo "============================================"
echo " Release:        $RELEASE"
echo " Archive:        $ARCHIVE_FILE_NAME"
echo " Expected SHA256: $EXPECTED_SHA"
echo " Source:         $SOURCE_URL"
echo "============================================"

if [[ -n "$CACHE_DIR" ]]; then
    CACHE_PATH="$CACHE_DIR"
else
    CACHE_PATH="$PROJECT_ROOT/runtime/.cache/ubuntu-rootfs"
fi
if [[ -n "$STAGING_DIR" ]]; then
    STAGING_PATH="$STAGING_DIR"
else
    BUILD_ID="$(date +%Y%m%d%H%M%S)"
    STAGING_PATH="$PROJECT_ROOT/runtime/build/staging/ubuntu-rootfs-arm64/$BUILD_ID"
fi
if [[ -n "$OUTPUT_DIR" ]]; then
    OUTPUT_PATH="$OUTPUT_DIR"
else
    OUTPUT_PATH="$PROJECT_ROOT/runtime/build/out/rootfs/linux-arm64"
fi

mkdir -p "$CACHE_PATH"
mkdir -p "$STAGING_PATH"
mkdir -p "$OUTPUT_PATH"

ARCHIVE_FILE="$CACHE_PATH/$ARCHIVE_FILE_NAME"

if [[ -f "$ARCHIVE_FILE" ]]; then
    CACHED_SHA=$(sha256sum "$ARCHIVE_FILE" | awk '{print $1}')
    if [[ "$CACHED_SHA" == "$EXPECTED_SHA" ]]; then
        echo "[CACHE] Cached archive SHA matches"
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
    echo "[DOWNLOAD] $SOURCE_URL"
    TMP_FILE="$ARCHIVE_FILE.tmp"
    if ! curl -L --fail --silent --show-error "$SOURCE_URL" -o "$TMP_FILE"; then
        echo "[FATAL] Download failed"
        rm -f "$TMP_FILE"
        exit 1
    fi
    DOWNLOAD_SHA=$(sha256sum "$TMP_FILE" | awk '{print $1}')
    if [[ "$DOWNLOAD_SHA" != "$EXPECTED_SHA" ]]; then
        echo "[FATAL] SHA256 mismatch after download! Expected: $EXPECTED_SHA, Got: $DOWNLOAD_SHA"
        rm -f "$TMP_FILE"
        exit 1
    fi
    mv "$TMP_FILE" "$ARCHIVE_FILE"
fi

echo "[VERIFY] Verifying archive SHA256..."
ACTUAL_SHA=$(sha256sum "$ARCHIVE_FILE" | awk '{print $1}')
if [[ "$ACTUAL_SHA" != "$EXPECTED_SHA" ]]; then
    echo "[FATAL] SHA256 mismatch! Expected: $EXPECTED_SHA, Got: $ACTUAL_SHA"
    exit 1
fi
echo "[PASS] SHA256 verified: $ACTUAL_SHA"

echo "[EXTRACT] Validating archive members before extraction..."
if tar -tzf "$ARCHIVE_FILE" | grep -qE '^((\.\./)|/)'; then
    echo "[FATAL] Archive contains absolute paths or parent traversal entries" >&2
    exit 1
fi
echo "[PASS] No absolute path or parent traversal entries"

echo "[EXTRACT] Extracting to staging..."
EXTRACT_DIR="$STAGING_PATH/rootfs"
if [[ -d "$EXTRACT_DIR" ]]; then
    rm -rf "$EXTRACT_DIR"
fi
mkdir -p "$EXTRACT_DIR"

if ! tar xzf "$ARCHIVE_FILE" -C "$EXTRACT_DIR" --no-same-owner; then
    echo "[FATAL] Extraction failed"
    exit 1
fi

echo "[EXTRACT] Extraction complete: $EXTRACT_DIR"

echo "[CLEAN] Cleaning rootfs before freeze..."
rm -rf "$EXTRACT_DIR/tmp"/* 2>/dev/null || true
rm -rf "$EXTRACT_DIR/var/tmp"/* 2>/dev/null || true
rm -rf "$EXTRACT_DIR/var/cache/apt"/* 2>/dev/null || true
rm -f "$EXTRACT_DIR/etc/machine-id" 2>/dev/null || true
rm -f "$EXTRACT_DIR/etc/ssh/ssh_host_"* 2>/dev/null || true
rm -rf "$EXTRACT_DIR/root/.bash_history" 2>/dev/null || true
rm -rf "$EXTRACT_DIR/root/.cache" 2>/dev/null || true
find "$EXTRACT_DIR" -name "*.log" -delete 2>/dev/null || true
find "$EXTRACT_DIR/var/log" -type f -delete 2>/dev/null || true
echo "[CLEAN] Rootfs cleaned"

if [[ "$SKIP_VERIFY" == "false" ]]; then
    echo "[VERIFY] Running static validation on final rootfs tree..."
    VALIDATOR_PATH="$PROJECT_ROOT/runtime/validation/linux-arm64/rootfs_validator.py"
    if [[ -f "$VALIDATOR_PATH" ]]; then
        if ! python "$VALIDATOR_PATH" --rootfs "$EXTRACT_DIR" --lock "$LOCK_FILE" --policy "$POLICY_FILE"; then
            echo "[FATAL] Validation failed"
            exit 1
        fi
        echo "[PASS] Static validation passed"
    else
        echo "[SKIP] Validator not found: $VALIDATOR_PATH"
    fi
fi

echo "============================================"
echo " Ubuntu ARM64 Rootfs Prepare Complete"
echo "============================================"
echo " Staging: $EXTRACT_DIR"
echo "============================================"

echo "[FREEZE] Generating tree manifest with filesystem semantics..."
generate_tree_manifest() {
    local dir="$1"
    local output="$2"
    > "$output"
    (
        cd "$dir"
        find . -print0 | sort -z | while IFS= read -r -d '' entry; do
            case "$entry" in
                .) continue ;;
                ./) continue ;;
            esac
            local rel_path="${entry#./}"
            if [[ -L "$entry" ]]; then
                local target
                target=$(readlink "$entry")
                echo "L $(stat -c '%a' "$entry") $target $rel_path" >> "$output"
            elif [[ -d "$entry" ]]; then
                echo "D $(stat -c '%a' "$entry") - $rel_path" >> "$output"
            elif [[ -f "$entry" ]]; then
                local sha
                sha=$(sha256sum "$entry" | awk '{print $1}')
                echo "F $(stat -c '%a' "$entry") $sha $rel_path" >> "$output"
            fi
        done
    )
}
generate_tree_manifest "$EXTRACT_DIR" "$OUTPUT_PATH/rootfs-files.sha256"
echo "[PASS] rootfs-files.sha256 generated"

FROZEN_TAR_NAME="ubuntu-rootfs-arm64.tar"
FROZEN_TAR_PATH="$OUTPUT_PATH/$FROZEN_TAR_NAME"
TEMP_TAR_PATH="$FROZEN_TAR_PATH.tmp.$$"

echo "[FREEZE] Creating deterministic frozen tar archive (preserving symlinks/dirs/modes)..."
(
    cd "$EXTRACT_DIR"
    tar --sort=name \
        --mtime='UTC 1970-01-01' \
        --owner=0 \
        --group=0 \
        --numeric-owner \
        --format=posix \
        -cf "$TEMP_TAR_PATH" \
        .
)
FROZEN_SHA=$(sha256sum "$TEMP_TAR_PATH" | awk '{print $1}')

[[ -f "$FROZEN_TAR_PATH" ]] && rm -f "$FROZEN_TAR_PATH"
mv "$TEMP_TAR_PATH" "$FROZEN_TAR_PATH"
echo "[FREEZE] Frozen archive created: $FROZEN_TAR_PATH"
echo "[FREEZE] Frozen SHA256: $FROZEN_SHA"

echo "[TREE SHA] Computing tree hash..."
TREE_SHA=$(sha256sum "$OUTPUT_PATH/rootfs-files.sha256" | awk '{print $1}')
echo "[TREE SHA] $TREE_SHA"

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

FINAL_RECORD="$OUTPUT_PATH/rootfs-build-record.json"
cat > "$FINAL_RECORD" << ENDOFJSON
{
  "schemaVersion": 1,
  "component": "ubuntu-rootfs",
  "distribution": "$(jq -r '.distribution' "$LOCK_FILE")",
  "flavor": "$(jq -r '.flavor' "$LOCK_FILE")",
  "release": "$RELEASE",
  "codename": "$(jq -r '.codename' "$LOCK_FILE")",
  "architecture": "$(jq -r '.architecture' "$LOCK_FILE")",
  "guestPlatform": "$(jq -r '.guestPlatform' "$LOCK_FILE")",
  "runtimeKind": "$(jq -r '.runtimeKind' "$LOCK_FILE")",
  "source": {
    "url": "$SOURCE_URL",
    "archiveFileName": "$ARCHIVE_FILE_NAME",
    "expectedSha256": "$EXPECTED_SHA",
    "actualSha256": "$ACTUAL_SHA"
  },
  "frozen": {
    "archiveFileName": "$FROZEN_TAR_NAME",
    "archiveSha256": "$FROZEN_SHA",
    "treeSha256": "$TREE_SHA"
  },
  "outputPath": "$OUTPUT_PATH",
  "timestamp": "$TIMESTAMP",
  "offline": $OFFLINE,
  "buildMode": "$([[ "$RELEASE_MODE" == "true" ]] && echo "release" || echo "dev")"
}
ENDOFJSON
echo "[RECORD] Final build record: $FINAL_RECORD"

echo "============================================"
echo "[DONE] Ubuntu ARM64 Rootfs Freeze Complete"
echo "============================================"
echo " Output: $OUTPUT_PATH"
echo "  $FROZEN_TAR_NAME   - frozen rootfs archive"
echo "  rootfs-files.sha256     - tree manifest"
echo "  rootfs-build-record.json - final build record"
echo "============================================"
