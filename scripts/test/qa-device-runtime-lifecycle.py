import concurrent.futures
import json
import os
import pathlib
import sqlite3
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


def main():
    binary = pathlib.Path(sys.argv[1]).resolve(strict=True)
    project = pathlib.Path(__file__).resolve().parents[2]
    with tempfile.TemporaryDirectory(prefix="amitia-device-qa-") as directory:
        root = pathlib.Path(directory)
        workspace = root / "workspace"
        workspace.mkdir()
        source_file = workspace / "hello.txt"
        source_file.write_text("isolated QA payload")
        outside = root / "outside.txt"
        outside.write_text("must never be read through the workspace")
        (workspace / "outside-link").symlink_to(outside)
        env = {
            "PATH": os.environ["PATH"],
            "AMITIA_RUNTIME_ROOT": str(root),
            "AMITIA_CONFIG_DIR": str(root / "config"),
            "AMITIA_DATA_DIR": str(root),
            "AMITIA_WORKSPACE_DIR": str(workspace),
            "AMITIA_SERVER_HOST": "127.0.0.1",
        }
        config = root / "config"
        config.mkdir()
        (config / "config.yml").write_bytes((project / "backend/config/config.yml").read_bytes())
        client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        base, token = "", ""

        def call(method, route, body=None, expected=200, code=None, authenticate=True, headers=None):
            request_headers = {"Content-Type": "application/json", **(headers or {})}
            if authenticate:
                request_headers["X-Amitia-Local-Token"] = token
            request = urllib.request.Request(
                base + route, None if body is None else json.dumps(body).encode(),
                request_headers, method=method,
            )
            try:
                response = client.open(request, timeout=5)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                payload = json.load(response)
                assert response.status == expected, (route, response.status, payload)
            if code is not None:
                assert payload["code"] == code, (route, payload)
                return payload["data"]
            return payload

        def start(log):
            return subprocess.Popen([binary, "--runtime-profile", "device-agent"], cwd=root, env=env, stdout=log, stderr=log)

        def ready(process):
            nonlocal base, token
            deadline = time.monotonic() + 40
            while time.monotonic() < deadline and process.poll() is None:
                try:
                    endpoint = json.loads((root / "run/endpoint.json").read_text())
                    if endpoint["pid"] == process.pid:
                        url = urllib.parse.urlsplit(endpoint["httpBaseURL"])
                        assert url.hostname == "127.0.0.1" and url.port != 3000
                        base = url.geturl()
                        token = (root / "security/local-token").read_text().strip()
                        if call("GET", "/readyz", authenticate=False)["data"]["status"] == "ready":
                            return
                except (FileNotFoundError, urllib.error.URLError, TimeoutError, json.JSONDecodeError, AssertionError):
                    pass
                time.sleep(0.1)
            raise RuntimeError(f"Device agent failed to become ready: {process.poll()}")

        def role_headers():
            authority = call("GET", "/api/characters/authority", code=200)
            return {"X-Amitia-Role-Authority": authority["roleAuthority"]}

        def read_workspace(uri, status=200):
            return call("GET", "/api/workspaces/read?" + urllib.parse.urlencode({"uri": uri}), expected=status)

        with (root / "runtime.log").open("wb") as log:
            process = start(log)
            try:
                ready(process)
                call("GET", "/api/extensions/pet/plugins", code=401, authenticate=False)
                for route, key in (("/api/extensions/pet/plugins", "plugins"), ("/api/extensions/desktop-pet/plugins", "plugins"), ("/api/extensions/kernel/agent-skills", "items")):
                    for page in (2**63 - 1, 2**62 + 1):
                        path = f"{route}?page={page}&pageSize=2"
                        data = call("GET", path, code=200) if key == "plugins" else call("GET", path)
                        assert data["page"] == page and data[key] == [], (path, data)
                print("PASS authorization and large pages across three real HTTP routes")

                call("POST", "/api/characters", {"name": "reject"}, expected=409,
                     headers={"X-Amitia-Role-Authority": "a" * 64})
                assert call("GET", "/api/characters", code=200) == []
                role = call("POST", "/api/characters", {"name": "QA role"}, code=200, headers=role_headers())
                route = "/api/characters/" + role["id"]
                changed = call("PUT", route, {"name": "QA updated"}, code=200, headers=role_headers())
                assert changed["name"] == "QA updated"
                assert call("GET", route, code=200)["name"] == "QA updated"

                def register_workspace(_):
                    return call("POST", "/api/workspaces/local",
                                {"localRoot": str(workspace), "name": "QA", "readOnly": True}, expected=201)

                with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
                    mounts = list(pool.map(register_workspace, range(8)))
                assert len({row["id"] for row in mounts}) == 1, mounts
                assert len(call("GET", "/api/workspaces")) == 1
                mount = mounts[0]
                uri = mount["rootUri"]
                assert read_workspace(uri + "hello.txt")["content"] == "isolated QA payload"
                assert "symlink" in read_workspace(uri + "outside-link", 400)["error"]
                assert "traversal" in read_workspace(uri + "../outside.txt", 400)["error"]
                print("PASS role update, 8 concurrent workspace registrations, content and traversal denial")

                process.kill()
                assert process.wait(timeout=10) < 0
                try:
                    call("GET", "/readyz", authenticate=False)
                    raise AssertionError("Killed process still responds")
                except urllib.error.URLError:
                    pass
                process = start(log)
                ready(process)
                assert call("GET", route, code=200)["name"] == "QA updated"
                assert read_workspace(uri + "hello.txt")["content"] == "isolated QA payload"
                with sqlite3.connect(root / "app.db") as db:
                    assert db.execute("PRAGMA quick_check").fetchone() == ("ok",)
                    assert db.execute("SELECT name FROM characters WHERE id=?", (role["id"],)).fetchone() == ("QA updated",)
                call("DELETE", route, code=200, headers=role_headers())
                assert all(item["id"] != role["id"] for item in call("GET", "/api/characters", code=200))
                mount_route = "/api/workspaces/" + mount["id"]
                assert call("DELETE", mount_route)["deleted"]
                assert call("GET", "/api/workspaces") == []
                assert source_file.read_text() == "isolated QA payload"
                print("PASS SIGKILL, service restart, persisted role and workspace, SQLite check and cleanup")
            finally:
                if process.poll() is None:
                    process.terminate()
                    process.wait(timeout=15)


if __name__ == "__main__":
    main()
