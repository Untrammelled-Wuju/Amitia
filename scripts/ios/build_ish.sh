#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
ISH_SRC="$ROOT_DIR/backend/third_party/ish-arm64"
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
    echo "[build_ish] ERROR: pinned ARM64 fork source missing" >&2
    exit 1
fi

cd "$ISH_SRC"

BUILD_DIR="$ISH_SRC/build-ios"
CROSS_FILE="$BUILD_DIR/ios-cross.txt"
mkdir -p "$BUILD_DIR"
SDK_PATH="$(xcrun --sdk iphoneos --show-sdk-path)"
CLANG_PATH="$(xcrun --sdk iphoneos --find clang)"
AR_PATH="$(xcrun --sdk iphoneos --find ar)"
STRIP_PATH="$(xcrun --sdk iphoneos --find strip)"

cat > "$CROSS_FILE" <<CROSS
[binaries]
c = ['$CLANG_PATH', '-arch', 'arm64', '-mios-version-min=14.0', '-isysroot', '$SDK_PATH', '-fblocks']
ar = '$AR_PATH'
strip = '$STRIP_PATH'

[host_machine]
system = 'darwin'
cpu_family = 'aarch64'
cpu = 'aarch64'
endian = 'little'
CROSS

SETUP_ARGS=()
if [ -f "$BUILD_DIR/meson-private/coredata.dat" ]; then SETUP_ARGS+=(--reconfigure); fi
meson setup "${SETUP_ARGS[@]}" "$BUILD_DIR" \
    --wrap-mode=nodownload \
    --cross-file "$CROSS_FILE" \
    --buildtype=release \
    -Dlog="" \
    -Dlog_handler=nslog \
    -Dkernel=ish \
    -Dengine=asbestos \
    -Dguest_arch=arm64

ninja -C "$BUILD_DIR" libish.a libish_emu.a libfakefs.a
ninja -C "$BUILD_DIR" vdso/arm64/libvdso.so.elf
python3 "$SCRIPT_DIR/rootfs_inputs.py" verify-elf "$BUILD_DIR/vdso/arm64/libvdso.so.elf"
"$CLANG_PATH" -arch arm64 -mios-version-min=14.0 -isysroot "$SDK_PATH" -fblocks -std=gnu11 \
    -DGUEST_ARM64=1 -DENGINE_ASBESTOS=1 -DLOG_HANDLER_NSLOG=1 -I"$ISH_SRC" -I"$BUILD_DIR" \
    -c "$ISH_SRC/amitia/amitia_ish_embed.c" -o "$BUILD_DIR/amitia_ish_embed.o"
"$AR_PATH" rcs "$BUILD_DIR/libamitia_ish_embed.a" "$BUILD_DIR/amitia_ish_embed.o"

mkdir -p "$OUTPUT_DIR/include"
mkdir -p "$OUTPUT_DIR/lib"
mkdir -p "$OUTPUT_DIR/resources"

cp "$BUILD_DIR/libish.a" "$OUTPUT_DIR/lib/"
cp "$BUILD_DIR/libish_emu.a" "$OUTPUT_DIR/lib/"
cp "$BUILD_DIR/libfakefs.a" "$OUTPUT_DIR/lib/"
cp "$BUILD_DIR/libamitia_ish_embed.a" "$OUTPUT_DIR/lib/"
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
