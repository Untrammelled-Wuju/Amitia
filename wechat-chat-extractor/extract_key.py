# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
import argparse
import os
import sys

from wechat_extractor.config import auto_detect_db_dir
from wechat_extractor.key_scanner import extract_keys


def main():
    parser = argparse.ArgumentParser(description="提取微信数据库解密密钥")
    parser.add_argument("--db-dir", help="微信数据库目录（默认自动检测）")
    parser.add_argument("--output", "-o", default="all_keys.json", help="输出JSON文件路径 (默认: all_keys.json)")
    parser.add_argument("--pid", type=int, help="指定微信进程PID（默认自动检测）")
    args = parser.parse_args()

    db_dir = args.db_dir or auto_detect_db_dir()
    if not db_dir:
        print("[!] 未能自动检测微信数据目录", file=sys.stderr)
        print("请通过 --db-dir 参数手动指定", file=sys.stderr)
        sys.exit(1)

    print(f"[+] 微信数据目录: {db_dir}")

    output_path = os.path.abspath(args.output)
    try:
        key_map = extract_keys(db_dir, output_path, pid=args.pid)
        print(f"\n[+] 成功提取 {len(key_map)} 个数据库密钥")
        print(f"    密钥文件: {output_path}")
    except RuntimeError as e:
        print(f"\n[!] 密钥提取失败: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
