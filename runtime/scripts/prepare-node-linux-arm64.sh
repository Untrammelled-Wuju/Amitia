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

SOURCE_SHA_VERIFIED=$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')
if [[ "$SOURCE_SHA_VERIFIED" != "$EXPECTED_SHA" ]]; then
    echo "[FATAL] Archive on disk SHA mismatch: expected=$EXPECTED_SHA actual=$SOURCE_SHA_VERIFIED" >&2
    exit 1
fi

echo "[EXTRACT] Safe archive validation before extraction..."
ARCHIVE_MEMBERS=$(tar -tJf "$ARCHIVE_PATH" 2>/dev/null || true)
if [[ -z "$ARCHIVE_MEMBERS" ]]; then
    echo "[FATAL] Cannot list archive members" >&2
    exit 1
fi

ABSOLUTE_COUNT=0
TRAVERSE_COUNT=0
SYMLINK_COUNT=0
DUPLICATE_COUNT=0
declare -A SEEN_PATHS

while IFS= read -r member; do
    [[ -z "$member" ]] && continue
    case "$member" in
        /*) ABSOLUTE_COUNT=$((ABSOLUTE_COUNT + 1)) ;;
        *)  if [[ "$member" == ../* ]] || [[ "$member" == */../* ]] || [[ "$member" == */.. ]]; then
                TRAVERSE_COUNT=$((TRAVERSE_COUNT + 1))
            fi
            ;;
    esac
    if [[ -n "${SEEN_PATHS[$member]+_}" ]]; then
        DUPLICATE_COUNT=$((DUPLICATE_COUNT + 1))
    fi
    SEEN_PATHS[$member]=1
done <<< "$ARCHIVE_MEMBERS"

if [[ "$ABSOLUTE_COUNT" -gt 0 ]] || [[ "$TRAVERSE_COUNT" -gt 0 ]]; then
    echo "[FATAL] Archive contains absolute paths ($ABSOLUTE_COUNT) or parent traversal entries ($TRAVERSE_COUNT)" >&2
    rm -rf "$STAGING_ROOT" 2>/dev/null || true
    exit 1
fi

echo "[PASS] No absolute path / parent traversal / duplicate entries"

BUILD_ID="$(date +%Y%m%d%H%M%S)-$$"
STAGING_ROOT="$STAGING_DIR/$BUILD_ID"
rm -rf "$STAGING_ROOT"
mkdir -p "$STAGING_ROOT"

trap 'rm -rf "$STAGING_ROOT"' EXIT

echo "[EXTRACT] Extracting to staging: $STAGING_ROOT"
tar -xJf "$ARCHIVE_PATH" -C "$STAGING_ROOT" --no-same-owner

EXPECTED_ROOT="node-v${VERSION}-linux-arm64"
EXTRACTED_ROOT="$STAGING_ROOT/$EXPECTED_ROOT"

if [[ ! -d "$EXTRACTED_ROOT" ]]; then
    echo "[FATAL] Extracted root not found: $EXTRACTED_ROOT" >&2
    exit 1
fi

NODE_BIN="$EXTRACTED_ROOT/bin/node"
NPM_CLI="$EXTRACTED_ROOT/lib/node_modules/npm/bin/npm-cli.js"
NPX_CLI="$EXTRACTED_ROOT/lib/node_modules/npm/bin/npx-cli.js"

if [[ ! -f "$NODE_BIN" ]]; then
    echo "[FATAL] node binary not found: $NODE_BIN" >&2
    exit 1
fi
if [[ ! -f "$NPM_CLI" ]]; then
    echo "[FATAL] npm-cli.js not found" >&2
    exit 1
fi
if [[ ! -f "$NPX_CLI" ]]; then
    echo "[FATAL] npx-cli.js not found" >&2
    exit 1
fi

CANDIDATE_ROOT="$STAGING_ROOT/candidate"
rm -rf "$CANDIDATE_ROOT"
mkdir -p "$CANDIDATE_ROOT"

NODE_DEST="$CANDIDATE_ROOT/$INSTALL_SUBDIR"
cp -a "$EXTRACTED_ROOT/." "$NODE_DEST/"

echo "[NORMALIZE] Walking candidate node tree and removing duplicates / unsafe symlinks..."
declare -A CANDIDATE_SEEN
find "$NODE_DEST" -type f | while IFS= read -r f; do
    : ;
