#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

COMPONENT_NAME="${1:-}"
SOURCE_DIR="${2:-}"

if [[ -z "$COMPONENT_NAME" ]] || [[ -z "$SOURCE_DIR" ]]; then
    echo "Usage: $0 <component-name> <source-dir>"
    echo "Example: $0 sidecar $PWD/backend/sidecar"
    exit 1
fi

COMPONENT_DIR="$SOURCE_DIR"
OUTPUT_DIR="$RUNTIME_ROOT/build/out/$COMPONENT_NAME/linux-arm64"
CACHE_DIR="$RUNTIME_ROOT/build/staging/$COMPONENT_NAME-linux-arm64"

if [[ ! -d "$COMPONENT_DIR" ]]; then
    echo "[FATAL] $COMPONENT_NAME source not found: $COMPONENT_DIR" >&2
    exit 1
fi

echo "============================================"
echo " $COMPONENT_NAME Freeze (Linux Shell)"
echo "============================================"

BUILD_ID="$(date +%Y%m%d%H%M%S)-$$"
STAGING_ROOT="$CACHE_DIR/$BUILD_ID"
rm -rf "$STAGING_ROOT"
mkdir -p "$STAGING_ROOT"
trap 'rm -rf "$STAGING_ROOT"' EXIT

CANDIDATE="$STAGING_ROOT/candidate"
rm -rf "$CANDIDATE"
mkdir -p "$CANDIDATE/$COMPONENT_NAME"

BUNDLE_SRC=""
LAUNCHER_SRC=""

if [[ -f "$COMPONENT_DIR/bundle.mjs" ]]; then
    BUNDLE_SRC="$COMPONENT_DIR/bundle.mjs"
elif [[ -f "$COMPONENT_DIR/src/index.ts" ]]; then
    echo "[BUNDLE] Building bundle.mjs via esbuild..."
    cd "$COMPONENT_DIR"
    if [[ ! -d "node_modules" ]]; then
        npm install 2>&1 | tail -3
    fi
    BUNDLE_SRC="$STAGING_ROOT/bundle.mjs"
    node "$RUNTIME_ROOT/../backend/scripts/bundle-sidecar.cjs" "$COMPONENT_NAME" "$STAGING_ROOT" 2>&1 | tail -5
    [[ -f "$BUNDLE_SRC" ]] || { echo "[FATAL] Failed to build bundle.mjs" >&2; exit 1; }
else
    echo "[FATAL] Neither bundle.mjs nor src/index.ts found in $COMPONENT_DIR" >&2
    exit 1
fi

cp "$BUNDLE_SRC" "$CANDIDATE/$COMPONENT_NAME/bundle.mjs"

if [[ -f "$COMPONENT_DIR/launcher.mjs" ]]; then
    LAUNCHER_SRC="$COMPONENT_DIR/launcher.mjs"
    cp "$LAUNCHER_SRC" "$CANDIDATE/$COMPONENT_NAME/launcher.mjs"
else
    cat > "$CANDIDATE/$COMPONENT_NAME/launcher.mjs" << 'LAUNCHEREOF'
import { createRequire } from "node:module"
const customRequire = createRequire(import.meta.url)
globalThis.require = customRequire
await import("./bundle.mjs")
LAUNCHEREOF
    LAUNCHER_SRC="$CANDIDATE/$COMPONENT_NAME/launcher.mjs"
fi

echo "[MANIFEST] Generating file manifest..."
MANIFEST_PATH="$CANDIDATE/$COMPONENT_NAME-files.sha256"
: > "$MANIFEST_PATH"

(
    cd "$CANDIDATE"
    find "$COMPONENT_NAME" -type f -print0 | sort -z | while IFS= read -r -d '' entry; do
        rel="${entry#$COMPONENT_NAME/}"
        sha=$(sha256sum "$entry" | awk '{print $1}')
        echo "$sha  $rel"
    done
) > "$MANIFEST_PATH"

