import argparse
import json
import pathlib
import sqlite3
import urllib.parse
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    parser.add_argument("title")
    parser.add_argument("original_error")
    args = parser.parse_args()
    root = args.data_dir.resolve()
    assert args.original_error.strip(), "需要实际捕获的原始通知错误"
    assert "artifacts" in root.parts and any(part.startswith("qa-") for part in root.parts)
    endpoint = json.loads((root / "run/endpoint.json").read_text())
    base = endpoint["httpBaseURL"]
    assert urllib.parse.urlsplit(base).hostname == "127.0.0.1", endpoint
    token = (root / "security/local-token").read_text().strip()

    def get(path, **query):
        suffix = "?" + urllib.parse.urlencode(query) if query else ""
        request = urllib.request.Request(base + path + suffix, headers={"X-Amitia-Local-Token": token})
        with urllib.request.urlopen(request, timeout=10) as response:
            result = json.load(response)
        assert result["code"] == 200, result
        return result["data"]

    tasks = [item for item in get("/api/reminders") if item["title"] == args.title]
    assert len(tasks) == 1, tasks
    task = tasks[0]
    history = get("/api/reminders/trigger-history", id=task["id"])
    failed = [
        (event, target)
        for event in history["items"]
        for target in event.get("result", {}).get("targets", [])
        if target["kind"] == "notification" and target["state"] == "display_failed"
    ]
    assert failed, history
    with sqlite3.connect((root / "app.db").as_uri() + "?mode=ro", uri=True) as database:
        for event, target in failed:
            result = event["result"]
            assert event["state"] == "partial" and result["messageSaved"], event
            assert result["conversationId"] == task["conversationId"], event
            records = []
            for path in (root / "conversations").rglob("*.jsonl"):
                with path.open(encoding="utf-8") as stream:
                    for line in stream:
                        record = json.loads(line)
                        if record.get("type") == "message_created" and record.get("messageId") == result["messageId"]:
                            records.append(record)
            assert len(records) == 1 and records[0]["conversationId"] == task["conversationId"], records
            persisted = database.execute(
                "SELECT status, last_error FROM notification_outbox WHERE id = ?",
                (target["id"],),
            ).fetchone()
            assert persisted is not None and persisted[0] == "display_failed", persisted
            print(json.dumps({
                "trigger": event["triggerId"],
                "conversationEvents": len(records),
                "historyError": target.get("error"),
                "outboxError": persisted[1],
            }, ensure_ascii=False))
            assert args.original_error in (target.get("error") or ""), target
            assert args.original_error in persisted[1], persisted


if __name__ == "__main__":
    main()
