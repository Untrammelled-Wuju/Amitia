import json
import pathlib
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[2]


def main():
    endpoint = json.loads((ROOT / "AmitiaData/run/endpoint.json").read_text())
    token = (ROOT / "AmitiaData/security/local-token").read_text().strip()
    results = []
    for action in ["manage", "import", "match"]:
        limit = (9 << 20) if action == "import" else (320 << 10)
        for name, body, original in [
            ("malformed", "{", "unexpected EOF"),
            ("empty", "", "EOF"),
            ("type", '{"requestId":123}', "json: cannot unmarshal"),
            ("oversized", '{"padding":"' + "x" * limit + '"}', "http: request body too large"),
        ]:
            path = "/api/device-mesh/v1/business/worldbook/" + action
            request = urllib.request.Request(endpoint["httpBaseURL"] + path, data=body.encode(), method="POST", headers={"X-Amitia-Local-Token": token, "Content-Type": "application/json"})
            try:
                response = urllib.request.urlopen(request, timeout=30)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                status = response.status
                payload = json.loads(response.read())
            results.append({"path": path, "case": name, "status": status, "response": payload, "passed": status == 400 and original in payload.get("message", "")})
    output = ROOT / "artifacts/qa-20261105-depth/worldbook-parse-live.json"
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(results, ensure_ascii=False, indent=2))
    print(f'{sum(item["passed"] for item in results)}/{len(results)} passed')
    if not all(item["passed"] for item in results):
        raise AssertionError(results)


if __name__ == "__main__":
    main()
