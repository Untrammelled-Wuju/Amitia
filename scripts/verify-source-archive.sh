#!/usr/bin/env bash
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SOURCE_PATH="${1:-$REPO_ROOT}"

if [ ! -d "$SOURCE_PATH" ]; then
  echo "Source Archive Guard: 目录不存在: $SOURCE_PATH" >&2
  exit 1
fi

findings=0

echo "=== Source Archive Guard ==="
echo "扫描目录: $SOURCE_PATH"

FORBIDDEN_NAMES="local-token mcp-secrets.key device.json qrcode.png"
FORBIDDEN_SUFFIXES=".db .db-wal .db-shm .db-journal .log .exe .dll .pdb"
FORBIDDEN_DIRS="AmitiaData migration_backups migration_backups_prev node_modules data.db"
BINARY_NAMES="server backend amitia-ext amitiax surreal qdrant"

LEAKED_SECRETS="c3j22EREBHyFPEcP09s6C5cPSa-xxY3Vfslq2xZILbI Am1t1a-D3skt0p-L0c4l-S3cr3t-K3y-2026 D3f4ult-R00t-JWT-S3cr3t-K3y-2026-Xyz Rj2Lf0Lh4RpEe4VwOrKoInJpMuS1bBmO hMMyAMy8t2JX sNRZFXQRAynD"

report() {
  echo "违禁内容: $1"
  findings=$((findings + 1))
}

while IFS= read -r -d '' file; do
  rel="${file#"$SOURCE_PATH"/}"
  name="$(basename "$file")"

  for dn in $FORBIDDEN_DIRS; do
    case "/$rel/" in
      */"$dn"/*) report "违禁目录: $rel" ;;
    esac
  done

  for fn in $FORBIDDEN_NAMES; do
    if [ "$name" = "$fn" ]; then
      report "违禁文件: $rel"
    fi
  done

  case "$rel" in
    desktop/resources/core/*|backend/node/*|backend/security/node*)
      ALLOWED=1
      ;;
    *)
      ALLOWED=0
      ;;
  esac

  if [ "$ALLOWED" = "0" ]; then
    for sfx in $FORBIDDEN_SUFFIXES; do
      case "$name" in
        *"$sfx") report "违禁文件类型: $rel" ;;
      esac
    done

    case "$name" in
      server|server.exe|backend|backend.exe|amitia-ext|amitia-ext.exe|amitiax|amitiax.exe|surreal|surreal.exe|qdrant|qdrant.exe)
        report "已编译产物: $rel"
        ;;
    esac
  fi

  case "$name" in
    backend-source*.zip|backend-src*.zip)
      report "源码归档混入: $rel"
      ;;
  esac

  case "$rel" in
    qdrant/storage/*|qdrant/snapshots/*|surrealdb/data/*|surrealdb/storage/*|backend/qdrant/storage/*|backend/qdrant/snapshots/*|desktop/resources/qdrant/storage/*|desktop/resources/qdrant/snapshots/*)
      report "向量库运行时数据: $rel"
      ;;
  esac

  case "$rel" in
    scripts/verify-source-archive.sh|scripts/verify-source-archive.ps1)
      SKIP_SECRET_SCAN=1
      ;;
    *)
      SKIP_SECRET_SCAN=0
      ;;
  esac

  if [ "$SKIP_SECRET_SCAN" = "0" ]; then
    case "$name" in
      *.go|*.ts|*.tsx|*.js|*.vue|*.dart|*.json|*.yml|*.yaml|*.md|*.toml|*.ps1|*.sh|*.py|*.sql)
        for secret in $LEAKED_SECRETS; do
          if grep -qF "$secret" "$file" 2>/dev/null; then
            report "含已知泄露凭据($secret): $rel"
          fi
        done
        ;;
    esac
  fi
done < <(find "$SOURCE_PATH" \
  -type d \( -name .git -o -name node_modules -o -name mobile_app -o -name wechat-chat-extractor \) -prune -o \
  -type f -print0)

if [ "$findings" -gt 0 ]; then
  echo ""
  echo "Source Archive Guard 失败: $findings 项违禁内容"
  exit 1
fi

echo ""
echo "Source Archive Guard 通过，未发现运行数据或泄露凭据"
exit 0
