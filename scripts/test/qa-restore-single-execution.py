import argparse
import concurrent.futures
import hashlib
import json
import pathlib
import threading
import urllib.error
import urllib.parse
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    parser.add_argument("--verify-after-restart", action="store_true")
    args = parser.parse_args()
    root = args.data_dir.resolve()
    if not (root / ".qa-restore-isolated").is_file():
        parser.error("目标必须是带 .qa-restore-isolated 标记的隔离测试目录")
    endpoint = json.loads((root / "run" / "endpoint.json").read_text())
    base = endpoint["httpBaseURL"]
    if urllib.parse.urlsplit(base).hostname not in ("127.0.0.1", "::1"):
        parser.error("仅允许环回 Core")
    token = (root / "security" / "local-token").read_text().strip()
    receipt = root / ".qa-restore-receipt.json"

    def call(method, path, body=None):
        request = urllib.request.Request(
            base + path,
            None if body is None else json.dumps(body).encode(),
            {"X-Amitia-Local-Token": token, "Content-Type": "application/json"},
            method=method,
        )
        try:
            response = urllib.request.urlopen(request, timeout=45)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            payload = json.load(response)
            return payload.get("code", response.status), payload

    def success(method, path, body=None):
        code, payload = call(method, path, body)
        assert code == 200, (method, path, code, payload)
        return payload["data"]

    if args.verify_after_restart:
        saved = json.loads(receipt.read_text())
        assert endpoint["pid"] != saved["pid"], endpoint["pid"]
        role = success("GET", saved["rolePath"])
        assert role["name"] == saved["name"], role
        print(json.dumps({"persisted": True, "pid": endpoint["pid"], "name": role["name"]}, ensure_ascii=False))
        return

    prefix = "/api/data-portability"
    original = "QA 备份前 " + uuid.uuid4().hex[:8]
    role = success("POST", "/api/characters", {"name": original})
    role_path = "/api/characters/" + role["id"]
    password = {"passphrase": "isolated restore regression"}
    backup = success("POST", prefix + "/backups", password)
    archive = root / "backups" / backup["name"]
    assert backup["encrypted"] and hashlib.sha256(archive.read_bytes()).hexdigest() == backup["checksum"]
    preview = prefix + "/backups/" + urllib.parse.quote(backup["name"]) + "/restore/preview"
    assert call("POST", preview, {"passphrase": "incorrect"})[0] == 401
    cancelled = success("POST", preview, password)
    success("DELETE", prefix + "/restores/pending")
    assert call("POST", prefix + "/restores/" + cancelled["operationId"], {})[0] >= 400
    ticket = success("POST", preview, password)
    assert call("POST", prefix + "/restores/invalid-operation", {})[0] >= 400
    assert success("PUT", role_path, {"name": original + " 已修改"})["name"] == original + " 已修改"
    stage = root / "imports" / ".restore" / (ticket["backupId"] + ".amitia-backup")
    stage.write_bytes(b"QA tampered staged archive")
    code, rejected = call("POST", prefix + "/restores/" + ticket["operationId"], {})
    assert code == 422 and "RESTORE_SQLITE_INTEGRITY_FAILED" in rejected["msg"]
    assert success("GET", role_path)["name"] == original + " 已修改"
    replacement = success("POST", preview, password)
    assert replacement["operationId"] != ticket["operationId"]
    restore = prefix + "/restores/" + replacement["operationId"]
    previous_files = set((root / "backups").iterdir())
    barrier = threading.Barrier(6)

    def submit(_):
        barrier.wait(timeout=5)
        code, result = call("POST", restore, {})
        detail = result.get("data") or {}
        return {"code": code, "status": detail.get("status"), "message": result.get("msg")}

    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        results = list(pool.map(submit, range(6)))
    restored = success("GET", role_path)
    pre_restore_count = len(set((root / "backups").iterdir()) - previous_files)
    print(json.dumps({"results": results, "preRestoreBackups": pre_restore_count, "name": restored["name"]}, ensure_ascii=False))
    completed = [item for item in results if item["code"] == 200]
    assert len(completed) == 1 and completed[0]["status"] == "completed", results
    assert all(item["code"] in (200, 409, 422) for item in results), results
    assert all(item["status"] in (None, "completed") for item in results), results
    assert pre_restore_count == 1 and restored["name"] == original
    assert call("POST", restore, {})[0] >= 400
    receipt.write_text(json.dumps({"rolePath": role_path, "name": original, "pid": endpoint["pid"]}))


if __name__ == "__main__":
    main()
