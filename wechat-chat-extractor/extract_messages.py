# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
import argparse
import json
import os
import sys
from datetime import datetime

from wechat_extractor.messages import list_chats, extract_messages, export_chat_to_json


def main():
    parser = argparse.ArgumentParser(description="提取微信聊天记录")
    subparsers = parser.add_subparsers(dest="command", help="子命令")

    list_parser = subparsers.add_parser("list", help="列出所有聊天会话")
    list_parser.add_argument("--decrypted", "-d", default="decrypted", help="解密数据库目录 (默认: decrypted)")
    list_parser.add_argument("--format", choices=["table", "json"], default="table", help="输出格式")

    chat_parser = subparsers.add_parser("chat", help="提取指定聊天的消息")
    chat_parser.add_argument("chat_name", help="聊天对象名称（支持模糊匹配）")
    chat_parser.add_argument("--decrypted", "-d", default="decrypted", help="解密数据库目录 (默认: decrypted)")
    chat_parser.add_argument("--limit", "-n", type=int, default=50, help="消息数量 (默认: 50)")
    chat_parser.add_argument("--offset", type=int, default=0, help="偏移量")
    chat_parser.add_argument("--start-time", help="开始时间 (YYYY-MM-DD HH:MM:SS)")
    chat_parser.add_argument("--end-time", help="结束时间 (YYYY-MM-DD HH:MM:SS)")
    chat_parser.add_argument("--export", "-o", help="导出到JSON文件")

    args = parser.parse_args()

    if args.command == "list":
        decrypted_dir = os.path.abspath(args.decrypted)
        if not os.path.isdir(decrypted_dir):
            print(f"[!] 解密目录不存在: {decrypted_dir}", file=sys.stderr)
            print("请先运行 decrypt_db.py 解密数据库", file=sys.stderr)
            sys.exit(1)

        chats = list_chats(decrypted_dir)

        if args.format == "json":
            print(json.dumps(chats, indent=2, ensure_ascii=False))
        else:
            print(f"{'会话名称':<30} {'消息数':>8} {'最后活跃':<20} {'类型':<6}")
            print("-" * 70)
            for c in chats:
                tag = "群聊" if c["is_group"] else "私聊"
                print(f"{c['display_name']:<30} {c['message_count']:>8} {c['last_message_time']:<20} {tag:<6}")

    elif args.command == "chat":
        decrypted_dir = os.path.abspath(args.decrypted)
        if not os.path.isdir(decrypted_dir):
            print(f"[!] 解密目录不存在: {decrypted_dir}", file=sys.stderr)
            print("请先运行 decrypt_db.py 解密数据库", file=sys.stderr)
            sys.exit(1)

        start_ts = None
        end_ts = None
        if args.start_time:
            start_ts = int(datetime.strptime(args.start_time, "%Y-%m-%d %H:%M:%S").timestamp())
        if args.end_time:
            end_ts = int(datetime.strptime(args.end_time, "%Y-%m-%d %H:%M:%S").timestamp())

        chats = list_chats(decrypted_dir)

        matched = None
        query = args.chat_name.lower()
        for c in chats:
            if query == c["username"].lower() or query == c["display_name"].lower():
                matched = c
                break
        if not matched:
            for c in chats:
                if query in c["username"].lower() or query in c["display_name"].lower():
                    matched = c
                    break

        if not matched:
            print(f"[!] 未找到匹配的聊天: {args.chat_name}", file=sys.stderr)
            sys.exit(1)

        if args.export:
            count = export_chat_to_json(
                matched["db_path"], matched["table_name"],
                matched["username"], args.export,
                start_time=start_ts, end_time=end_ts
            )
            print(f"[+] 导出 {count} 条消息到: {args.export}")
        else:
            messages = extract_messages(
                matched["db_path"], matched["username"],
                limit=args.limit, offset=args.offset,
                start_time=start_ts, end_time=end_ts
            )
            print(f"\n与 {matched['display_name']} 的聊天记录 ({len(messages)} 条):")
            print("-" * 60)
            for m in messages:
                sender = m["sender"] or m["sender_id"] or matched["display_name"]
                print(f"[{m['time']}] {sender}: {m['content'][:200]}")
                if m["type"] != "文本":
                    print(f"  [{m['type']}]")

    else:
        parser.print_help()


if __name__ == "__main__":
    main()
