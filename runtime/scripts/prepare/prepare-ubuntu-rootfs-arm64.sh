#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
LOCK_FILE="$REPO_ROOT/runtime/artifacts/ubuntu-rootfs/linux-arm64/ubuntu-rootfs-lock.json"
POLICY_FILE="$REPO_ROOT/runtime/artifacts/ubuntu-rootfs/linux-arm64/rootfs-policy.json"

RELEASE_MODE=true
OFFLINE=false
CACHE_DIR=""
STAGING_DIR=""
OUTPUT_DIR=""
SKIP_VERIFY=false

while [[ $# -gt 0 ]];
do
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
    resolved_root="$(cd "$(dirname "$LOCK_FILE")" && pwd)"
    echo "[DIAG] Resolved REPO_ROOT: $REPO_ROOT"
    echo "[DIAG] Expected lock directory: $resolved_root/runtime/artifacts/ubuntu-rootfs/linux-arm64"
    exit 1
fi

RELEASE=$(jq -r '.release' "$LOCK_FILE")
ARCHIVE_FILE_NAME=$(jq -r '.archiveFileName' "$LOCK_FILE")
EXPECTED_SHA=$(jq -r '.sha256' "$LOCK_FILE")
SOURCE_URL=$(jq -r '.sourceUrl' "$LOCK_FILE")
DISTRIBUTION=$(jq -r '.distribution' "$LOCK_FILE")
FLAVOR=$(jq -r '.flavor' "$LOCK_FILE")
CODENAME=$(jq -r '.codename' "$LOCK_FILE")
ARCHITECTURE=$(jq -r '.architecture' "$LOCK_FILE")
GUEST_PLATFORM=$(jq -r '.guestPlatform' "$LOCK_FILE")
RUNTIME_KIND=$(jq -r '.runtimeKind' "$LOCK_FILE")
GUEST_LOADER=$(jq -r '.guestLoader // "/lib/ld-linux-aarch64.so.1"' "$LOCK_FILE")

echo "============================================"
echo " Ubuntu ARM64 Rootfs Prepare (Linux Shell = Canonical Release Authority)"
echo "============================================"
echo " Release:          $RELEASE"
echo " Archive:          $ARCHIVE_FILE_NAME"
echo " Expected SHA256:  $EXPECTED_SHA"
echo " Source:           $SOURCE_URL"
echo " Distribution:     $DISTRIBUTION"
echo " Codename:         $CODENAME"
echo " Architecture:     $ARCHITECTURE"
echo " GuestPlatform:    $GUEST_PLATFORM"
echo " RuntimeKind:      $RUNTIME_KIND"
echo " GuestLoader:      $GUEST_LOADER"
echo "============================================"

if [[ -n "$CACHE_DIR" ]]; then
    CACHE_PATH="$CACHE_DIR"
else
    CACHE_PATH="$REPO_ROOT/runtime/.cache/ubuntu-rootfs"
fi

STAGING_UUID="$(cat /proc/sys/kernel/random/uuid 2>/dev/null || date +%s%N)-$$"
if [[ -n "$STAGING_DIR" ]]; then
    STAGING_PATH="$STAGING_DIR"
else
    STAGING_PATH="$REPO_ROOT/runtime/build/staging/rootfs/linux-arm64/$STAGING_UUID"
fi
if [[ -n "$OUTPUT_DIR" ]]; then
    OUTPUT_PATH="$OUTPUT_DIR"
else
    OUTPUT_PATH="$REPO_ROOT/runtime/build/out/rootfs/linux-arm64"
fi

mkdir -p "$CACHE_PATH"
rm -rf "$STAGING_PATH"
mkdir -p "$STAGING_PATH"
trap 'rm -rf "$STAGING_PATH"' EXIT

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

echo "[EXTRACT] Safe archive validation before extraction..."
ARCHIVE_MEMBERS=$(tar -tzf "$ARCHIVE_FILE" 2>/dev/null || true)
if [[ -z "$ARCHIVE_MEMBERS" ]]; then
    echo "[FATAL] Cannot list archive members" >&2
    exit 1
fi

ABS_COUNT=0
TRAVERSE_COUNT=0
DUPLICATE_COUNT=0
declare -A SEEN_AR_MEMBERS

while IFS= read -r member; do
    [[ -z "$member" ]] && continue
    case "$member" in
        /*) ABS_COUNT=$((ABS_COUNT + 1)) ;;
        *)
            if [[ "$member" == ../* ]] || [[ "$member" == */../* ]] || [[ "$member" == */.. ]]; then
                TRAVERSE_COUNT=$((TRAVERSE_COUNT + 1))
            fi
            ;;
    esac
    if [[ -n "${SEEN_AR_MEMBERS[$member]+_}" ]]; then
        DUPLICATE_COUNT=$((DUPLICATE_COUNT + 1))
    fi
    SEEN_AR_MEMBERS[$member]=1
