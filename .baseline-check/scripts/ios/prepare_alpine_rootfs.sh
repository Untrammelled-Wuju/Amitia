#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
ISH_SRC="$ROOT_DIR/backend/third_party/ish"
DEPS_DIR="$ROOT_DIR/mobile_app/ios/ThirdParty/iSH"
OUTPUT_DIR="$ROOT_DIR/mobile_app/ios/Resources/Rootfs"
ROOTFS_ZIP="$OUTPUT_DIR/alpine-rootfs.zip"
RELEASE_MANIFEST="$OUTPUT_DIR/rootfs-release.json"

VERSION="3.21.0"
ALPINE_SHA256=""
FAKEFSIFY_BIN=""
KEEP_WORK=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)
            VERSION="$2"; shift 2 ;;
        --sha256)
            ALPINE_SHA256="$2"; shift 2 ;;
        --fakefsify)
            FAKEFSIFY_BIN="$2"; shift 2 ;;
        --output)
            OUTPUT_DIR="$2"; shift 2 ;;
        --keep-work)
            KEEP_WORK=1; shift ;;
        *)
            echo "[prepare_rootfs] Unknown arg: $1" >&2; exit 1 ;;
    esac
done

if [ "$(uname)" != "Darwin" ]; then
    echo "[prepare_rootfs] ERROR: Requires macOS" >&2
    exit 1
fi

if [ -z "$ALPINE_SHA256" ]; then
    echo "[prepare_rootfs] ERROR: --sha256 required" >&2
    exit 1
fi

mkdir -p "$OUTPUT_DIR"
WORK="$(mktemp -d)"
trap 'if [ "$KEEP_WORK" = "0" ] && [ -n "$WORK" ] && [ -d "$WORK" ]; then rm -rf "$WORK"; fi' EXIT

cd "$WORK"

echo "[prepare_rootfs] Downloading Alpine $VERSION aarch64 minirootfs..."
MINIROOTFS_URL="https://dl-cdn.alpinelinux.org/alpine/v${VERSION}/releases/aarch64/alpine-minirootfs-${VERSION}-aarch64.tar.gz"
MINIROOTFS_FILE="$WORK/alpine-minirootfs-${VERSION}-aarch64.tar.gz"

curl -fsSL --retry 3 -o "$MINIROOTFS_FILE" "$MINIROOTFS_URL"

echo "$ALPINE_SHA256  $MINIROOTFS_FILE" | shasum -a 256 -c -

TINYROOTFS_DIR="$WORK/tinyrootfs"
mkdir -p "$TINYROOTFS_DIR"
tar -xzf "$MINIROOTFS_FILE" -C "$TINYROOTFS_DIR"

if [ -z "$FAKEFSIFY_BIN" ]; then
    if [ -x "$DEPS_DIR/../tools/fakefsify" ]; then
        FAKEFSIFY_BIN="$DEPS_DIR/../tools/fakefsify"
    elif [ -x "$ISH_SRC/build-native/tools/fakefsify" ]; then
        FAKEFSIFY_BIN="$ISH_SRC/build-native/tools/fakefsify"
    fi
fi

if [ -z "$FAKEFSIFY_BIN" ] || [ ! -x "$FAKEFSIFY_BIN" ]; then
    echo "[prepare_rootfs] Building fakefsify from iSH source..."
    cd "$ISH_SRC"
    if [ ! -d "build-native" ]; then
        meson setup build-native --cross-file ios-cross.txt --buildtype=release 2>/dev/null || meson setup build-native --buildtype=release
    fi
    ninja -C build-native tools/fakefsify 2>/dev/null || ninja -C build-native fakefsify
    if [ -x "$ISH_SRC/build-native/tools/fakefsify" ]; then
        FAKEFSIFY_BIN="$ISH_SRC/build-native/tools/fakefsify"
    else
        FAKEFSIFY_BIN="$(find "$ISH_SRC/build-native" -name fakefsify -type f -executable | head -1)"
    fi
    cd "$WORK"
fi

if [ -z "$FAKEFSIFY_BIN" ] || [ ! -x "$FAKEFSIFY_BIN" ]; then
    echo "[prepare_rootfs] ERROR: fakefsify binary not found" >&2
    exit 1
fi

FAKEFS_DIR="$WORK/fakefs-out"
mkdir -p "$FAKEFS_DIR"
"$FAKEFSIFY_BIN" "$TINYROOTFS_DIR" "$FAKEFS_DIR"

