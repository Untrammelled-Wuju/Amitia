import json
import os
import pathlib
import signal
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request


def endpoint(root):
    try:
        return json.loads((root / "run/endpoint.json").read_text())
    except FileNotFoundError:
        return None


def ready(info):
    if info is None:
        return False
    try:
        with urllib.request.urlopen(info["httpBaseURL"] + "/readyz", timeout=2) as response:
            return json.load(response)["data"]["state"] == "ready"
    except (urllib.error.URLError, TimeoutError, socket.timeout):
        return False


def kill_core(root, info, host_pid):
    command = subprocess.check_output(["ps", "-p", str(info["pid"]), "-o", "ppid=,command="], text=True).strip()
    parent, executable = command.split(maxsplit=1)
    assert int(parent) == host_pid and "AmitiaCore --runtime-profile=local" in executable, command
    assert str(root.parent / "source-desktop") in executable, command
    assert info["httpBaseURL"].startswith("http://127.0.0.1:"), info
    os.kill(info["pid"], signal.SIGKILL)


def main():
    root = pathlib.Path(sys.argv[1]).resolve()
    host_pid = int(sys.argv[2])
    assert (root / ".qa-database-parent-exit").is_file()
    original = endpoint(root)
    assert ready(original), original
    kill_core(root, original, host_pid)
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        replacement = endpoint(root)
        if replacement and replacement["pid"] != original["pid"] and ready(replacement):
            break
        time.sleep(.25)
    else:
        raise AssertionError(f"Core failed to recover from SIGKILL, last endpoint: {endpoint(root)}")
    assert replacement["httpBaseURL"] != original["httpBaseURL"], (original, replacement)
    print(json.dumps({"electron": host_pid, "originalCore": original["pid"],
                      "replacementCore": replacement["pid"], "newEndpointReady": True}), flush=True)
    if sys.argv[3:] == ["--verify-single-attempt"]:
        kill_core(root, replacement, host_pid)
        for _ in range(24):
            observed = endpoint(root)
            assert observed is None or observed["pid"] == replacement["pid"], observed
            time.sleep(.25)
        assert endpoint(root) is None
        os.kill(host_pid, 0)
        print("second crash did not start another Core; Electron still running")


if __name__ == "__main__":
    main()
