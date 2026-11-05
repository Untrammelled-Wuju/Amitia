import json
import pathlib
import urllib.error
import urllib.request


def main():
    root = pathlib.Path(__file__).resolve().parents[2]
    endpoint = json.loads((root / "AmitiaData/run/endpoint.json").read_text())
    token = (root / "AmitiaData/security/local-token").read_text().strip()
    results = []
    for name, body, expected in [
        ("malformed", "{", "unexpected EOF"),
        ("empty", "", "EOF"),
        ("type", '{"recheckId":123}', "json: cannot unmarshal"),
        ("required", "{}", "failed on the 'required' tag"),
    ]:
        request = urllib.request.Request(
            endpoint["httpBaseURL"] + "/api/knowledge/vector-orphans/audits/qa-nonexistent/recovery-reviews",
            data=body.encode(),
            headers={"Content-Type": "application/json", "X-Amitia-Local-Token": token},
        )
        try:
            response = urllib.request.urlopen(request, timeout=20)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read().decode()
            results.append({"case": name, "status": response.status, "response": raw,
                            "passed": response.status == 400 and expected in raw})
    output = root / "artifacts/qa-20261105-exploration"
    output.mkdir(parents=True, exist_ok=True)
    (output / "recovery-live.json").write_text(json.dumps(results, ensure_ascii=False, indent=2))
    print(f'{sum(item["passed"] for item in results)}/{len(results)} passed')
    if not all(item["passed"] for item in results):
        raise AssertionError(results)


if __name__ == "__main__":
    main()
