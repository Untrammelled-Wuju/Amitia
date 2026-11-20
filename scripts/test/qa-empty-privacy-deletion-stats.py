import json
import pathlib
import sys
import urllib.error
import urllib.parse
import urllib.request


def main():
    root = pathlib.Path(sys.argv[1]).resolve()
    if not (root / ".qa-privacy-isolated").is_file():
        raise ValueError("需要带 .qa-privacy-isolated 标记的全新数据目录")
    base = json.loads((root / "run/endpoint.json").read_text())["httpBaseURL"]
    if urllib.parse.urlsplit(base).hostname != "127.0.0.1":
        raise ValueError("需要环回 Core")
    token = (root / "security/local-token").read_text().strip()

    def call(path):
        request = urllib.request.Request(base + path, headers={"X-Amitia-Local-Token": token})
        try:
            response = urllib.request.urlopen(request, timeout=10)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            return response.status, json.load(response)

    status, roles = call("/api/device-mesh/v1/business/roles")
    assert (status, roles["roles"]) == (200, []), (status, roles)
    results = [call("/api/privacy/deletion/stats") for _ in range(3)]
    print(json.dumps(results, ensure_ascii=False), flush=True)
    for status, payload in results:
        assert (status, payload["code"]) == (200, 200), (status, payload)
        for key in ("total", "totalTombstones", "pending", "memoryDeletions"):
            assert type(payload["data"][key]) is int and payload["data"][key] == 0, payload


if __name__ == "__main__":
    main()