TREE_SHA=$(sha256sum "$MANIFEST_PATH" | awk '{print $1}')
BUNDLE_SHA=$(sha256sum "$CANDIDATE/$COMPONENT_NAME/bundle.mjs" | awk '{print $1}')
LAUNCHER_SHA=$(sha256sum "$CANDIDATE/$COMPONENT_NAME/launcher.mjs" | awk '{print $1}')
BUNDLE_SIZE=$(stat -c '%s' "$CANDIDATE/$COMPONENT_NAME/bundle.mjs" 2>/dev/null || stat -f '%z' "$CANDIDATE/$COMPONENT_NAME/bundle.mjs" 2>/dev/null || wc -c < "$CANDIDATE/$COMPONENT_NAME/bundle.mjs")
LAUNCHER_SIZE=$(stat -c '%s' "$CANDIDATE/$COMPONENT_NAME/launcher.mjs" 2>/dev/null || stat -f '%z' "$CANDIDATE/$COMPONENT_NAME/launcher.mjs" 2>/dev/null || wc -c < "$CANDIDATE/$COMPONENT_NAME/launcher.mjs")
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

PUBLISH_DIR="$OUTPUT_DIR"
mkdir -p "$PUBLISH_DIR"

PUBLISH_TMP="$PUBLISH_DIR/.candidate.$$"
rm -rf "$PUBLISH_TMP"
mkdir -p "$PUBLISH_TMP"

cp -a "$CANDIDATE/." "$PUBLISH_TMP/"

if [[ -d "$PUBLISH_DIR/$COMPONENT_NAME" ]]; then rm -rf "$PUBLISH_DIR/$COMPONENT_NAME"; fi
[[ -f "$PUBLISH_DIR/$COMPONENT_NAME-files.sha256" ]] && rm -f "$PUBLISH_DIR/$COMPONENT_NAME-files.sha256"
[[ -f "$PUBLISH_DIR/$COMPONENT_NAME-build-record.json" ]] && rm -f "$PUBLISH_DIR/$COMPONENT_NAME-build-record.json"

mv "$PUBLISH_TMP/$COMPONENT_NAME" "$PUBLISH_DIR/$COMPONENT_NAME"
mv "$PUBLISH_TMP/$COMPONENT_NAME-files.sha256" "$PUBLISH_DIR/$COMPONENT_NAME-files.sha256"

cat > "$PUBLISH_DIR/$COMPONENT_NAME-build-record.json" << ENDOFJSON
{
  "schemaVersion": 1,
  "component": "$COMPONENT_NAME",
  "platform": "linux",
  "architecture": "arm64",
  "source": {
    "type": "sidecar-bundle",
    "bundle": "bundle.mjs",
    "launcher": "launcher.mjs"
  },
  "artifact": {
    "bundleFile": "$COMPONENT_NAME/bundle.mjs",
    "bundleSha256": "$BUNDLE_SHA",
    "bundleSize": $BUNDLE_SIZE,
    "launcherFile": "$COMPONENT_NAME/launcher.mjs",
    "launcherSha256": "$LAUNCHER_SHA",
    "launcherSize": $LAUNCHER_SIZE
  },
  "treeSha256": "$TREE_SHA",
  "frozenAt": "$TIMESTAMP"
}
ENDOFJSON

rm -rf "$PUBLISH_TMP"

echo "[FREEZE] $COMPONENT_NAME published to: $PUBLISH_DIR"
echo "============================================"
echo "[DONE] $COMPONENT_NAME Freeze Complete"
echo " Output: $PUBLISH_DIR"
echo "  $COMPONENT_NAME/bundle.mjs"
echo "  $COMPONENT_NAME/launcher.mjs"
echo "  $COMPONENT_NAME-files.sha256"
echo "  $COMPONENT_NAME-build-record.json"
echo "============================================"
