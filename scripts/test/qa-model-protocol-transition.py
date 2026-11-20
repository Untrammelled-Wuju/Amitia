import json
import pathlib
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid


def main():
    root = pathlib.Path(sys.argv[1]).resolve()
    if not (root / ".qa-model-isolated").is_file():
        raise ValueError("需要带 .qa-model-isolated 标记的隔离数据目录")
    base = json.loads((root / "run/endpoint.json").read_text())["httpBaseURL"]
    if urllib.parse.urlsplit(base).hostname != "127.0.0.1":
        raise ValueError("需要环回 Core")
    token = (root / "security/local-token").read_text().strip()

    def call(method, path, body=None):
        request = urllib.request.Request(
            base + path,
            data=None if body is None else json.dumps(body).encode(),
            headers={"X-Amitia-Local-Token": token, "Content-Type": "application/json"},
            method=method,
        )
        try:
            response = urllib.request.urlopen(request, timeout=10)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            return response.status, json.load(response)

    configs = "/api/model/configs"
    status, created = call("POST", configs, {
        "apiType": "ollama", "protocol": "ollama_chat", "baseUrl": "http://127.0.0.1:61377",
        "name": "QA 协议切换 " + uuid.uuid4().hex, "modelName": "qa-compatible", "isActive": 0,
    })
    assert (status, created["code"]) == (200, 200), (status, created)
    config = f"{configs}/{created['data']['id']}"
    try:
        assert created["data"]["protocol"] == "ollama_chat", created["data"]["protocol"]
        status, updated = call("PUT", config, {
            "apiType": "openai-compatible", "baseUrl": "http://127.0.0.1:61377/v1",
        })
        if updated["code"] == 200:
            assert status == 200, (status, updated["code"])
            expected = ("openai-compatible", "openai_chat", "http://127.0.0.1:61377/v1")
        else:
            assert updated["code"] == 400 and updated.get("msg"), (status, updated)
            expected = ("ollama", "ollama_chat", "http://127.0.0.1:61377")
        status, stored = call("GET", config)
        assert (status, stored["code"]) == (200, 200), (status, stored)
        observed = tuple(stored["data"][key] for key in ("apiType", "protocol", "baseUrl"))
        assert observed == expected, {"updateCode": updated["code"], "expected": expected, "stored": observed}
        print(json.dumps({"updateCode": updated["code"], "stored": observed}, ensure_ascii=False))
    finally:
        status, deleted = call("DELETE", config)
        assert (status, deleted["code"]) == (200, 200), (status, deleted)


if __name__ == "__main__":
    main()
