#!/bin/bash
# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
# Qdrant 二进制下载脚本 (Linux)
# 用法: bash scripts/download-qdrant.sh

set -e

VERSION="1.9.0"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
QDRANT_DIR="$ROOT_DIR/qdrant"
CONFIG_DIR="$QDRANT_DIR/config"

mkdir -p "$QDRANT_DIR"
mkdir -p "$CONFIG_DIR"

ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  QDRANT_ARCH="x86_64-unknown-linux-gnu" ;;
    aarch64) QDRANT_ARCH="aarch64-unknown-linux-gnu" ;;
    *)       echo "不支持的架构: $ARCH"; exit 1 ;;
esac

URL="https://github.com/qdrant/qdrant/releases/download/v${VERSION}/qdrant-${QDRANT_ARCH}.tar.gz"
TARGET_NAME="qdrant_linux_x86"
if [ "$ARCH" = "aarch64" ]; then
    TARGET_NAME="qdrant_linux_aarch64"
fi

echo "下载 Qdrant v${VERSION} for Linux (${ARCH})..."
if command -v curl &> /dev/null; then
    curl -L "$URL" -o "$QDRANT_DIR/qdrant.tar.gz"
elif command -v wget &> /dev/null; then
    wget "$URL" -O "$QDRANT_DIR/qdrant.tar.gz"
else
    echo "错误: 需要 curl 或 wget"
    exit 1
fi

echo "解压..."
tar xzf "$QDRANT_DIR/qdrant.tar.gz" -C "$QDRANT_DIR"
rm "$QDRANT_DIR/qdrant.tar.gz"
mv "$QDRANT_DIR/qdrant" "$QDRANT_DIR/$TARGET_NAME"
chmod +x "$QDRANT_DIR/$TARGET_NAME"

echo "Qdrant 下载完成: $QDRANT_DIR/$TARGET_NAME"
echo "配置目录已创建: $CONFIG_DIR"
echo "Qdrant 会在启动时自动生成配置文件"