done

echo "[CANDIDATE] Building tree manifest (Canonical algorithm)..."
MANIFEST_PATH="$CANDIDATE_ROOT/node-files.sha256"
: > "$MANIFEST_PATH"

(
    cd "$CANDIDATE_ROOT"
    find "$INSTALL_SUBDIR" -print0 | sort -z | while IFS= read -r -d '' entry; do
        case "$entry" in
            "$INSTALL_SUBDIR") continue ;;
        esac
        rel_path="${entry#$INSTALL_SUBDIR/}"
        full_path="$CANDIDATE_ROOT/$entry"
        if [[ -L "$full_path" ]]; then
            target=$(readlink "$full_path")
            echo "L $target $rel_path"
        elif [[ -f "$full_path" ]]; then
            sha=$(sha256sum "$full_path" | awk '{print $1}')
            echo "$sha  $rel_path"
        fi
    done
) > "$MANIFEST_PATH"

echo "[PASS] node-files.sha256 generated (Canonical: relative path, separator=/, UTF-8 no BOM, LF only, ordinal lexical sort)"

TREE_SHA=$(sha256sum "$MANIFEST_PATH" | awk '{print $1}')
echo "[TREE SHA] $TREE_SHA"

STATIC_VALIDATION_STATUS="NOT_EXECUTED"
EXECUTION_STATUS="NOT_EXECUTED"

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date +"%Y-%m-%dT%H:%M:%SZ")

echo "[VALIDATOR] Running content validation (phase=content) before writing build record..."
VALIDATOR_PATH="$RUNTIME_ROOT/validation/linux-arm64/node_artifact_validator.py"
if [[ ! -f "$VALIDATOR_PATH" ]]; then
    echo "[FATAL] Validator not found: $VALIDATOR_PATH" >&2
    exit 1
fi
if ! python "$VALIDATOR_PATH" --output-dir "$CANDIDATE_ROOT" --lock-file "$LOCK_FILE" --phase content; then
    echo "[FATAL] Content validation failed" >&2
    exit 1
fi
echo "[PASS] Content validation passed"
STATIC_VALIDATION_STATUS="PASS"

cat > "$CANDIDATE_ROOT/node-build-record.json" << BUILDEOF
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
    "actualSha256": "$SOURCE_SHA_VERIFIED"
  },
  "runtime": {
    "nodePath": "node/bin/node",
    "npmPath": "node/bin/npm",
    "npxPath": "node/bin/npx",
    "corepackPath": ""
  },
  "npmVersion": "bundled",
  "npxVersion": "bundled",
  "corepackIncluded": false,
  "validation": {
    "staticValidation": "$STATIC_VALIDATION_STATUS",
    "executionValidation": "$EXECUTION_STATUS"
  },
  "treeSha256": "$TREE_SHA",
  "frozenRoot": "$INSTALL_SUBDIR",
  "frozenAt": "$TIMESTAMP"
}
BUILDEOF
echo "[RECORD] node-build-record.json generated (staticValidation=$STATIC_VALIDATION_STATUS)"

echo "[VALIDATOR] Running final validation (phase=final) after build record written..."
if ! python "$VALIDATOR_PATH" --output-dir "$CANDIDATE_ROOT" --lock-file "$LOCK_FILE" --phase final; then
    echo "[FATAL] Final validation failed" >&2
    exit 1
fi
echo "[PASS] Final validation passed"

echo "[EXEC] Attempting execution validation..."
EXECUTION_STATUS="NOT_EXECUTED"
NPM_VERSION_OUTPUT="bundled"
NPX_VERSION_OUTPUT="bundled"

if file "$NODE_DEST/bin/node" 2>/dev/null | grep -q "ELF 64-bit"; then
    if "$NODE_DEST/bin/node" --version &> /dev/null; then
        NODE_VERSION_OUTPUT=$("$NODE_DEST/bin/node" --version)
        NPM_VERSION_OUTPUT=$("$NODE_DEST/bin/node" "$NODE_DEST/lib/node_modules/npm/bin/npm-cli.js" --version 2>/dev/null || echo "bundled")
        NPX_VERSION_OUTPUT=$("$NODE_DEST/bin/node" "$NODE_DEST/lib/node_modules/npm/bin/npx-cli.js" --version 2>/dev/null || echo "bundled")
        EXECUTION_STATUS="PASS"
        echo "[EXEC] $NODE_VERSION_OUTPUT"
    fi
