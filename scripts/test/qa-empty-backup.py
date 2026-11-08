import json
import pathlib
import sqlite3
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid


def main():
    root = pathlib.Path(sys.argv[1]).resolve()
    if not (root / ".qa-restore-isolated").is_file():
        raise ValueError("必须使用带 .qa-restore-isolated 标记的独立数据目录")
    endpoint = json.loads((root / "run" / "endpoint.json").read_text())["httpBaseURL"]
    if urllib.parse.urlsplit(endpoint).hostname != "127.0.0.1":
        raise ValueError("必须使用环回 Core")
    token = (root / "security" / "local-token").read_text().strip()

    def call(method, route, body=None, expected=200):
        request = urllib.request.Request(
            endpoint + route,
            None if body is None else json.dumps(body).encode(),
            {"X-Amitia-Local-Token": token, "Content-Type": "application/json"},
            method=method,
        )
        try:
            response = urllib.request.urlopen(request, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            payload = json.load(response)
            code = payload.get("code", response.status)
        assert code == expected, (method, route, payload)
        return payload.get("data", payload)

    prefix = "/api/data-portability"
    password = {"passphrase": "isolated empty backup regression"}
    assert not call("GET", "/api/device-mesh/v1/business/roles")["roles"]
    assert call("GET", "/api/storage/info")["memoryCount"] == 0
    owner = call("GET", "/api/device-mesh/v1/coordination/me")["policy"]["deviceId"]
    memory_id = "qa-historical-" + uuid.uuid4().hex
    with sqlite3.connect(root / "extensions-v2" / "kernel.db", timeout=5) as db:
        db.execute(
            "INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,deleted,body,updated_at) VALUES(?, 'memory', ?, 'qa-retired-role', 1, 0, '{}', '2026-11-06T00:00:00Z')",
            (owner, memory_id),
        )
    try:
        assert call("GET", "/api/storage/info")["memoryCount"] == 1
        call("POST", prefix + "/backups", password, expected=422)
    finally:
        with db:
            db.execute(
                "DELETE FROM kernel_device_owned_resources WHERE owner_id=? AND resource_id=?",
                (owner, memory_id),
            )
        db.close()

    staged = root / "imports" / ".restore"
    for _ in range(2):
        backup = call("POST", prefix + "/backups", password)
        preview = prefix + "/backups/" + urllib.parse.quote(backup["name"]) + "/restore/preview"
        existing = set(staged.glob("*.amitia-backup"))
        call("POST", preview, {"passphrase": "incorrect"}, expected=401)
        assert set(staged.glob("*.amitia-backup")) == existing
        prepared = call("POST", preview, password)
        components = {item["id"]: item["itemCount"] for item in prepared["manifest"]["components"]}
        assert components["character.records.v1"] == 0
        assert len(list(staged.glob("*.amitia-backup"))) == 1

    call("DELETE", prefix + "/restores/pending")
    assert not list(staged.glob("*.amitia-backup"))
    prepared = call("POST", preview, password)
    result = call("POST", prefix + "/restores/" + prepared["operationId"], {})
    assert result["status"] == "completed"
    assert not list(staged.glob("*.amitia-backup"))
    assert not list((root / "imports" / ".uploads").glob("*.amitia-backup"))
    print(json.dumps({"emptyRestoreCompleted": True, "historicalMemoryRejected": True, "plaintextAfterRestore": []}))


if __name__ == "__main__":
    main()