if [ ! -d "$FAKEFS_DIR/data" ]; then
    echo "[prepare_rootfs] ERROR: fakefsify failed: data/ not found" >&2
    exit 1
fi
if [ ! -f "$FAKEFS_DIR/meta.db" ]; then
    echo "[prepare_rootfs] ERROR: fakefsify failed: meta.db not found" >&2
    exit 1
fi

cd "$FAKEFS_DIR"

echo "[prepare_rootfs] Creating device mounts placeholders"
mkdir -p data/dev data/proc data/sys data/tmp data/run data/root data/home

echo "[prepare_rootfs] Ensuring /etc/passwd"
if [ ! -f data/etc/passwd ]; then
    printf 'root:x:0:0:root:/root:/bin/sh\n' > data/etc/passwd
fi

echo "[prepare_rootfs] Ensuring /etc/apk/repositories points to matching v${VERSION}"
APK_DIR="data/etc/apk"
mkdir -p "$APK_DIR"
MINOR="${VERSION#*.}"
MAJOR="${VERSION%%.*}"
cat > "$APK_DIR/repositories" <<EOF
https://dl-cdn.alpinelinux.org/alpine/v${MAJOR}.${MINOR}/main
https://dl-cdn.alpinelinux.org/alpine/v${MAJOR}.${MINOR}/community
EOF

echo "[prepare_rootfs] Creating version manifest"
cat > rootfs.manifest.json <<EOF
{
  "schemaVersion": 1,
  "format": "ish_fakefs",
  "formatVersion": "1",
  "distribution": "alpine",
  "version": "${VERSION}",
  "architecture": "aarch64",
  "packageSha256": "PENDING",
  "sourceType": "bundled",
  "installedAt": "PENDING"
}
EOF

echo "[prepare_rootfs] Creating ZIP package"
ZIP_TMP="$WORK/rootfs-tmp"
rm -rf "$ZIP_TMP" "$ROOTFS_ZIP"
mkdir -p "$ZIP_TMP"
cp -R data "$ZIP_TMP/"
cp meta.db "$ZIP_TMP/"
cp rootfs.manifest.json "$ZIP_TMP/"

cd "$ZIP_TMP"
zip -r -9 "$ROOTFS_ZIP" data meta.db rootfs.manifest.json

PACKAGE_SHA256=$(shasum -a 256 "$ROOTFS_ZIP" | cut -d' ' -f1)

echo "[prepare_rootfs] Package SHA256: $PACKAGE_SHA256"

PYTHON_BIN=""
if command -v python3 >/dev/null 2>&1; then
    PYTHON_BIN="python3"
elif command -v python >/dev/null 2>&1; then
    PYTHON_BIN="python"
fi

if [ -n "$PYTHON_BIN" ]; then
    cd "$ROOT_DIR"
    $PYTHON_BIN -c "
import json, sys
digest = sys.argv[1]
version = sys.argv[2]
path = sys.argv[3]
obj = {
    'schemaVersion': 1,
    'distribution': 'alpine',
    'version': version,
    'architecture': 'aarch64',
    'format': 'ish_fakefs',
    'asset': 'alpine-rootfs.zip',
    'sha256': digest
}
with open(path, 'w') as f:
    json.dump(obj, f, indent=2)
    f.write('\n')
" "$PACKAGE_SHA256" "$VERSION" "$RELEASE_MANIFEST"

    $PYTHON_BIN -c "
import json, sys, os
digest = sys.argv[1]
manifest = sys.argv[2]
with open(manifest) as f:
    mf = json.load(f)
mf['packageSha256'] = digest
with open(manifest, 'w') as f:
    json.dump(mf, f, indent=2)
    f.write('\n')
" "$PACKAGE_SHA256" "$ZIP_TMP/rootfs.manifest.json"

    cd "$ZIP_TMP"
    rm -f "$ROOTFS_ZIP"
    zip -r -9 "$ROOTFS_ZIP" data meta.db rootfs.manifest.json
fi

echo "[prepare_rootfs] Output complete:"
echo "  ZIP:         $ROOTFS_ZIP"
echo "  Manifest:    $RELEASE_MANIFEST"
echo "  SHA256:      $PACKAGE_SHA256"
echo "  Alpine:      $VERSION aarch64"
echo "  Src SHA256:  $ALPINE_SHA256"
