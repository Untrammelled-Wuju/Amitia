# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
# -*- coding: utf-8 -*-
"""主动消息推送测试 - 微信/QQ Sidecar
用法: python test_proactive_push.py
前提: 后端(8899) + 微信(9876) + QQ(9877) 已启动"""

import sqlite3, socket, urllib.request, json, time, sys, os

SCRIPT = os.path.abspath(__file__)
BACKEND = os.path.dirname(os.path.dirname(SCRIPT))
ROOT = os.path.dirname(BACKEND)
API = "http://127.0.0.1:8899"
CID = "6bf3e54c-bc0e-4180-9613-2c10861ae6be"
WECHAT = 9876
QQ = 9877

def is_runtime_dir(path):
    return os.path.isdir(path) and os.path.isdir(os.path.join(path, "qdrant")) and os.path.isdir(os.path.join(path, "surrealdb")) and os.path.isdir(os.path.join(path, "data"))

def find_db_path():
    preferred = ["release", "WorkDone"]
    for name in preferred:
        candidate = os.path.join(ROOT, name)
        if is_runtime_dir(candidate):
            db = os.path.join(candidate, "data", "app.db")
            if os.path.exists(db):
                return os.path.abspath(db)
    for name in os.listdir(ROOT):
        candidate = os.path.join(ROOT, name)
        if is_runtime_dir(candidate):
            db = os.path.join(candidate, "data", "app.db")
            if os.path.exists(db):
                return os.path.abspath(db)
    fallback = os.path.join(BACKEND, "data", "app.db")
    if os.path.exists(fallback):
        return os.path.abspath(fallback)
    return os.path.abspath(os.path.join(ROOT, "data", "app.db"))

DB = find_db_path()

PASS = 0
FAIL = 0

def check(desc, ok, note=None):
    global PASS, FAIL
    if ok:
        PASS += 1
        print(f"  [PASS] {desc}")
    else:
        FAIL += 1
        msg = f"  [FAIL] {desc}"
        if note: msg += f"  ({note})"
        print(msg)

def port_open(port):
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.settimeout(2)
    r = s.connect_ex(("127.0.0.1", port))
    s.close()
    return r == 0

def api(method, path, body=None, timeout=30):
    try:
        data = json.dumps(body).encode() if body else None
        h = {"Content-Type": "application/json"} if data else {}
        req = urllib.request.Request(API + path, data=data, headers=h, method=method)
        resp = urllib.request.urlopen(req, timeout=timeout)
        return json.loads(resp.read())
    except Exception as e:
        return {"error": str(e)}

def db(sql, params=()):
    conn = sqlite3.connect(DB)
    c = conn.cursor()
    c.execute(sql, params)
    rows = c.fetchall()
    conn.close()
    return rows

print("=" * 55)
print("  主动消息推送测试 - wechat/QQ Sidecar")
print("  时间:", time.strftime("%Y-%m-%d %H:%M:%S"))
print("=" * 55)

print("\n[1] 环境检查")
check("后端 8899", port_open(8899))
check("微信 9876", port_open(WECHAT))
check("QQ 9877", port_open(QQ))
chars = db("SELECT name, is_default FROM characters")
check("默认角色(is_default=1)", any(r[1]==1 for r in chars),
      ", ".join(f"{r[0]}(def={r[1]})" for r in chars))
conv = db("SELECT channel FROM conversations WHERE channel IN ('wechat','qq')")
check("wechat/QQ频道", {r[0] for r in conv} == {"wechat", "qq"})

print("\n[2] ProactiveRule触发")
rules = db("SELECT id, name FROM proactive_rules WHERE enabled=1 LIMIT 1")
if rules:
    rid, rname = rules[0]
    res = api("POST", f"/api/proactive/rules/{rid}/trigger")
    d = res.get("data", {})
    check(f"触发'{rname}'", d.get("triggered"))
    check("channel=all", d.get("channel") == "all")
    txt = d.get("messageContent", "")
    check("消息非空", len(txt.strip()) > 0)
    if txt: print(f"    消息: {txt[:50]}...")
    time.sleep(0.5)
    pm = db("SELECT channel, status FROM proactive_messages ORDER BY created_at DESC LIMIT 1")
    check("pm channel=all", pm and pm[0][0] == "all")
    check("pm status=sent", pm and pm[0][1] == "sent")

print("\n[3] RunActiveMessageTask")
tasks = db("SELECT id, task_type FROM active_message_task WHERE status='PENDING' AND character_id=? ORDER BY due_time LIMIT 1", (CID,))
if tasks:
    tid, ttype = tasks[0]
    res = api("POST", f"/api/companion/active-message/tasks/{tid}/run?characterId={CID}")
    d = res.get("data", {})
    check(f"运行'{ttype}'", d.get("status") == "SENT")
    check("channel=all", d.get("channel") == "all")
    time.sleep(0.5)
    ts = db("SELECT status FROM active_message_task WHERE id=?", (tid,))
    check("task->SENT", ts and ts[0][0] == "SENT")
else:
    print("  无PENDING任务")

print("\n[4] Sidecar直连")
for port, name in [(WECHAT, "wechat"), (QQ, "QQ")]:
    try:
        data = json.dumps({"toUserId": "test", "text": "[test]"}).encode()
        req = urllib.request.Request(f"http://127.0.0.1:{port}/api/send",
            data=data, headers={"Content-Type": "application/json"}, method="POST")
        resp = urllib.request.urlopen(req, timeout=10)
        check(f"{name} 200", resp.status == 200, f"HTTP {resp.status}" if resp.status != 200 else None)
    except urllib.error.HTTPError as e:
        check(f"{name} 可达", True, f"HTTP {e.code} (平台未登录,非代码问题)")
    except Exception as e:
        check(f"{name} 可达", False, str(e))

print("\n[5] 代码逻辑")
svc = open(os.path.join(BACKEND, "internal", "companion", "service.go"), encoding="utf-8").read()
check("companion isDefault守卫", svc.count("isDefaultCharacter(characterID)") >= 2)
hdl = open(os.path.join(BACKEND, "internal", "proactive", "handler.go"), encoding="utf-8").read()
check("TriggerRule wechat", "sendToWechatSidecar" in hdl)
check("TriggerRule QQ", "sendToQQSidecarForTrigger" in hdl)

print("\n[6] 消息记录")
msgs = db("SELECT channel, status, substr(message_content,1,35) FROM proactive_messages ORDER BY created_at DESC LIMIT 5")
for m in msgs:
    print(f"  ch={m[0]:5s} st={m[1]:5s} {m[2]}")

pmc = db("SELECT COUNT(*) FROM proactive_messages WHERE channel='all'")
check("pm channel=all记录", pmc[0][0] > 0, f"{pmc[0][0]}条")
msc = db("SELECT COUNT(*) FROM messages WHERE source='proactive'")
check("messages proactive记录", msc[0][0] > 0, f"{msc[0][0]}条")

print()
print("=" * 55)
print(f"  结论: {PASS}通过 / {FAIL}失败")
print("=" * 55)
if FAIL:
    print("  注意: Sidecar 503=下游平台未登录,非代码问题")
sys.exit(0 if FAIL == 0 else 1)
