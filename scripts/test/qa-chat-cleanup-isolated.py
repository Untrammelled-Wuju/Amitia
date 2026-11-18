import argparse
import json
import pathlib
import sqlite3
import urllib.error
import urllib.parse
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    parser.add_argument("qa_role")
    args = parser.parse_args()
    root = args.data_dir.resolve()
    if not (root / ".qa-restore-isolated").is_file() or not args.qa_role.startswith("QA "):
        parser.error("需要隔离目录和以 QA 开头的测试角色")
    base = json.loads((root / "run/endpoint.json").read_text())["httpBaseURL"]
    if urllib.parse.urlsplit(base).hostname != "127.0.0.1":
        parser.error("只允许访问隔离环回 Core")
    token = (root / "security/local-token").read_text().strip()
    prefix = "/api/device-mesh/v1/business"

    def call(path, body=None):
        if body is not None:
            body = json.dumps(dict(body, requestId=str(uuid.uuid4()))).encode()
        request = urllib.request.Request(
            base + prefix + path, data=body,
            headers={"Content-Type": "application/json", "X-Amitia-Local-Token": token},
        )
        try:
            response = urllib.request.urlopen(request, timeout=25)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            payload = json.load(response)
            return response.status, payload.get("data") or payload

    status, catalog = call("/roles")
    assert status == 200, catalog
    role, = (role for role in catalog["roles"] if role["name"] == args.qa_role)
    scope = dict(catalog["executionScope"], roleId=role["id"], roleRevision=role["revision"], memoryRoleId=role["id"])
    common = {"characterId": role["id"], "targetDeviceId": scope["targetDeviceId"], "expectedExecutionScope": scope}
    preview_path = "/chat-management/cleanup/preview"
    for filters, message in (
        ({}, "请至少选择一个清理条件"),
        ({"beforeDate": "2020-01-01"}, "当前条件没有匹配到任何会话"),
    ):
        status, refusal = call(preview_path, dict(common, **filters))
        assert status == 409 and refusal["message"] == message, (status, refusal)

    status, preview = call(preview_path, dict(common, beforeDate="2100-01-01"))
    assert status == 200 and preview["conversationCount"] > 0 and preview["messageCount"] > 0, preview
    scope = preview["executionScope"]
    confirm = dict(common, previewId=preview["previewId"], expectedExecutionScope=scope)
    confirm_path = "/chat-management/cleanup/confirm"

    def saved_state():
        with sqlite3.connect((root / "extensions-v2/kernel.db").as_uri() + "?mode=ro", uri=True) as db:
            chats = dict(db.execute(
                "SELECT kind, count(*) FROM kernel_device_owned_resources WHERE deleted=0 AND kind IN ('message','conversation') AND role_id=? AND owner_id=? GROUP BY kind",
                (scope["roleId"], scope["resourceOwnerId"]),
            ))
            memories = db.execute(
                "SELECT resource_id,revision FROM kernel_device_owned_resources WHERE deleted=0 AND kind='memory' ORDER BY resource_id"
            ).fetchall()
        return chats, memories

    before = saved_state()
    assert before[0] == {"conversation": preview["conversationCount"], "message": preview["messageCount"]}, before
    assert before[1], "测试前须已有可供核对的记忆"
    status, invalid = call(confirm_path, dict(confirm, confirmText="错误确认"))
    assert status == 409 and invalid["message"] == "确认文本不匹配" and saved_state() == before, invalid
    archives = set((root / "backups").glob("*.amitia-backup*"))
    status, result = call(confirm_path, dict(confirm, confirmText="确认清理"))
    added = set((root / "backups").glob("*.amitia-backup*")) - archives
    after = saved_state()
    assert status == 200 and result["conversationCount"] == preview["conversationCount"], result
    assert len(added) == 1 and pathlib.Path(result["backupPath"]) in added, result
    assert after == ({}, before[1]), after
    status, replay = call(confirm_path, dict(confirm, confirmText="确认清理"))
    assert status == 409 and "清理预览不存在" in replay["message"] and saved_state() == after, replay
    print(json.dumps({"conversations": preview["conversationCount"], "messages": preview["messageCount"],
                      "retainedMemories": len(before[1]), "archives": len(added)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
