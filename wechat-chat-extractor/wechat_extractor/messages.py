# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
import hashlib
import json
import os
import re
import sqlite3
from contextlib import closing
from datetime import datetime

try:
    import zstandard as zstd
    _zstd_dctx = zstd.ZstdDecompressor()
    _HAS_ZSTD = True
except ImportError:
    _HAS_ZSTD = False

MSG_TYPE_NAMES = {
    1: "文本", 3: "图片", 34: "语音", 42: "名片",
    43: "视频", 47: "表情", 48: "位置", 49: "链接/文件",
    50: "通话", 10000: "系统", 10002: "撤回",
}


def _split_msg_type(t):
    try:
        t = int(t)
    except (TypeError, ValueError):
        return 0, 0
    if t > 0xFFFFFFFF:
        return t & 0xFFFFFFFF, t >> 32
    return t, 0


def format_msg_type(t):
    base_type, _ = _split_msg_type(t)
    return MSG_TYPE_NAMES.get(base_type, f"type={t}")


def decompress_content(content, ct):
    if ct and ct == 4 and isinstance(content, bytes):
        if _HAS_ZSTD:
            try:
                return _zstd_dctx.decompress(content).decode("utf-8", errors="replace")
            except Exception:
                return None
        else:
            return "(zstd压缩内容，需安装zstandard库)"
    if isinstance(content, bytes):
        try:
            return content.decode("utf-8", errors="replace")
        except Exception:
            return None
    return content


def _parse_message_content(content, local_type, is_group):
    if content is None:
        return "", ""
    if isinstance(content, bytes):
        return "", "(二进制内容)"
    sender = ""
    text = content
    if is_group and ":\n" in content:
        sender, text = content.split(":\n", 1)
    return sender, text


def _is_safe_table_name(table_name):
    return bool(re.fullmatch(r"Msg_[0-9a-f]{32}", table_name))


def _load_name2id_maps(conn):
    id_to_username = {}
    try:
        for uid, uname in conn.execute("SELECT id, user_name FROM Name2Id").fetchall():
            if uname:
                id_to_username[uid] = uname
    except sqlite3.Error:
        pass
    return id_to_username


def load_contact_names(decrypted_dir):
    contact_db = os.path.join(decrypted_dir, "contact", "contact.db")
    if not os.path.exists(contact_db):
        return {}
    names = {}
    try:
        conn = sqlite3.connect(contact_db)
        for uname, nick, remark in conn.execute(
            "SELECT username, nick_name, remark FROM contact"
        ).fetchall():
            display = remark if remark else nick if nick else uname
            names[uname] = display
        conn.close()
    except sqlite3.Error:
        pass
    return names


def find_msg_databases(decrypted_dir):
    db_files = []
    for root, dirs, files in os.walk(decrypted_dir):
        for f in files:
            if re.search(r"message_?\d*\.db$", f) and not f.endswith("-wal") and not f.endswith("-shm"):
                db_files.append(os.path.join(root, f))
    return sorted(db_files)


def extract_messages(db_path, username=None, limit=100, offset=0, start_time=None, end_time=None):
    names = {}
    contact_db = os.path.join(os.path.dirname(db_path), "..", "contact", "contact.db")
    if os.path.exists(contact_db):
        names = load_contact_names(os.path.dirname(contact_db))

    messages = []
    conn = sqlite3.connect(db_path)
    try:
        tables = conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'Msg_%'"
        ).fetchall()
        id_to_username = _load_name2id_maps(conn)

        for (table_name,) in tables:
            if not _is_safe_table_name(table_name):
                continue

            tname = table_name
            username_from_table = None
            for uid, uname in id_to_username.items():
                if hashlib.md5(uname.encode()).hexdigest() == tname[4:]:
                    username_from_table = uname
                    break

            if username and username_from_table and username_from_table != username:
                continue

            where_parts = []
            params = []
            if start_time:
                where_parts.append("create_time >= ?")
                params.append(int(start_time))
            if end_time:
                where_parts.append("create_time <= ?")
                params.append(int(end_time))
            where_sql = f"WHERE {' AND '.join(where_parts)}" if where_parts else ""

            try:
                rows = conn.execute(
                    f"SELECT create_time, local_type, content, compress_type, real_sender_id "
                    f"FROM [{tname}] {where_sql} ORDER BY create_time DESC LIMIT ? OFFSET ?",
                    params + [limit, offset]
                ).fetchall()

                for create_time, local_type, content, compress_type, real_sender_id in rows:
                    base_type, sub_type = _split_msg_type(local_type)
                    is_group = username_from_table and "@chatroom" in username_from_table

                    decompressed = decompress_content(content, compress_type)
                    sender, text = _parse_message_content(decompressed, local_type, is_group)

                    sender_name = ""
                    if real_sender_id and real_sender_id in id_to_username:
                        sender_name = names.get(id_to_username[real_sender_id], id_to_username[real_sender_id])

                    dt = datetime.fromtimestamp(create_time)
                    messages.append({
                        "time": dt.strftime("%Y-%m-%d %H:%M:%S"),
                        "timestamp": create_time,
                        "type": format_msg_type(local_type),
                        "type_code": base_type,
                        "sender": sender or sender_name,
                        "sender_id": id_to_username.get(real_sender_id, str(real_sender_id)) if real_sender_id else "",
                        "content": text or "",
                        "chat": username_from_table or tname,
                        "is_group": is_group,
                    })
            except sqlite3.Error:
                continue
    finally:
        conn.close()

    messages.sort(key=lambda m: m["timestamp"], reverse=True)
    return messages[:limit]


