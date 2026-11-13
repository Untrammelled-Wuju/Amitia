#!/usr/bin/env python3
"""Owned chat end-to-end battery against a running Core.

Covers the real HTTP surface used by the desktop UI:
  POST /api/device-mesh/v1/business/messages      (non streaming and SSE)
  POST /api/device-mesh/v1/business/chat-management/search   (read back)
  GET  /api/device-mesh/v1/business/conversations

Every case records what the server actually answered and whether the data was
still readable afterwards. Any HTTP 5xx fails the run.

Usage:
  AMITIA_BASE_URL=http://127.0.0.1:PORT AMITIA_TOKEN=... \
  AMITIA_CHARACTER_ID=<role uuid> python3 scripts/test/qa-mesh-chat-e2e.py
"""

import concurrent.futures
import json
import os
import sys
import time
import urllib.error
import urllib.request

BASE = os.environ["AMITIA_BASE_URL"].rstrip("/")
TOKEN = os.environ["AMITIA_TOKEN"]
CHARACTER = os.environ["AMITIA_CHARACTER_ID"]
PREFIX = os.environ.get("AMITIA_RUN_PREFIX", "e2e")

results = []
server_errors = []


def record(name, passed, detail):
    results.append((name, passed, detail))
    print(("PASS " if passed else "FAIL ") + name + " :: " + detail)


def call(method, path, body=None, accept=None, timeout=120):
    data = None if body is None else json.dumps(body, ensure_ascii=False).encode()
    request = urllib.request.Request(BASE + path, data=data, method=method)
    request.add_header("X-Amitia-Local-Token", TOKEN)
    if data is not None:
        request.add_header("Content-Type", "application/json")
    if accept:
        request.add_header("Accept", accept)
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            raw = response.read()
            status = response.status
    except urllib.error.HTTPError as error:
        raw = error.read()
        status = error.code
    elapsed = time.time() - started
    if status >= 500:
        server_errors.append(f"{status} {method} {path} {raw[:200]!r}")
    try:
        payload = json.loads(raw.decode("utf-8"))
    except Exception:
        payload = {"_raw": raw[:400].decode("utf-8", "replace")}
    return status, elapsed, payload


def send(request_id, message, conversation_id="", accept=None, timeout=120):
    body = {"requestId": request_id, "characterId": CHARACTER, "message": message}
    if conversation_id:
        body["conversationId"] = conversation_id
    return call("POST", "/api/device-mesh/v1/business/messages", body, accept=accept, timeout=timeout)


SCOPE = {}


def search(keyword, page_size=128, conversation_id=""):
    return call(
        "POST",
        "/api/device-mesh/v1/business/chat-management/search",
        {
            "keyword": keyword,
            "page": 1,
            "pageSize": page_size,
            "characterId": CHARACTER,
            "conversationId": conversation_id,
            "expectedExecutionScope": SCOPE,
        },
    )


def search_total(keyword, conversation_id=""):
    status, _, payload = search(keyword, conversation_id=conversation_id)
    data = payload.get("data") or {}
    return status, data.get("total", 0), data.get("items") or []


def run_sse(request_id, message, conversation_id=""):
    """Consume the SSE variant and return the event types and assembled reply."""
    body = {"requestId": request_id, "characterId": CHARACTER, "message": message}
    if conversation_id:
        body["conversationId"] = conversation_id
    request = urllib.request.Request(
        BASE + "/api/device-mesh/v1/business/messages",
        data=json.dumps(body, ensure_ascii=False).encode(),
        method="POST",
    )
    request.add_header("X-Amitia-Local-Token", TOKEN)
    request.add_header("Content-Type", "application/json")
    request.add_header("Accept", "text/event-stream")
    events, text, status = [], "", 0
    started = time.time()
    with urllib.request.urlopen(request, timeout=120) as response:
        status = response.status
        for raw in response:
            line = raw.decode("utf-8", "replace").strip()
            if not line.startswith("data:"):
                continue
            try:
                event = json.loads(line[5:].strip())
            except Exception:
                continue
            events.append(event.get("type", "?"))
            data = event.get("data") or {}
            delta = event.get("text") or event.get("delta") or data.get("text")
            if isinstance(delta, str):
                text += delta
            if event.get("type") in ("completed", "failed", "interrupted"):
                break
    return status, time.time() - started, events, text


