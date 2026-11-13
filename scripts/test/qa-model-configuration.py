import argparse
import concurrent.futures
import json
import pathlib
import urllib.error
import urllib.parse
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    args = parser.parse_args()
    root = args.data_dir.resolve()
    if not (root / ".qa-model-isolated").is_file():
        parser.error("需要带 .qa-model-isolated 标记的隔离目录")
    base = json.loads((root / "run/endpoint.json").read_text())["httpBaseURL"]
    if urllib.parse.urlsplit(base).hostname != "127.0.0.1":
        parser.error("需要环回 Core")
    token = (root / "security/local-token").read_text().strip()
    configs = "/api/model/configs"

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
            raw = response.read()
            assert response.status == 200, (method, path, response.status, raw)
            return json.loads(raw)

    def reject(method, path, body, detail):
        result = call(method, path, body)
        assert result["code"] == 400 and detail in result["msg"], result
        return result["msg"]

    before = call("GET", configs)["data"]
    invalid = {"baseUrl": "http://%", "modelName": "qa-model", "isActive": True}
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
        messages = list(pool.map(lambda _: reject("POST", configs, invalid, "invalid URL escape"), range(6)))
    assert call("GET", configs)["data"] == before
    reject("POST", configs, {}, "baseUrl")
    for _ in range(3):
        result = call("POST", "/api/model/test", invalid)
        assert result["code"] == 200 and result["data"]["success"] is False, result
        assert 'parse "http://%/models": invalid URL escape "%"' in result["data"]["message"], result
    print(json.dumps({"rejectedCreates": 7, "preservedRows": len(before),
                      "malformedConnectionAttempts": 3, "originalError": messages[0]}, ensure_ascii=False))


if __name__ == "__main__":
    main()