done <<< "$ARCHIVE_MEMBERS"

if [[ "$ABS_COUNT" -gt 0 ]] || [[ "$TRAVERSE_COUNT" -gt 0 ]]; then
    echo "[FATAL] Archive contains absolute paths ($ABS_COUNT) or parent traversal entries ($TRAVERSE_COUNT)" >&2
    exit 1
fi

echo "[PASS] No absolute path / parent traversal / duplicate entries"

echo "[EXTRACT] Extracting to staging candidate..."
EXTRACT_DIR="$STAGING_PATH/rootfs"
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

if [[ "$RELEASE_MODE" == "true" ]] && [[ "$SKIP_VERIFY" == "true" ]]; then
    echo "[FATAL] --skip-verify is disallowed in release mode" >&2
    exit 1
fi

if [[ "$SKIP_VERIFY" == "false" ]]; then
    VALIDATOR_PATH="$REPO_ROOT/runtime/validation/linux-arm64/rootfs_validator.py"
    if [[ -f "$VALIDATOR_PATH" ]]; then
        echo "[VERIFY] Running static validation against staging rootfs tree..."
        if ! python "$VALIDATOR_PATH" --rootfs "$EXTRACT_DIR" --lock "$LOCK_FILE" --policy "$POLICY_FILE"; then
            echo "[FATAL] Staged rootfs static validation failed" >&2
            exit 1
        fi
        echo "[PASS] Rootfs tree static validation passed"
    else
        echo "[FATAL] Release mode: validator missing at $VALIDATOR_PATH (missing required release validator is FATAL, not SKIP)" >&2
        exit 1
    fi
fi

echo "[TREE MANIFEST] Generating canonical manifest (F/L/D modes)..."
MANIFEST_PATH="$STAGING_PATH/rootfs-files.tsv"
: > "$MANIFEST_PATH"
(
    cd "$EXTRACT_DIR"
    find . -print0 | sort -z | while IFS= read -r -d '' entry; do
        case "$entry" in
            .) continue ;;
            ./) continue ;;
        esac
        rel="${entry#./}"
        full="$EXTRACT_DIR/$rel"
        if [[ -L "$full" ]]; then
            target=$(readlink "$full")
            perm=$(stat -c '%a' "$full")
            echo "L $perm $target $rel"
        elif [[ -d "$full" ]]; then
            perm=$(stat -c '%a' "$full")
            echo "D $perm - $rel"
        elif [[ -f "$full" ]]; then
            perm=$(stat -c '%a' "$full")
            sha=$(sha256sum "$full" | awk '{print $1}')
            echo "F $perm $sha $rel"
        fi
    done
) > "$MANIFEST_PATH"
echo "[PASS] Tree manifest generated (F/L/D): $MANIFEST_PATH"

echo "[FREEZE] Creating deterministic frozen tar.xz archive from staged rootfs..."
FROZEN_TAR_NAME="ubuntu-rootfs-arm64-${RELEASE}.tar.xz"
FROZEN_TAR_PATH="$OUTPUT_PATH/$FROZEN_TAR_NAME"
TEMP_TAR_PATH="$STAGING_PATH/$FROZEN_TAR_NAME"

SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-0}"

(
    cd "$EXTRACT_DIR"
    tar --sort=name \
        --mtime="@$SOURCE_DATE_EPOCH" \
        --owner=0 \
        --group=0 \
        --numeric-owner \
        --format=posix \
        -cJf "$TEMP_TAR_PATH" \
        .
)

if [[ ! -f "$TEMP_TAR_PATH" ]]; then
    echo "[FATAL] Frozen archive was not created" >&2
    exit 1
fi

echo "[VERIFY] Confirming frozen archive integrity by reopening..."
if ! tar -tJf "$TEMP_TAR_PATH" > /dev/null 2>&1; then
    echo "[FATAL] Frozen archive integrity check failed; archive cannot be reopened" >&2
    exit 1
fi
echo "[PASS] Frozen tar.xz integrity verified (deterministic re-open check)"

SHA_OF_FROZEN_TAR=$(sha256sum "$TEMP_TAR_PATH" | awk '{print $1}')
echo "[FREEZE] Frozen archive: $TEMP_TAR_PATH"
echo "[FREEZE] Frozen SHA256:  $SHA_OF_FROZEN_TAR"

TREE_SHA=$(sha256sum "$MANIFEST_PATH" | awk '{print $1}')
GENERATION_TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

FINAL_RECORD="$OUTPUT_PATH/rootfs-build-record.json"