def list_chats(decrypted_dir):
    chats = []
    db_files = find_msg_databases(decrypted_dir)
    names = load_contact_names(decrypted_dir)

    for db_path in db_files:
        try:
            conn = sqlite3.connect(db_path)
            tables = conn.execute(
                "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'Msg_%'"
            ).fetchall()
            id_to_username = _load_name2id_maps(conn)

            for (table_name,) in tables:
                if not _is_safe_table_name(table_name):
                    continue
                tname = table_name
                username = None
                for uid, uname in id_to_username.items():
                    if hashlib.md5(uname.encode()).hexdigest() == tname[4:]:
                        username = uname
                        break
                if not username:
                    username = tname

                count_row = conn.execute(f"SELECT COUNT(*), MAX(create_time) FROM [{tname}]").fetchone()
                msg_count = count_row[0] if count_row else 0
                last_time = count_row[1] if count_row and count_row[1] else 0

                display_name = names.get(username, username)
                is_group = "@chatroom" in username

                chats.append({
                    "username": username,
                    "display_name": display_name,
                    "is_group": is_group,
                    "message_count": msg_count,
                    "last_message_time": datetime.fromtimestamp(last_time).strftime("%Y-%m-%d %H:%M:%S") if last_time else "",
                    "last_timestamp": last_time,
                    "db_path": db_path,
                    "table_name": tname,
                })
            conn.close()
        except sqlite3.Error:
            continue

    chats.sort(key=lambda c: c["last_timestamp"], reverse=True)
    return chats


def export_chat_to_json(db_path, table_name, username, output_path, start_time=None, end_time=None):
    names = {}
    contact_db = os.path.join(os.path.dirname(db_path), "..", "contact", "contact.db")
    if os.path.exists(contact_db):
        names = load_contact_names(os.path.dirname(contact_db))

    messages = []
    conn = sqlite3.connect(db_path)
    try:
        id_to_username = _load_name2id_maps(conn)
        where_parts = []
        params = []
        if start_time:
            where_parts.append("create_time >= ?")
            params.append(int(start_time))
        if end_time:
            where_parts.append("create_time <= ?")
            params.append(int(end_time))
        where_sql = f"WHERE {' AND '.join(where_parts)}" if where_parts else ""

        rows = conn.execute(
            f"SELECT create_time, local_type, content, compress_type, real_sender_id "
            f"FROM [{table_name}] {where_sql} ORDER BY create_time ASC",
            params
        ).fetchall()

        is_group = "@chatroom" in (username or "")

        for create_time, local_type, content, compress_type, real_sender_id in rows:
            decompressed = decompress_content(content, compress_type)
            sender, text = _parse_message_content(decompressed, local_type, is_group)

            sender_name = ""
            if real_sender_id and real_sender_id in id_to_username:
                sender_name = names.get(id_to_username[real_sender_id], id_to_username[real_sender_id])

            dt = datetime.fromtimestamp(create_time)
            messages.append({
                "time": dt.strftime("%Y-%m-%d %H:%M:%S"),
                "timestamp": create_time,
                "type": format_msg_type(local_type),
                "type_code": local_type,
                "sender": sender or sender_name,
                "content": text or "",
            })
    finally:
        conn.close()

    os.makedirs(os.path.dirname(output_path) or ".", exist_ok=True)
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(messages, f, indent=2, ensure_ascii=False)

    return len(messages)
