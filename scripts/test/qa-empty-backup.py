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
    device_agent = sys.argv[2:] == ["--device-agent"]
    if not (root / ".qa-restore-isolated").is_file():
        raise ValueError("必须使用带 .qa-restore-isolated 标记的独立数据目录")
    endpoint = json.loads((root / "run" / "endpoint.json").read_text())["httpBaseURL"]
    if urllib.parse.urlsplit(endpoint).hostname != "127.0.0.1":
        raise ValueError("必须使用环回 Core")
    token = (root / "security" / "local-token").read_text().strip()

    def call(method, route, body=None, expected=200, headers=None):
        request = urllib.request.Request(
            endpoint + route,
            None if body is None else json.dumps(body).encode(),
            {"X-Amitia-Local-Token": token, "Content-Type": "application/json", **(headers or {})},
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
    if device_agent:
        assert call("GET", "/api/characters") == []
        owner = json.loads((root / "device-mesh/identity.json").read_text())["deviceId"]
        initial = call("POST", prefix + "/backups", password)
        assert initial["encrypted"] and initial["size"] > 0
        call("DELETE", prefix + "/backups/" + urllib.parse.quote(initial["name"]))
    else:
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
        if not device_agent:
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
    call("POST", prefix + "/restores/stale-qa-operation", {}, expected=422)
    assert len(list(staged.glob("*.amitia-backup"))) == 1
    result = call("POST", prefix + "/restores/" + prepared["operationId"], {})
    assert result["status"] == "completed"
    assert not list(staged.glob("*.amitia-backup"))
    assert not list((root / "imports" / ".uploads").glob("*.amitia-backup"))
    record_restored = False
    if device_agent:
        def role_headers():
            authority = call("GET", "/api/characters/authority")["roleAuthority"]
            return {"X-Amitia-Role-Authority": authority}

        original_name = "QA role snapshot"
        role = call("POST", "/api/characters", {"name": original_name}, headers=role_headers())
        role_route = "/api/characters/" + urllib.parse.quote(role["id"])
        backup = call("POST", prefix + "/backups", password)
        assert backup["encrypted"]
        archive = root / "backups" / backup["name"]
        assert original_name.encode() not in archive.read_bytes()
        changed = call("PUT", role_route, {"name": "QA changed after backup"}, headers=role_headers())
        assert changed["name"] == "QA changed after backup"
        restore_preview = prefix + "/backups/" + urllib.parse.quote(backup["name"]) + "/restore/preview"
        prepared = call("POST", restore_preview, password)
        components = {item["id"]: item["itemCount"] for item in prepared["manifest"]["components"]}
        assert components["character.records.v1"] == 1
        result = call("POST", prefix + "/restores/" + prepared["operationId"], {})
        assert result["status"] == "completed"
        assert call("GET", role_route)["name"] == original_name
        assert not list(staged.glob("*.amitia-backup"))
        call("DELETE", role_route, headers=role_headers())
        call("DELETE", prefix + "/backups/" + urllib.parse.quote(backup["name"]))
        record_restored = True
    print(json.dumps({"emptyRestoreCompleted": True, "historicalMemoryRejected": True, "staleRestoreRejected": True,
                      "characterRecordRestored": record_restored, "plaintextAfterRestore": []}))


if __name__ == "__main__":
    main()
