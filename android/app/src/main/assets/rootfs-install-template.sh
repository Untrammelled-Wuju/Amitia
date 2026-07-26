#!/bin/sh
set -e

ROOTFS_DIR="${1:-$HOME/.amitia/rootfs}"
BIN_DIR="$ROOTFS_DIR/bin"
ETC_DIR="$ROOTFS_DIR/etc"
DATA_DIR="${2:-$HOME/.amitia/data}"
LOG_DIR="$ROOTFS_DIR/logs"

ASSETS_DIR="$(cd "$(dirname "$0")" && pwd)"

mkdir -p "$BIN_DIR" "$ETC_DIR" "$DATA_DIR" "$LOG_DIR" "$DATA_DIR/sqlite" "$DATA_DIR/qdrant" "$DATA_DIR/surrealdb" "$DATA_DIR/uploads" "$DATA_DIR/models" "$DATA_DIR/extensions" "$DATA_DIR/backups"

cp "$ASSETS_DIR/amitia-backend-arm64" "$BIN_DIR/amitia-backend"
cp "$ASSETS_DIR/qdrant_linux_aarch64" "$BIN_DIR/qdrant"
cp "$ASSETS_DIR/surreal_linux_aarch64" "$BIN_DIR/surreal"

chmod +x "$BIN_DIR/amitia-backend" "$BIN_DIR/qdrant" "$BIN_DIR/surreal"

cat > "$ETC_DIR/config.yml" <<EOF
server:
  port: 18899
  host: "127.0.0.1"
  mode: "release"
storage:
  dataDir: "$DATA_DIR"
jwt:
  secret: "AMITIA_LOCAL_AUTH_SECRET"
  expireDays: 7
app:
  name: "Amitia"
  version: "26.1.8"
  deployMode: "android-embedded"
qdrant:
  host: "127.0.0.1"
  port: 19178
surrealdb:
  host: "127.0.0.1"
  port: 18000
  namespace: "uai"
  database: "memory_graph"
  username: "root"
  password: "root"
  dataPath: "$DATA_DIR/surrealdb/graph.db"
EOF

cat > "$ROOTFS_DIR/VERSION" <<EOF
amitia-rootfs=1.0.0
createdAt=2026-07-26
backend_sha256=ABDD63EB020A01718684EDCF130785DA0CFD45DCB691AAB5D260A8E17386879B
qdrant_sha256=6CB81123D2A3E405335C984EFB7928DC21A1EAC47A3B73A7609AEE076DBE0B04
surreal_sha256=A235206F2C4A803616D7669F56BC5BCA5CC0AE7ED2B79ACBE91F14712B28FE5C
EOF

echo "RootFS installed at: $ROOTFS_DIR"
echo "User data dir: $DATA_DIR"
echo "Binaries: amitia-backend, qdrant, surreal"
echo "Config: $ETC_DIR/config.yml"
echo "Done."
