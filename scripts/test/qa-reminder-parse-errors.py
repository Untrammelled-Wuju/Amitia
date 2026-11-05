import json
import pathlib
import urllib.error
import urllib.request


def main():
    root = pathlib.Path(__file__).resolve().parents[2]
    endpoint = json.loads((root / 'AmitiaData/run/endpoint.json').read_text())
    token = (root / 'AmitiaData/security/local-token').read_text().strip()
    routes = [
        ('POST', '/api/reminders', '{"title":123}'),
        ('PUT', '/api/reminders/qa-invalid-input', '{"generation":"bad"}'),
        ('DELETE', '/api/reminders/qa-invalid-input', '{"generation":"bad"}'),
        ('POST', '/api/reminders/qa-invalid-input/toggle', '{"generation":"bad"}'),
        ('POST', '/api/reminders/qa-invalid-input/trigger', '{"generation":"bad"}'),
        ('PUT', '/api/reminders/cleanup-config', '{"cleanupDays":123}'),
    ]
    results = []
    for method, path, invalid_type in routes:
        for body, original in [('{', 'unexpected EOF'), ('', 'EOF'), (invalid_type, 'json: cannot unmarshal')]:
            request = urllib.request.Request(
                endpoint['httpBaseURL'] + path, data=body.encode(), method=method,
                headers={'X-Amitia-Local-Token': token, 'Content-Type': 'application/json'},
            )
            try:
                response = urllib.request.urlopen(request, timeout=20)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                status = response.status
                payload = json.loads(response.read())
            passed = status == 400 and payload.get('code') == 400 and original in payload.get('msg', '')
            results.append({'method': method, 'path': path, 'input': body, 'status': status, 'response': payload, 'passed': passed})
    output = root / 'artifacts/qa-20261105-followup/reminder-live-after.json'
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(results, ensure_ascii=False, indent=2))
    print(f'{sum(item["passed"] for item in results)}/{len(results)} passed')
    if not all(item['passed'] for item in results):
        raise AssertionError(results)


if __name__ == '__main__':
    main()
