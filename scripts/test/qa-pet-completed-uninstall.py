import argparse
import json
import pathlib
import sqlite3
import urllib.parse
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    parser.add_argument("baseline_db", type=pathlib.Path)
    parser.add_argument("operation_id")
    args = parser.parse_args()
    root = args.data_dir.resolve()
    assert (root / ".qa-pet-isolated").is_file(), root
    base = json.loads((root / "run/endpoint.json").read_text())["httpBaseURL"]
    assert urllib.parse.urlsplit(base).hostname == "127.0.0.1", base
    token = (root / "security/local-token").read_text().strip()

    def call(path, device=None):
        headers = {"X-Amitia-Local-Token": token}
        if device:
            headers["X-Amitia-Device-ID"] = device
        request = urllib.request.Request(base + path, headers=headers)
        with urllib.request.urlopen(request, timeout=10) as response:
            assert response.status == 200, (path, response.status)
            return json.load(response)

    with sqlite3.connect(args.baseline_db.resolve().as_uri() + "?mode=ro", uri=True) as before, \
         sqlite3.connect((root / "app.db").as_uri() + "?mode=ro", uri=True) as after:
        for db in (before, after):
            db.row_factory = sqlite3.Row
        operation_query = "SELECT * FROM desktop_pet_installation_operations WHERE id = ?"
        original = before.execute(operation_query, (args.operation_id,)).fetchone()
        assert original and (original["operation_type"], original["status"], original["stage"]) == ("uninstall", "completed", "completed")
        assert dict(after.execute(operation_query, (args.operation_id,)).fetchone()) == dict(original)
        installation_id = original["installation_id"]
        install_query = "SELECT status, lifecycle_state, is_active, install_storage_key FROM desktop_pet_installations WHERE id = ?"
        previous = before.execute(install_query, (installation_id,)).fetchone()
        current = after.execute(install_query, (installation_id,)).fetchone()
        assert tuple(previous)[:3] == ("uninstalling", "uninstalling", 0), tuple(previous)
        assert tuple(current)[:3] == ("uninstalled", "uninstalled", 0), tuple(current)
        trash_query = "SELECT storage_key, status FROM desktop_pet_installation_trash_entries WHERE operation_id = ? AND installation_id = ?"
        keys = (args.operation_id, installation_id)
        assert not before.execute(trash_query, keys).fetchall()
        trash = after.execute(trash_query, keys).fetchall()
        assert len(trash) == 1 and trash[0]["status"] == "retained", trash
        source = root / "desktop-pets/installations" / previous["install_storage_key"]
        destination = root / "desktop-pets/storage-trash" / trash[0]["storage_key"]
        assert not source.exists() and (destination / "manifest.json").is_file(), (source, destination)
        siblings_query = "SELECT id, status, lifecycle_state, is_active FROM desktop_pet_installations WHERE id <> ? ORDER BY id"
        siblings = {row["id"]: tuple(row) for row in after.execute(siblings_query, (installation_id,))}
        for row in before.execute(siblings_query, (installation_id,)):
            assert siblings.get(row["id"]) == tuple(row), dict(row)
        assert after.execute("PRAGMA quick_check").fetchone()[0] == "ok"

    identity = call("/internal/device-mesh/identity")["deviceId"]
    response = call("/api/desktop-pets/installations", identity)
    data = response.get("data", response)
    items = data if isinstance(data, list) else data.get("installations", data.get("items"))
    assert isinstance(items, list), list(data)
    matches = [item for item in items if item.get("id") == installation_id or item.get("installationId") == installation_id]
    assert len(matches) == 1 and matches[0]["status"] == "uninstalled", matches
    print(json.dumps({"installation": installation_id, "status": "uninstalled", "operation": args.operation_id,
                      "operationUnchanged": True, "retainedTrash": len(trash), "sqliteQuickCheck": "ok"}, ensure_ascii=False))


if __name__ == "__main__":
    main()
