#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
ISH_SRC="$ROOT_DIR/backend/third_party/ish-arm64"
OUTPUT_DIR="$ROOT_DIR/mobile_app/ios/Resources/Rootfs"
VERSION="3.21.0"
ALPINE_SHA256=""
NODE_ARCHIVE=""
NODE_SHA256=""
QDRANT_ARCHIVE=""
QDRANT_SHA256=""
FAKEFSIFY_BIN=""
GO_BIN="${AMITIA_GO_BIN:-go}"
KEEP_WORK=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version) VERSION="$2"; shift 2 ;;
        --sha256) ALPINE_SHA256="$2"; shift 2 ;;
        --node-archive) NODE_ARCHIVE="$2"; shift 2 ;;
        --node-sha256) NODE_SHA256="$2"; shift 2 ;;
        --qdrant-archive) QDRANT_ARCHIVE="$2"; shift 2 ;;
        --qdrant-sha256) QDRANT_SHA256="$2"; shift 2 ;;
        --fakefsify) FAKEFSIFY_BIN="$2"; shift 2 ;;
        --go) GO_BIN="$2"; shift 2 ;;
        --output) OUTPUT_DIR="$2"; shift 2 ;;
        --keep-work) KEEP_WORK=1; shift ;;
        *) echo "[prepare_rootfs] Unknown arg: $1" >&2; exit 1 ;;
    esac
done

if [ "$(uname)" != "Darwin" ]; then
    echo "[prepare_rootfs] ERROR: Requires macOS" >&2
    exit 1
fi
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || [[ ! "$ALPINE_SHA256" =~ ^[a-fA-F0-9]{64}$ ]] || [[ ! "$NODE_SHA256" =~ ^[a-fA-F0-9]{64}$ ]] || [[ ! "$QDRANT_SHA256" =~ ^[a-fA-F0-9]{64}$ ]] || [ ! -f "$NODE_ARCHIVE" ] || [ ! -f "$QDRANT_ARCHIVE" ]; then
    echo "[prepare_rootfs] ERROR: valid Alpine, ARM64 musl Node and ARM64 static/musl Qdrant archive/hash required" >&2
    exit 1
fi
for tool in python3 curl shasum; do
    command -v "$tool" >/dev/null || { echo "[prepare_rootfs] Missing $tool" >&2; exit 1; }
done
command -v "$GO_BIN" >/dev/null || { echo "[prepare_rootfs] Go source compiler missing" >&2; exit 1; }
echo "$NODE_SHA256  $NODE_ARCHIVE" | shasum -a 256 -c -
echo "$QDRANT_SHA256  $QDRANT_ARCHIVE" | shasum -a 256 -c -
mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR="$(cd "$OUTPUT_DIR" && pwd)"
NODE_ARCHIVE="$(cd "$(dirname "$NODE_ARCHIVE")" && pwd)/$(basename "$NODE_ARCHIVE")"
QDRANT_ARCHIVE="$(cd "$(dirname "$QDRANT_ARCHIVE")" && pwd)/$(basename "$QDRANT_ARCHIVE")"
ROOTFS_ZIP="$OUTPUT_DIR/alpine-rootfs.zip"
RELEASE_MANIFEST="$OUTPUT_DIR/rootfs-release.json"
WORK="$(mktemp -d)"
trap 'if [ "$KEEP_WORK" = "0" ]; then rm -rf "$WORK"; fi' EXIT
RELEASE="${VERSION%.*}"
MINIROOTFS_FILE="$WORK/alpine-minirootfs.tar.gz"
curl -fsSL --retry 3 -o "$MINIROOTFS_FILE" "https://dl-cdn.alpinelinux.org/alpine/v${RELEASE}/releases/aarch64/alpine-minirootfs-${VERSION}-aarch64.tar.gz"
echo "$ALPINE_SHA256  $MINIROOTFS_FILE" | shasum -a 256 -c -

if [ -z "$FAKEFSIFY_BIN" ]; then
    FAKEFSIFY_BIN="$ISH_SRC/build-native/tools/fakefsify"
fi
if [ ! -x "$FAKEFSIFY_BIN" ]; then
    command -v meson >/dev/null && command -v ninja >/dev/null || { echo "[prepare_rootfs] Host meson/ninja missing" >&2; exit 1; }
    meson setup "$ISH_SRC/build-native" "$ISH_SRC" --wrap-mode=nodownload --buildtype=release -Dkernel=ish -Dengine=asbestos -Dguest_arch=arm64
    ninja -C "$ISH_SRC/build-native" tools/fakefsify
fi
if [ ! -x "$FAKEFSIFY_BIN" ]; then
    echo "[prepare_rootfs] ERROR: host fakefsify requires host libarchive/sqlite development dependencies" >&2
    exit 1
fi

cd "$ROOT_DIR/backend"
export GOOS=linux GOARCH=arm64 CGO_ENABLED=0 GOWORK=off
"$GO_BIN" list -deps -json ./cmd/server > "$WORK/packages.json"
python3 "$SCRIPT_DIR/rootfs_inputs.py" sources "$WORK/packages.json" "$ROOT_DIR/backend" "$WORK/core-source-inputs.json"
"$GO_BIN" build -trimpath -buildvcs=false -o "$WORK/AmitiaCore" ./cmd/server
"$GO_BIN" list -deps -json ./cmd/server > "$WORK/packages-after.json"
python3 "$SCRIPT_DIR/rootfs_inputs.py" sources "$WORK/packages-after.json" "$ROOT_DIR/backend" "$WORK/core-source-inputs-after.json"
cmp "$WORK/core-source-inputs.json" "$WORK/core-source-inputs-after.json" || { echo "[prepare_rootfs] Source changed during build" >&2; exit 1; }
python3 "$SCRIPT_DIR/rootfs_inputs.py" prepare "$MINIROOTFS_FILE" "$NODE_ARCHIVE" "$WORK/AmitiaCore" "$WORK/core-source-inputs.json" "$VERSION" "$WORK/guest-input.tar.gz" "$WORK/rootfs.manifest.json" "$QDRANT_ARCHIVE"
"$FAKEFSIFY_BIN" "$WORK/guest-input.tar.gz" "$WORK/fakefs-out"
python3 "$SCRIPT_DIR/rootfs_inputs.py" package "$WORK/fakefs-out" "$WORK/rootfs.manifest.json" "$WORK/core-source-inputs.json" "$ROOTFS_ZIP" "$RELEASE_MANIFEST"
echo "[prepare_rootfs] Complete: $ROOTFS_ZIP"
echo "[prepare_rootfs] Verified release manifest: $RELEASE_MANIFEST"

