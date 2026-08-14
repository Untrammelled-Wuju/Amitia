#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
ISH_SRC="$ROOT_DIR/backend/third_party/ish"
OUTPUT_DIR="$ROOT_DIR/mobile_app/ios/ThirdParty/iSH"

if [ "$(uname)" != "Darwin" ]; then
    echo "[build_ish] ERROR: iSH build requires macOS" >&2
    exit 1
fi

if ! command -v meson >/dev/null 2>&1; then
    echo "[build_ish] ERROR: meson not found. Install via: brew install meson" >&2
    exit 1
fi

if ! command -v ninja >/dev/null 2>&1; then
    echo "[build_ish] ERROR: ninja not found. Install via: brew install ninja" >&2
    exit 1
fi

cd "$ROOT_DIR"

if [ ! -f "$ISH_SRC/meson.build" ]; then
    echo "[build_ish] Initializing iSH submodule..." >&2
    git submodule update --init --recursive backend/third_party/ish
fi

cd "$ISH_SRC"

BUILD_DIR="$ISH_SRC/build-ios"
CROSS_FILE="$ISH_SRC/ios-cross.txt"

cat > "$CROSS_FILE" <<'CROSS'
[binaries]
c = ['clang', '-arch', 'arm64', '-mios-version-min=14.0', '-isysroot', '/Applications/Xcode.app/Contents/Developer/Platforms/iPhoneOS.platform/Developer/SDKs/iPhoneOS.sdk']
cpp = ['clang++', '-arch', 'arm64', '-mios-version-min=14.0', '-isysroot', '/Applications/Xcode.app/Contents/Developer/Platforms/iPhoneOS.platform/Developer/SDKs/iPhoneOS.sdk']
ar = 'ar'
strip = 'strip'

[host_machine]
system = 'darwin'
cpu_family = 'aarch64'
cpu = 'aarch64'
endian = 'little'
CROSS

meson setup "$BUILD_DIR" \
    --cross-file "$CROSS_FILE" \
    --buildtype=release \
    -Dlog="" \
    -Dlog_handler=nslog \
    -Dkernel=ish \
    -Dengine=asbestos \
    -Dguest_arch=arm64

ninja -C "$BUILD_DIR" libish.a libish_emu.a libfakefs.a
ninja -C "$BUILD_DIR" vdso/arm64/libvdso.so.elf

mkdir -p "$OUTPUT_DIR/include"
mkdir -p "$OUTPUT_DIR/lib"
mkdir -p "$OUTPUT_DIR/resources"

cp "$BUILD_DIR/libish.a" "$OUTPUT_DIR/lib/"
cp "$BUILD_DIR/libish_emu.a" "$OUTPUT_DIR/lib/"
cp "$BUILD_DIR/libfakefs.a" "$OUTPUT_DIR/lib/"
cp "$BUILD_DIR/vdso/arm64/libvdso.so.elf" "$OUTPUT_DIR/resources/libvdso.so.elf"

cp -R "$ISH_SRC/kernel" "$OUTPUT_DIR/include/"
cp -R "$ISH_SRC/fs" "$OUTPUT_DIR/include/"
cp -R "$ISH_SRC/emu" "$OUTPUT_DIR/include/"
cp -R "$ISH_SRC/util" "$OUTPUT_DIR/include/"
cp -R "$ISH_SRC/platform" "$OUTPUT_DIR/include/"
cp -R "$ISH_SRC/asbestos" "$OUTPUT_DIR/include/"

if [ -f "$ISH_SRC/amitia/amitia_ish_embed.h" ]; then
    cp "$ISH_SRC/amitia/amitia_ish_embed.h" "$OUTPUT_DIR/include/"
fi

echo "[build_ish] iSH build complete. Output: $OUTPUT_DIR"
echo "[build_ish] Libraries:"
ls -la "$OUTPUT_DIR/lib/"
echo "[build_ish] VDSO:"
ls -la "$OUTPUT_DIR/resources/"
