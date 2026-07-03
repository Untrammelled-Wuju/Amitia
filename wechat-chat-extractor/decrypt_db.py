# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
import argparse
import json
import os
import sys

from wechat_extractor.crypto import full_decrypt, decrypt_wal
from wechat_extractor.config import auto_detect_db_dir


def main():
    parser = argparse.ArgumentParser(description="解密微信数据库")
    parser.add_argument("--db-dir", help="微信数据库目录（默认自动检测）")
    parser.add_argument("--keys", "-k", default="all_keys.json", help="密钥JSON文件 (默认: all_keys.json)")
    parser.add_argument("--output", "-o", default="decrypted", help="解密输出目录 (默认: decrypted)")
    parser.add_argument("--db", help="只解密指定数据库（相对路径）")
    args = parser.parse_args()

    if not os.path.exists(args.keys):
        print(f"[!] 密钥文件不存在: {args.keys}", file=sys.stderr)
        print("请先运行 extract_key.py 提取密钥", file=sys.stderr)
        sys.exit(1)

    with open(args.keys, "r", encoding="utf-8") as f:
        all_keys = json.load(f)

    db_dir = args.db_dir or auto_detect_db_dir()
    if not db_dir:
        print("[!] 未能自动检测微信数据目录", file=sys.stderr)
        sys.exit(1)

    print(f"[+] 微信数据目录: {db_dir}")
    print(f"[+] 找到 {len(all_keys)} 个数据库密钥")

    if args.db:
        if args.db not in all_keys:
            print(f"[!] 密钥文件中未找到: {args.db}", file=sys.stderr)
            sys.exit(1)
        targets = {args.db: all_keys[args.db]}
    else:
        targets = all_keys

    output_dir = os.path.abspath(args.output)
    success = 0
    failed = 0

    for rel_path, key_info in targets.items():
        enc_key = bytes.fromhex(key_info["enc_key"])
        db_path = os.path.join(db_dir, rel_path)
        out_path = os.path.join(output_dir, rel_path)

        if not os.path.exists(db_path):
            print(f"  SKIP: {rel_path} (文件不存在)")
            continue

        try:
            pages = full_decrypt(db_path, out_path, enc_key)
            wal_path = db_path + "-wal"
            if os.path.exists(wal_path):
                wal_pages = decrypt_wal(wal_path, out_path, enc_key)
                print(f"  OK: {rel_path} ({pages} pages + {wal_pages} WAL frames)")
            else:
                print(f"  OK: {rel_path} ({pages} pages)")
            success += 1
        except Exception as e:
            print(f"  FAIL: {rel_path} ({e})")
            failed += 1

    print(f"\n[+] 解密完成: {success} 成功, {failed} 失败")
    print(f"    输出目录: {output_dir}")


if __name__ == "__main__":
    main()