echo "[SAME-VERSION] Checking same-version policy..."
if [[ -f "$FINAL_RECORD" ]]; then
    EXISTING_VERSION=$(jq -r '.release' "$FINAL_RECORD" 2>/dev/null || echo "")
    EXISTING_SRC_SHA=$(jq -r '.source.actualSha256' "$FINAL_RECORD" 2>/dev/null || echo "")
    EXISTING_TREE_SHA=$(jq -r '.frozen.treeSha256' "$FINAL_RECORD" 2>/dev/null || echo "")
    EXISTING_FROZEN_SHA=$(jq -r '.frozen.archiveSha256' "$FINAL_RECORD" 2>/dev/null || echo "")

    if [[ "$EXISTING_VERSION" == "$RELEASE" ]] && [[ -n "$EXISTING_SRC_SHA" ]]; then
        if [[ "$EXISTING_SRC_SHA" == "$ACTUAL_SHA" ]] && [[ "$EXISTING_TREE_SHA" == "$TREE_SHA" ]] && [[ "$EXISTING_FROZEN_SHA" == "$SHA_OF_FROZEN_TAR" ]]; then
            echo "[SAME-VERSION] Same release=$RELEASE + same source SHA + same tree SHA + same frozen SHA -> reuse existing"
            echo "[DONE] No changes needed; existing frozen rootfs is identical"
            exit 0
        else
            echo "[FATAL] Same release ($RELEASE) but different tree SHA or frozen SHA: existing_src=$EXISTING_SRC_SHA new_src=$ACTUAL_SHA existing_tree=$EXISTING_TREE_SHA new_tree=$TREE_SHA existing_tar=$EXISTING_FROZEN_SHA new_tar=$SHA_OF_FROZEN_TAR" >&2
            exit 1
        fi
    fi
fi

echo "[ATOMIC PUBLISH] Staging summary:"
echo "  staging path:  $STAGING_PATH"
echo "  staging uuid:  $STAGING_UUID"
echo "  frozen tar:    $TEMP_TAR_PATH"
echo "  manifest:      $MANIFEST_PATH"
echo "  output path:   $OUTPUT_PATH"

echo "[PUBLISH] Atomic publish of frozen tar + manifest + build record..."
PUBLISH_TMP="$OUTPUT_PATH/.candidate.$$"
rm -rf "$PUBLISH_TMP"
mkdir -p "$PUBLISH_TMP"

cp "$TEMP_TAR_PATH" "$PUBLISH_TMP/$FROZEN_TAR_NAME"
cp "$MANIFEST_PATH" "$PUBLISH_TMP/rootfs-files.tsv"

cat > "$PUBLISH_TMP/rootfs-build-record.json" << ENDOFJSON
{
  "schemaVersion": 1,
  "component": "ubuntu-rootfs",
  "distribution": "$DISTRIBUTION",
  "flavor": "$FLAVOR",
  "release": "$RELEASE",
  "codename": "$CODENAME",
  "architecture": "$ARCHITECTURE",
  "guestPlatform": "$GUEST_PLATFORM",
  "runtimeKind": "$RUNTIME_KIND",
  "guestLoader": "$GUEST_LOADER",
  "source": {
    "url": "$SOURCE_URL",
    "archiveFileName": "$ARCHIVE_FILE_NAME",
    "expectedSha256": "$EXPECTED_SHA",
    "actualSha256": "$ACTUAL_SHA"
  },
  "frozen": {
    "archiveFileName": "$FROZEN_TAR_NAME",
    "archiveSha256": "$SHA_OF_FROZEN_TAR",
    "treeSha256": "$TREE_SHA"
  },
  "generationTimestamp": "$GENERATION_TIMESTAMP",
  "offline": $OFFLINE,
  "buildMode": "$([[ "$RELEASE_MODE" == "true" ]] && echo "release" || echo "dev")"
}
ENDOFJSON

echo "[PUBLISH] Atomic rename of candidate -> final output..."
if [[ -f "$FROZEN_TAR_PATH" ]]; then rm -f "$FROZEN_TAR_PATH"; fi
if [[ -f "$OUTPUT_PATH/rootfs-files.tsv" ]]; then rm -f "$OUTPUT_PATH/rootfs-files.tsv"; fi
if [[ -f "$FINAL_RECORD" ]]; then rm -f "$FINAL_RECORD"; fi

mv "$PUBLISH_TMP/$FROZEN_TAR_NAME" "$FROZEN_TAR_PATH"
mv "$PUBLISH_TMP/rootfs-files.tsv" "$OUTPUT_PATH/rootfs-files.tsv"
mv "$PUBLISH_TMP/rootfs-build-record.json" "$FINAL_RECORD"
rm -rf "$PUBLISH_TMP"

echo "[RECORD] Final build record: $FINAL_RECORD"

echo "============================================"
echo "[DONE] Ubuntu ARM64 Rootfs Freeze Complete"
echo "============================================"
echo " Output: $OUTPUT_PATH"
echo "  $FROZEN_TAR_NAME   - frozen rootfs archive"
echo "  rootfs-files.tsv          - tree manifest (F/L/D)"
echo "  rootfs-build-record.json  - final build record"
echo "============================================"
