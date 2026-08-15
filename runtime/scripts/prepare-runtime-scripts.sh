#!/bin/bash
# DEPRECATED: Prototype script, not used by the authoritative Python build system.
# The Python builders are at runtime/build/<component>/linux-arm64/build.py
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SOURCE_DIR="$RUNTIME_ROOT/build/node-runtime-scripts/linux-arm64/source"
OUTPUT_DIR="$RUNTIME_ROOT/build/out/runtime-scripts/linux-arm64"
CACHE_DIR="$RUNTIME_ROOT/build/staging/runtime-scripts-linux-arm64"

if [[ ! -d "$SOURCE_DIR" ]]; then
    echo "[FATAL] runtime scripts source not found: $SOURCE_DIR" >&2
    exit 1
fi

echo "============================================"
echo " Runtime Scripts Freeze (Linux Shell)"
echo "============================================"

BUILD_ID="$(date +%Y%m%d%H%M%S)-$$"
STAGING_ROOT="$CACHE_DIR/$BUILD_ID"
rm -rf "$STAGING_ROOT"
mkdir -p "$STAGING_ROOT"
trap 'rm -rf "$STAGING_ROOT"' EXIT

CANDIDATE="$STAGING_ROOT/candidate"
rm -rf "$CANDIDATE"
mkdir -p "$CANDIDATE/scripts/node"
mkdir -p "$CANDIDATE/scripts/node/lib"

for f in "$SOURCE_DIR"/amitia-node-*.sh "$SOURCE_DIR"/amitia-npm-*.sh "$SOURCE_DIR"/amitia-npx-*.sh "$SOURCE_DIR"/amitia-plugin-host.sh "$SOURCE_DIR"/amitia-task-host.sh; do
    [[ -f "$f" ]] || continue
    cp "$f" "$CANDIDATE/scripts/node/$(basename "$f")"
done

for f in "$SOURCE_DIR"/probe-node-runtime.mjs; do
    [[ -f "$f" ]] || continue
    cp "$f" "$CANDIDATE/scripts/node/$(basename "$f")"
done

if [[ -d "$SOURCE_DIR/lib" ]]; then
    for f in "$SOURCE_DIR"/lib/*.sh; do
        [[ -f "$f" ]] || continue
        cp "$f" "$CANDIDATE/scripts/node/lib/$(basename "$f")"
    done
fi

REQUIRED_FILES=("scripts/node/amitia-node-prepare.sh" "scripts/node/amitia-node-probe.sh")
for req in "${REQUIRED_FILES[@]}"; do
    if [[ ! -f "$CANDIDATE/$req" ]]; then
        echo "[FATAL] Required script missing: $req" >&2
        exit 1
    fi
done

echo "[MANIFEST] Generating file manifest..."
MANIFEST_PATH="$CANDIDATE/runtime-scripts-files.sha256"
: > "$MANIFEST_PATH"

(
    cd "$CANDIDATE"
    find "scripts" -type f -print0 | sort -z | while IFS= read -r -d '' entry; do
        rel="${entry#scripts/}"
        full="$CANDIDATE/$entry"
        if [[ -L "$full" ]]; then
            target=$(readlink "$full")
            echo "L $target $rel"
        else
            sha=$(sha256sum "$full" | awk '{print $1}')
            echo "$sha  $rel"
        fi
    done
) > "$MANIFEST_PATH"

TREE_SHA=$(sha256sum "$MANIFEST_PATH" | awk '{print $1}')
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

PUBLISH_DIR="$OUTPUT_DIR"
mkdir -p "$PUBLISH_DIR"

PUBLISH_TMP="$PUBLISH_DIR/.candidate.$$"
rm -rf "$PUBLISH_TMP"
mkdir -p "$PUBLISH_TMP"

cp -a "$CANDIDATE/." "$PUBLISH_TMP/"

if [[ -d "$PUBLISH_DIR/scripts" ]]; then rm -rf "$PUBLISH_DIR/scripts"; fi
[[ -f "$PUBLISH_DIR/runtime-scripts-files.sha256" ]] && rm -f "$PUBLISH_DIR/runtime-scripts-files.sha256"
[[ -f "$PUBLISH_DIR/runtime-scripts-build-record.json" ]] && rm -f "$PUBLISH_DIR/runtime-scripts-build-record.json"

mv "$PUBLISH_TMP/scripts" "$PUBLISH_DIR/scripts"
mv "$PUBLISH_TMP/runtime-scripts-files.sha256" "$PUBLISH_DIR/runtime-scripts-files.sha256"

SCRIPT_COUNT=$(find "$PUBLISH_DIR/scripts" -type f | wc -l | tr -d ' ')

cat > "$PUBLISH_DIR/runtime-scripts-build-record.json" << ENDOFJSON
{
  "schemaVersion": 1,
  "component": "runtime-scripts",
  "platform": "linux",
  "architecture": "arm64",
  "scriptsRoot": "scripts/node",
  "scripts": [
    "amitia-node-prepare.sh",
    "amitia-node-probe.sh",
    "amitia-node-exec.sh",
    "amitia-npm-exec.sh",
    "amitia-npx-exec.sh",
    "amitia-plugin-host.sh",
    "amitia-task-host.sh"
  ],
  "scriptCount": $SCRIPT_COUNT,
  "treeSha256": "$TREE_SHA",
  "frozenAt": "$TIMESTAMP"
}
ENDOFJSON

rm -rf "$PUBLISH_TMP"

echo "[FREEZE] runtime-scripts published to: $PUBLISH_DIR"
echo "============================================"
echo "[DONE] Runtime Scripts Freeze Complete"
echo " Output: $PUBLISH_DIR"
echo "  scripts/node/              - frozen runtime scripts"
echo "  runtime-scripts-files.sha256 - file manifest"
echo "  runtime-scripts-build-record.json - build record"
echo "============================================"