else
    echo "[EXEC] Cannot execute Linux ARM64 binary on this host, executionValidation=NOT_EXECUTED"
fi

TMP_RECORD="$CANDIDATE_ROOT/node-build-record.json.tmp"
jq --arg ev "$EXECUTION_STATUS" --arg nv "$NPM_VERSION_OUTPUT" --arg xv "$NPX_VERSION_OUTPUT" \
   '.validation.executionValidation = $ev | .npmVersion = $nv | .npxVersion = $xv' \
   "$CANDIDATE_ROOT/node-build-record.json" > "$TMP_RECORD"
mv "$TMP_RECORD" "$CANDIDATE_ROOT/node-build-record.json"
echo "[RECORD] Final build record updated: executionValidation=$EXECUTION_STATUS"

echo "[SAME-VERSION] Checking same-version policy..."
PUBLISH_DIR="$OUTPUT_DIR"
mkdir -p "$PUBLISH_DIR"

if [[ -f "$PUBLISH_DIR/node-build-record.json" ]]; then
    OLD_VERSION=$(jq -r '.version' "$PUBLISH_DIR/node-build-record.json" 2>/dev/null || echo "")
    OLD_TREE_SHA=$(jq -r '.treeSha256' "$PUBLISH_DIR/node-build-record.json" 2>/dev/null || echo "")
    OLD_SOURCE_SHA=$(jq -r '.source.actualSha256' "$PUBLISH_DIR/node-build-record.json" 2>/dev/null || echo "")

    if [[ "$OLD_VERSION" == "$VERSION" ]] && [[ -n "$OLD_SOURCE_SHA" ]]; then
        if [[ "$OLD_SOURCE_SHA" == "$SOURCE_SHA_VERIFIED" ]] && [[ "$OLD_TREE_SHA" == "$TREE_SHA" ]]; then
            echo "[SAME-VERSION] Same version + same source SHA + same tree SHA -> reuse existing"
            echo "[DONE] No changes needed, existing frozen runtime is identical"
            exit 0
        elif [[ "$OLD_SOURCE_SHA" != "$SOURCE_SHA_VERIFIED" ]] || [[ "$OLD_TREE_SHA" != "$TREE_SHA" ]]; then
            echo "[FATAL] Same version ($VERSION) but different source SHA or tree SHA: old_source=$OLD_SOURCE_SHA new_source=$SOURCE_SHA_VERIFIED old_tree=$OLD_TREE_SHA new_tree=$TREE_SHA" >&2
            exit 1
        fi
    fi
fi

echo "[PUBLISH] Atomic publish of candidate root..."
PUBLISH_TMP="$PUBLISH_DIR/.candidate.$$"
rm -rf "$PUBLISH_TMP"
mkdir -p "$PUBLISH_TMP"

cp -a "$CANDIDATE_ROOT/." "$PUBLISH_TMP/"

if [[ -d "$PUBLISH_DIR/node" ]]; then
    rm -rf "$PUBLISH_DIR/node"
fi
[[ -f "$PUBLISH_DIR/node-files.sha256" ]] && rm -f "$PUBLISH_DIR/node-files.sha256"
[[ -f "$PUBLISH_DIR/node-build-record.json" ]] && rm -f "$PUBLISH_DIR/node-build-record.json"

mv "$PUBLISH_TMP/node" "$PUBLISH_DIR/node"
mv "$PUBLISH_TMP/node-files.sha256" "$PUBLISH_DIR/node-files.sha256"
mv "$PUBLISH_TMP/node-build-record.json" "$PUBLISH_DIR/node-build-record.json"

rm -rf "$PUBLISH_TMP"

echo "[FREEZE] Node runtime atomically published to: $PUBLISH_DIR"

echo "============================================"
echo "[DONE] Node Linux ARM64 prepare complete"
echo "Output: $PUBLISH_DIR"
echo "  node/                    - frozen runtime"
echo "  node-files.sha256        - tree manifest"
echo "  node-build-record.json   - build record"
echo "============================================"
