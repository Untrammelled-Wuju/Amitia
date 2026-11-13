import argparse
import concurrent.futures
import json
import os
import pathlib
import signal
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--verify-reconnect", action="store_true")
    args = parser.parse_args()
    root = args.data_dir.resolve()
    assert (root / ".qa-pet-isolated").is_file(), root
    base = json.loads((root / "run/endpoint.json").read_text())["httpBaseURL"]
    assert urllib.parse.urlsplit(base).hostname == "127.0.0.1", base
    token = (root / "security/local-token").read_text().strip()
    project = pathlib.Path(__file__).resolve().parents[2]
    node = project / "AmitiaData/node/bin/node"
    fixture = project / "scripts/test/qa-mcp-stdio-server.mjs"
    collection = "/api/mcp/servers"

    def call(method, path, body=None, expected=200):
        request = urllib.request.Request(base + path, method=method,
            data=None if body is None else json.dumps(body).encode(),
            headers={"X-Amitia-Local-Token": token, "Content-Type": "application/json"})
        try:
            response = urllib.request.urlopen(request, timeout=18)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            result = json.load(response)
            assert response.status == expected, (method, path, response.status, result)
            return result.get("data", result)

    def await_marker(marker, connection):
        deadline = time.monotonic() + 3
        while not marker.is_file():
            assert not connection.done(), connection.result() if connection.done() else None
            assert time.monotonic() < deadline, marker
            time.sleep(0.025)

    def assert_exited(pid):
        deadline = time.monotonic() + 3
        while time.monotonic() < deadline:
            try:
                os.kill(pid, 0)
            except ProcessLookupError:
                return
            time.sleep(0.025)
        raise AssertionError(f"MCP 进程未退出 {pid}")

    def verify_reconnect(marker):
        config = {"name": "QA-stdio-reconnect", "transport": "stdio", "command": str(node),
                  "args": [str(fixture), str(marker), "0"], "enabled": True, "authType": "none"}
        record = call("POST", collection, config, expected=201)
        path = f"{collection}/{record['id']}"

        def wait_until_ready(count):
            deadline = time.monotonic() + 7
            while time.monotonic() < deadline:
                current = call("GET", path)
                pids = marker.read_text().splitlines() if marker.is_file() else []
                if len(pids) == count and current["status"] == "ready":
                    return int(pids[-1]), current
                time.sleep(0.05)
            raise AssertionError((current["status"], pids))

        try:
            pid, ready = wait_until_ready(1)
            command = subprocess.check_output(["ps", "-p", str(pid), "-o", "command="], text=True)
            assert str(fixture) in command and str(marker) in command, command
            os.kill(pid, signal.SIGKILL)
            assert_exited(pid)
            replacement, recovered = wait_until_ready(2)
            assert replacement != pid and recovered["lastConnectedAt"] > ready["lastConnectedAt"]
            assert [tool["remoteName"] for tool in call("GET", path + "/tools")] == ["echo"]
            assert call("POST", path + "/disconnect", {})["disconnected"]
            assert_exited(replacement)
            time.sleep(2)
            assert call("GET", path)["status"] == "disconnected"
            assert len(marker.read_text().splitlines()) == 2
            print("子进程强退后重新握手并恢复工具，主动断开后保持断开", flush=True)
        finally:
            assert call("DELETE", path)["deleted"]

    with tempfile.TemporaryDirectory(prefix="qa-mcp-") as tmp, concurrent.futures.ThreadPoolExecutor() as pool:
        for index in range(args.rounds):
            paths = []
            markers = {kind: pathlib.Path(tmp) / f"{kind}-{index}.pid" for kind in ("healthy", "slow")}
            try:
                for kind, pause in [("healthy", 0), ("slow", 4200)]:
                    config = {"name": f"QA-stdio-{kind}-{index}", "transport": "stdio", "command": str(node),
                              "args": [str(fixture), str(markers[kind]), str(pause)], "enabled": False, "authType": "none"}
                    record = call("POST", collection, config, expected=201)
                    assert record["status"] == "disconnected" and record["enabled"] == 0, record
                    paths.append(f"{collection}/{record['id']}")
                healthy, slow = paths
                assert call("POST", healthy + "/connect", {})["connected"]
                assert [tool["remoteName"] for tool in call("GET", healthy + "/tools")] == ["echo"]
                slow_connect = pool.submit(call, "POST", slow + "/connect", {})
                await_marker(markers["slow"], slow_connect)
                assert call("GET", slow)["status"] == "connecting"
                duplicate = pool.submit(call, "POST", slow + "/connect", {})
                start = time.monotonic()
                assert call("POST", healthy + "/disconnect", {})["disconnected"]
                elapsed = time.monotonic() - start
                independent = not slow_connect.done()
                assert call("GET", healthy)["status"] == "disconnected"
                assert_exited(int(markers["healthy"].read_text()))
                assert slow_connect.result()["connected"] and duplicate.result()["connected"]
                assert len(markers["slow"].read_text().splitlines()) == 1
                print(f"轮 {index + 1} 断开耗时 {elapsed:.3f}s，慢服务仍在握手={independent}，慢服务只启动一次", flush=True)
                assert independent and elapsed < 2, (elapsed, independent)
            finally:
                for path in reversed(paths):
                    assert call("DELETE", path)["deleted"]
                    assert call("GET", path, expected=404)["detail"] == "record not found"
                for marker in markers.values():
                    if marker.is_file():
                        for pid in marker.read_text().splitlines():
                            assert_exited(int(pid))
        if args.verify_reconnect:
            verify_reconnect(pathlib.Path(tmp) / "reconnect.pid")


if __name__ == "__main__":
    main()
