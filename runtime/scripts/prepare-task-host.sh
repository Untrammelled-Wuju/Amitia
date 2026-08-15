#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPONENT_DIR="$RUNTIME_ROOT/task-host"
OUTPUT_DIR="$RUNTIME_ROOT/build/out/task-host/linux-arm64"
CACHE_DIR="$RUNTIME_ROOT/build/staging/task-host-linux-arm64"

if [[ ! -d "$COMPONENT_DIR" ]]; then
    echo "[FATAL] task-host source not found: $COMPONENT_DIR" >&2
    exit 1
fi

echo "============================================"
echo " Task Host Freeze (Linux Shell)"
echo "============================================"

BUILD_ID="$(date +%Y%m%d%H%M%S)-$$"
STAGING_ROOT="$CACHE_DIR/$BUILD_ID"
rm -rf "$STAGING_ROOT"
mkdir -p "$STAGING_ROOT"
trap 'rm -rf "$STAGING_ROOT"' EXIT

cd "$COMPONENT_DIR"

if [[ -f "package-lock.json" ]]; then
    echo "[INSTALL] npm ci --omit=dev"
    npm ci --omit=dev --ignore-scripts 2>&1 | tail -5
else
    echo "[INSTALL] npm install --omit=dev"
    npm install --omit=dev --ignore-scripts 2>&1 | tail -5
fi

echo "[BUILD] tsc"
if [[ -f "tsconfig.json" ]]; then
    npx tsc --project tsconfig.json 2>&1 | tail -10
else
    echo "[FATAL] tsconfig.json not found" >&2
    exit 1
fi

if [[ ! -f "dist/index.js" ]]; then
    echo "[FATAL] dist/index.js not produced" >&2
    exit 1
fi

CANDIDATE="$STAGING_ROOT/candidate"
rm -rf "$CANDIDATE"
mkdir -p "$CANDIDATE/task-host/dist"

cp "$COMPONENT_DIR/dist/index.js" "$CANDIDATE/task-host/dist/index.js"
if [[ -f "$COMPONENT_DIR/package.json" ]]; then
    cp "$COMPONENT_DIR/package.json" "$CANDIDATE/task-host/package.json"
fi

echo "[MANIFEST] Generating file manifest..."
MANIFEST_PATH="$CANDIDATE/task-host-files.sha256"
: > "$MANIFEST_PATH"

(
    cd "$CANDIDATE"
    find "task-host" -type f -print0 | sort -z | while IFS= read -r -d '' entry; do
        rel="${entry#task-host/}"
        sha=$(sha256sum "$entry" | awk '{print $1}')
        echo "$sha  $rel"
    done
) > "$MANIFEST_PATH"

TREE_SHA=$(sha256sum "$MANIFEST_PATH" | awk '{print $1}')
FILE_SHA=$(sha256sum "$CANDIDATE/task-host/dist/index.js" | awk '{print $1}')
FILE_SIZE=$(stat -c '%s' "$CANDIDATE/task-host/dist/index.js" 2>/dev/null || stat -f '%z' "$CANDIDATE/task-host/dist/index.js" 2>/dev/null || wc -c < "$CANDIDATE/task-host/dist/index.js")
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

PUBLISH_DIR="$OUTPUT_DIR"
mkdir -p "$PUBLISH_DIR"

PUBLISH_TMP="$PUBLISH_DIR/.candidate.$$"
rm -rf "$PUBLISH_TMP"
mkdir -p "$PUBLISH_TMP"

cp -a "$CANDIDATE/." "$PUBLISH_TMP/"

if [[ -d "$PUBLISH_DIR/task-host" ]]; then rm -rf "$PUBLISH_DIR/task-host"; fi
[[ -f "$PUBLISH_DIR/task-host-files.sha256" ]] && rm -f "$PUBLISH_DIR/task-host-files.sha256"
[[ -f "$PUBLISH_DIR/task-host-build-record.json" ]] && rm -f "$PUBLISH_DIR/task-host-build-record.json"

mv "$PUBLISH_TMP/task-host" "$PUBLISH_DIR/task-host"
mv "$PUBLISH_TMP/task-host-files.sha256" "$PUBLISH_DIR/task-host-files.sha256"

cat > "$PUBLISH_DIR/task-host-build-record.json" << ENDOFJSON
{
  "schemaVersion": 1,
  "component": "task-host",
  "platform": "linux",
  "architecture": "arm64",
  "source": {
    "type": "typescript",
    "entry": "src/index.ts",
    "output": "dist/index.js"
  },
  "artifact": {
    "file": "task-host/dist/index.js",
    "sha256": "$FILE_SHA",
    "size": $FILE_SIZE
  },
  "treeSha256": "$TREE_SHA",
  "frozenAt": "$TIMESTAMP"
}
ENDOFJSON

rm -rf "$PUBLISH_TMP"

echo "[FREEZE] task-host published to: $PUBLISH_DIR"
echo "============================================"
echo "[DONE] Task Host Freeze Complete"
echo " Output: $PUBLISH_DIR"
echo "  task-host/dist/index.js"
echo "  task-host-files.sha256"
echo "  task-host-build-record.json"
echo "============================================"
