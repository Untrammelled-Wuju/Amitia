import argparse
import json
import os
import pathlib
import signal
import subprocess
import time
import urllib.parse
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    parser.add_argument("--verify-after-restart", action="store_true")
    args = parser.parse_args()
    root = args.data_dir.resolve()
    if not (root / ".qa-database-parent-exit").is_file():
        parser.error("必须使用带 .qa-database-parent-exit 标记的隔离测试目录")
    endpoint = json.loads((root / "run" / "endpoint.json").read_text())
    base = endpoint["httpBaseURL"]
    if urllib.parse.urlsplit(base).hostname != "127.0.0.1":
        parser.error("必须使用环回 Core")
    core = endpoint["pid"]
    receipt = root / ".qa-database-parent-exit.json"

    def processes():
        output = subprocess.check_output(["ps", "-axo", "pid=,ppid=,command="], text=True)
        return {
            int(pid): (int(parent), command)
            for pid, parent, command in (line.strip().split(maxsplit=2) for line in output.splitlines())
        }

    def databases(rows):
        result = {}
        for name, relative in (("qdrant", "qdrant/qdrant"), ("surrealdb", "surrealdb/surreal")):
            prefix = str(root / relative) + " "
            pids = [pid for pid, (_, command) in rows.items() if command.startswith(prefix)]
            assert len(pids) == 1, (name, pids)
            result[name] = pids[0]
        return result

    def check_ready():
        with urllib.request.urlopen(base + "/readyz", timeout=10) as response:
            data = json.load(response)["data"]
        assert data["state"] == "ready", data
        assert all(item["id"] not in ("provider.vector-store", "provider.graph-store") for item in data["components"]), data
        return data

    ready = check_ready()
    rows = processes()
    children = databases(rows)
    if args.verify_after_restart:
        old = json.loads(receipt.read_text())
        retired = set(old["databases"].values()) | set(old["supervisors"])
        assert core != old["core"] and not retired.intersection(rows), retired.intersection(rows)
        assert all(pid != old["databases"][name] for name, pid in children.items())
        print(json.dumps({"core": core, "databases": children, "ready": ready}, ensure_ascii=False))
        return

    supervisors = set()
    for pid in children.values():
        parent = rows[pid][0]
        assert parent == core or (rows.get(parent, (0, ""))[0] == core and "--parent-bound-child" in rows[parent][1])
        if parent != core:
            supervisors.add(parent)
    watched = set(children.values()) | supervisors
    receipt.write_text(json.dumps({"core": core, "databases": children, "supervisors": sorted(supervisors)}))
    start = time.monotonic()
    os.kill(core, signal.SIGKILL)
    deadline = start + 5
    while time.monotonic() < deadline:
        rows = processes()
        if not watched.intersection(rows):
            print(json.dumps({"core": core, "databases": children, "supervisors": sorted(supervisors), "exitedInSeconds": round(time.monotonic() - start, 3)}))
            return
        time.sleep(0.05)
    raise AssertionError({pid: rows[pid] for pid in watched.intersection(rows)})


if __name__ == "__main__":
    main()