def main():
    run = f"{PREFIX}-{int(time.time())}"

    rid = run + "-basic"
    message = f"端到端基线 {run}"
    status, elapsed, payload = send(rid, message)
    data = payload.get("data") or {}
    record(
        "normal-turn",
        status == 200 and payload.get("code") == 200 and data.get("saved") is True and bool(data.get("reply")),
        f"status={status} {elapsed*1000:.0f}ms saved={data.get('saved')} reply={str(data.get('reply'))[:24]!r}",
    )
    conversation = data.get("conversationId") or ""
    first_turn = data.get("turnId") or ""
    SCOPE.update(data.get("executionScope") or {})

    reply = data.get("reply") or ""
    user_status, total, items = search_total(message, conversation)
    reply_status, _, replies = search_total(reply, conversation)
    user_hits = [item for item in items if item.get("role") == "user" and item.get("content") == message]
    reply_hits = [item for item in replies if item.get("role") == "assistant" and item.get("content") == reply]
    record(
        "readback-search",
        bool(conversation and reply) and user_status == reply_status == 200 and len(user_hits) == len(reply_hits) == 1,
        f"user_status={user_status} user_hits={len(user_hits)} reply_status={reply_status} reply_hits={len(reply_hits)}",
    )

    status, elapsed, payload = send(rid, message, conversation)
    data = payload.get("data") or {}
    after_total = search_total(message, conversation)[1]
    record(
        "idempotent-resend",
        status == 200 and data.get("turnId") == first_turn and after_total == total,
        f"status={status} turn={data.get('turnId')} first={first_turn} total {total}->{after_total}",
    )

    status, elapsed, payload = send(rid, "被篡改的重复请求", conversation)
    tampered = search_total("被篡改的重复请求")[1]
    record(
        "idempotent-mismatch",
        status in (200, 409, 422) and tampered == 0,
        f"status={status} msg={(payload.get('message') or payload.get('msg') or '')[:40]!r} tampered_rows={tampered}",
    )

    try:
        status, elapsed, events, text = run_sse(run + "-stream", "流式回复测试", conversation)
        stream_ok = status == 200 and bool(events) and (text != "" or "completed" in events)
        detail = f"status={status} {elapsed*1000:.0f}ms events={events[:8]} text={text[:24]!r}"
    except Exception as error:  # noqa: BLE001 - report the transport failure verbatim
        stream_ok, detail = False, f"transport error: {error!r}"
    record("streaming-turn", stream_ok, detail)

    status, _, payload = send(run + "-empty", "")
    record("empty-message", 400 <= status < 500, f"status={status} msg={str(payload.get('message') or payload.get('msg'))[:40]!r}")

    status, _, payload = send(run + "-blank", "   ")
    record("blank-message", 400 <= status < 500, f"status={status} msg={str(payload.get('message') or payload.get('msg'))[:40]!r}")

    status, _, payload = send(run + "-emoji", "😀🐱 表情与多脚本测试 ÆØÅ مرحبا")
    data = payload.get("data") or {}
    emoji_rows = search_total("表情与多脚本测试")[1]
    record(
        "emoji-message",
        status == 200 and bool(data.get("reply")) and emoji_rows >= 1,
        f"status={status} reply={str(data.get('reply'))[:20]!r} rows={emoji_rows}",
    )

    status, _, payload = send(run + "-huge", "长" * ((4 << 20) // 3 + 32), timeout=180)
    record("oversized-body", 400 <= status < 500, f"status={status} msg={str(payload.get('message') or payload.get('msg'))[:48]!r}")

    status, elapsed, payload = send(run + "-large", "长" * 200000, conversation, timeout=300)
    record(
        "overlong-message",
        400 <= status < 500,
        f"status={status} {elapsed:.1f}s code={payload.get('code')} msg={str(payload.get('message') or payload.get('msg'))[:32]!r}",
    )

    parallel_text = f"并发消息{run}"

    def parallel(index):
        return send(f"{run}-par-{index}", f"{parallel_text}-{index}", conversation, timeout=300)

    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        parallel_outcomes = list(pool.map(parallel, range(6)))
    ok = [o for o in parallel_outcomes if o[0] == 200]
    turns = {(o[2].get("data") or {}).get("turnId") for o in ok}
    persisted = sum(search_total(f"{parallel_text}-{i}")[1] for i in range(6))
    record(
        "parallel-turns",
        len(ok) == 6 and len(turns) == 6 and persisted >= 6,
        f"ok={len(ok)}/6 distinct_turns={len(turns)} persisted_rows={persisted}",
    )

    duplicate_text = f"重复并发请求-{run}"

    def duplicate(_):
        return send(run + "-dup", duplicate_text, conversation, timeout=300)

    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        duplicates = list(pool.map(duplicate, range(4)))
    dup_ok = [o for o in duplicates if o[0] == 200]
    dup_turns = {(o[2].get("data") or {}).get("turnId") for o in dup_ok}
    dup_rows = search_total(duplicate_text)[1]
    dup_status = sorted(o[0] for o in duplicates)
    record(
        "duplicate-concurrent",
        len(dup_ok) == 1 and len(dup_turns) == 1 and dup_rows == 1 and all(s in (200, 409) for s in dup_status),
        f"statuses={dup_status} ok={len(dup_ok)}/4 turns={len(dup_turns)} rows={dup_rows}",
    )

    status, _, payload = call("GET", f"/api/device-mesh/v1/business/conversations?characterId={CHARACTER}&pageSize=50")
    listing = payload.get("data") or {}
    entries = listing.get("historicalConversations") or listing.get("items")
    found = isinstance(entries, list) and any(
        isinstance(item, dict) and item.get("conversationId") == conversation for item in entries
    )
    record("conversation-list", status == 200 and found, f"status={status} listed={found}")

    rt_status, _, rt_payload = call(
        "GET", f"/api/device-mesh/v1/business/relationship-time?characterId={CHARACTER}"
    )
    rt = rt_payload.get("data") or {}
    rt_state = rt.get("state") or {}
    rt_ok = (
        rt_status == 200
        and rt_state.get("firstInteractionAt", "0001-01-01").startswith("0001") is False
        and rt_state.get("globalLastCommittedAt", "0001-01-01").startswith("0001") is False
    )
    record(
        "relationship-state-read",
        rt_ok,
        f"status={rt_status} space={str(rt_state.get('spaceId'))[:8]} first={rt_state.get('firstInteractionAt')} last={rt_state.get('globalLastCommittedAt')}",
    )

    settings = dict(rt.get("settings") or {})
    settings["enabled"] = False
    put_status, _, put_payload = call(
        "PUT",
        f"/api/device-mesh/v1/business/relationship-time?characterId={CHARACTER}",
        {"settings": settings, "expectedExecutionScope": rt.get("executionScope")},
    )
    reread = call("GET", f"/api/device-mesh/v1/business/relationship-time?characterId={CHARACTER}")[2].get("data") or {}
    disabled = (reread.get("settings") or {}).get("enabled") is False
    record(
        "relationship-settings-write",
        put_status == 200 and disabled,
        f"status={put_status} enabled_after={((reread.get('settings') or {}).get('enabled'))} msg={str(put_payload.get('message'))[:32]!r}",
    )

    status, _, _ = send(run + "-after-disable", "关闭关系时间后的回复", conversation)
    record("turn-after-settings", status == 200, f"status={status}")

    fresh = call("GET", f"/api/device-mesh/v1/business/relationship-time?characterId={CHARACTER}")[2].get("data") or {}
    stale = dict(fresh.get("executionScope") or {})
    stale["roleId"] = "stale-role"
    stale_status, _, _ = call(
        "PUT",
        f"/api/device-mesh/v1/business/relationship-time?characterId={CHARACTER}",
        {"settings": fresh.get("settings"), "expectedExecutionScope": stale},
    )
    record("relationship-settings-stale-scope", stale_status == 409, f"status={stale_status}")

    restore = dict(fresh.get("settings") or {})
    restore["enabled"] = True
    restore_status, _, restore_payload = call(
        "PUT",
        f"/api/device-mesh/v1/business/relationship-time?characterId={CHARACTER}",
        {"settings": restore, "expectedExecutionScope": fresh.get("executionScope")},
    )
    final_enabled = (
        (call("GET", f"/api/device-mesh/v1/business/relationship-time?characterId={CHARACTER}")[2].get("data") or {}).get("settings") or {}
    ).get("enabled")
    record(
        "relationship-settings-restore",
        restore_status == 200 and final_enabled is True,
        f"status={restore_status} enabled={final_enabled} msg={str(restore_payload.get('message'))[:32]!r}",
    )

    print()
    print(f"cases={len(results)} failed={sum(1 for _, ok, _ in results if not ok)} server_errors={len(server_errors)}")
    for error in server_errors:
        print("5xx :: " + error)
    print("conversation=" + conversation)
    return 0 if all(ok for _, ok, _ in results) and not server_errors else 1


if __name__ == "__main__":
    sys.exit(main())
