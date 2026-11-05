import json
import pathlib
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[2]
ENDPOINT = json.loads((ROOT / "AmitiaData/run/endpoint.json").read_text())
BASE = ENDPOINT["httpBaseURL"]
TOKEN = (ROOT / "AmitiaData/security/local-token").read_text().strip()


def request(method, path, body=None, expected=200, headers=None):
    data = None if body is None else (body if isinstance(body, bytes) else json.dumps(body).encode())
    merged = {"X-Amitia-Local-Token": TOKEN, "Content-Type": "application/json"}
    if headers:
        merged.update(headers)
    req = urllib.request.Request(BASE + path, data=data, method=method, headers=merged)
    try:
        response = urllib.request.urlopen(req, timeout=30)
    except urllib.error.HTTPError as error:
        response = error
    raw = response.read()
    try:
        payload = json.loads(raw.decode())
    except json.JSONDecodeError:
        payload = raw.decode(errors="replace")
    if response.status != expected:
        raise AssertionError(f"{method} {path}: expected {expected}, got {response.status}: {payload}")
    return payload


def data(payload):
    if isinstance(payload, dict) and "data" in payload and payload.get("code") in (None, 200):
        return payload["data"]
    return payload


def main():
    export_path = None
    mcp_id = None
    cleanup_backup = None
    try:
        storage = data(request("GET", "/api/storage/info"))
        assert storage["scope"] == "all-roles", storage
        assert storage["conversationCount"] >= 0 and storage["messageCount"] >= 0 and storage["memoryCount"] >= 0, storage
        assert storage["dbSizeBytes"] >= storage.get("kernelDbSizeBytes", 0), storage

        preview = data(request("POST", "/api/data-portability/exports/preview", {"scope": "all", "profile": "portable"}))
        assert preview["components"], preview
        exported = data(request("POST", "/api/data-portability/exports", {"scope": "all", "profile": "portable"}))
        assert exported["size"] > 0 and any(item["id"] == "memory.owned" for item in exported["components"]), exported
        export_path = ROOT / "AmitiaData/exports" / exported["file"]
        assert export_path.read_bytes()[:2] == b"PK", export_path

        backups = data(request("GET", "/api/data-portability/backups"))["backups"]
        assert all(not item["name"].startswith(".") for item in backups), backups
        assert all(item["name"].endswith(".amitia-backup") or item["name"].endswith(".amitia-backup.enc") for item in backups), backups

        profile_before = data(request("GET", "/api/temporal/profile"))
        invalid_profiles = [
            {"timezoneMode": "invalid_mode"},
            {"weekStart": 99},
            {"hemisphere": "sideways"},
            {"daypartConfigJson": "{broken"},
            {"quietHoursJson": '{"enabled":true,"start":"25:99","end":"07:00"}'},
        ]
        for invalid in invalid_profiles:
            failure = request("PUT", "/api/temporal/profile", invalid, expected=400)
            assert failure["msg"], failure
        profile_after = data(request("GET", "/api/temporal/profile"))
        for key in ["timezoneMode", "weekStart", "hemisphere", "daypartConfigJson", "quietHoursJson"]:
            assert profile_after[key] == profile_before[key], (key, profile_before[key], profile_after[key])

        invalid_anchor = {
            "scopeType": "space",
            "anchorType": "invalid",
            "title": "QA-INVALID",
            "timeKind": "annual_date",
            "localDate": "2026-02-31",
            "localTime": "24:00",
            "timezone": "Asia/Shanghai",
            "status": "invalid",
            "preWindowSeconds": -1,
        }
        request("POST", "/api/temporal/anchors", invalid_anchor, expected=400)
        request("DELETE", "/api/temporal/anchors/qa-missing-anchor", expected=404)
        limit_failure = request("GET", "/api/temporal/anchors?limit=abc", expected=400)
        assert "invalid syntax" in limit_failure["msg"], limit_failure
        request("GET", "/api/temporal/events?limit=0", expected=400)

        marker = uuid.uuid4().hex
        server_body = {
            "name": "QA-MCP-" + marker,
            "transport": "streamable_http",
            "endpoint": "https://" + marker + ".invalid/mcp",
            "authType": "none",
            "enabled": False,
        }
        created = request("POST", "/api/mcp/servers", server_body, expected=201)
        mcp_id = data(created)["id"]
        duplicate = request("POST", "/api/mcp/servers", server_body, expected=409)
        assert "MCP_SERVER_ALREADY_EXISTS" in duplicate["detail"] and "UNIQUE constraint failed" in duplicate["detail"], duplicate
        malformed = request("POST", "/api/mcp/servers", b'{"transport":', expected=400)
        assert "unexpected EOF" in malformed["detail"], malformed
        request("DELETE", f"/api/mcp/servers/{urllib.parse.quote(mcp_id)}")
        mcp_id = None

        roles = data(request("GET", "/api/device-mesh/v1/business/roles"))["roles"]
        assert roles, "no available role for cleanup regression"
        role = roles[0]["id"]
        scope_view = data(request("GET", "/api/device-mesh/v1/business/data?" + urllib.parse.urlencode({"kind": "working", "characterId": role})))
        scope = scope_view["executionScope"]
        cleanup_preview = data(request("POST", "/api/device-mesh/v1/business/chat-management/cleanup/preview", {
            "requestId": str(uuid.uuid4()),
            "characterId": role,
            "targetDeviceId": scope["targetDeviceId"],
            "expectedExecutionScope": scope,
            "olderThanDays": 36500,
            "includeMemories": False,
        }))
        assert "estimatedSize" in cleanup_preview and "estimatedSizeFormatted" in cleanup_preview, cleanup_preview
        cleanup = data(request("POST", "/api/device-mesh/v1/business/chat-management/cleanup/confirm", {
            "requestId": str(uuid.uuid4()),
            "characterId": role,
            "targetDeviceId": scope["targetDeviceId"],
            "expectedExecutionScope": scope,
            "previewId": cleanup_preview["previewId"],
            "confirmText": "确认清理",
        }))
        cleanup_backup = cleanup["backupPath"]
        assert pathlib.Path(cleanup_backup).is_file(), cleanup
        assert "cleanedFormatted" in cleanup and "backupId" in cleanup, cleanup
        backup_name = pathlib.Path(cleanup_backup).name
        listed = data(request("GET", "/api/data-portability/backups"))["backups"]
        assert any(item["name"] == backup_name for item in listed), (backup_name, listed)
        request("DELETE", "/api/data-portability/backups/" + urllib.parse.quote(backup_name))
        cleanup_backup = None

        print(json.dumps({
            "storage": {"scope": storage["scope"], "conversations": storage["conversationCount"], "messages": storage["messageCount"], "memories": storage["memoryCount"]},
            "export": {"file": exported["file"], "size": exported["size"], "components": len(exported["components"])},
            "temporalInvalidCases": len(invalid_profiles) + 4,
            "mcpDuplicate": duplicate["status"],
            "cleanup": {"conversationCount": cleanup["conversationCount"], "messageCount": cleanup["messageCount"], "cleanedFormatted": cleanup["cleanedFormatted"], "backupVerified": True},
        }, ensure_ascii=False))
    finally:
        if export_path and export_path.exists():
            export_path.unlink()
        if cleanup_backup and pathlib.Path(cleanup_backup).exists():
            pathlib.Path(cleanup_backup).unlink()
        if mcp_id:
            try:
                request("DELETE", f"/api/mcp/servers/{urllib.parse.quote(mcp_id)}")
            except Exception as error:
                print(f"cleanup MCP failed: {error}", file=sys.stderr)


if __name__ == "__main__":
    main()
